package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/canonical/signal-studio/internal/alertcoverage"
	"github.com/canonical/signal-studio/internal/analyze"
	"github.com/canonical/signal-studio/internal/report"
	"github.com/canonical/signal-studio/internal/rules"
)

func runAnalyze(args []string) int {
	// Separate positional args from flags so that flags can appear after
	// the path argument (e.g. "analyze config.yaml -f json"). The standard
	// library flag package stops at the first non-flag argument.
	flagArgs, positionalArgs := splitArgs(args)

	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	formatFlag := fs.String("format", "", "Output format: text, json, sarif, markdown (default: auto-detect)")
	fShort := fs.String("f", "", "Short alias for --format")
	failOn := fs.String("fail-on", "warning", "Exit 1 threshold: info, warning, critical")
	minSeverity := fs.String("min-severity", "info", "Minimum severity to include: info, warning, critical")
	noColor := fs.Bool("no-color", false, "Disable ANSI color output")
	alertsPath := fs.String("alerts", "", "Alert rules file (Prometheus/CRD format)")
	rulesURL := fs.String("rules-url", "", "Fetch alert rules from Prometheus/Mimir API")
	rulesToken := fs.String("rules-token", "", "Bearer token for rules endpoint")
	rulesOrgID := fs.String("rules-org-id", "", "X-Scope-OrgID for Mimir multi-tenant setups")

	if err := fs.Parse(flagArgs); err != nil {
		return exitToolError
	}
	// Merge any remaining args from fs.Parse (shouldn't be any) with positional args.
	positionalArgs = append(fs.Args(), positionalArgs...)

	// Determine format (short flag takes precedence if both set).
	format := *formatFlag
	if *fShort != "" {
		format = *fShort
	}
	if format == "" {
		format = autoDetectFormat()
	}

	// Read input YAML.
	yamlBytes, err := readInput(positionalArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitToolError
	}

	// Load alert rules from file and/or API.
	alertRulesYAML, err := loadAlertRules(*alertsPath, *rulesURL, *rulesToken, *rulesOrgID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading alert rules: %v\n", err)
		return exitToolError
	}

	// Run analysis.
	opts := analyze.Options{
		MinSeverity:    parseSeverity(*minSeverity),
		AlertRulesYAML: alertRulesYAML,
	}
	rpt, err := analyze.Run(yamlBytes, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitToolError
	}

	// Format output.
	formatter := selectFormatter(format, *noColor)
	if err := formatter.Format(rpt, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		return exitToolError
	}

	// Determine exit code.
	threshold := parseSeverity(*failOn)
	if analyze.ExceedsThreshold(rpt.Findings, threshold) {
		return exitFindings
	}
	return exitOK
}

// readInput reads YAML from a file path, explicit "-" for stdin, or auto-detected pipe.
func readInput(args []string) ([]byte, error) {
	if len(args) > 0 && args[0] != "-" {
		return os.ReadFile(args[0])
	}

	// Explicit "-" or no args: try stdin.
	if len(args) == 0 {
		// Auto-detect piped stdin.
		stat, _ := os.Stdin.Stat()
		isPipe := (stat.Mode() & os.ModeCharDevice) == 0
		if !isPipe {
			return nil, fmt.Errorf("no input file specified and stdin is not piped\n\nUsage: signal-studio analyze <config.yaml> [flags]")
		}
	}

	return io.ReadAll(os.Stdin)
}

// autoDetectFormat returns "text" for TTY, "json" for piped output.
func autoDetectFormat() string {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return "json"
	}
	if (fi.Mode() & os.ModeCharDevice) != 0 {
		return "text"
	}
	return "json"
}

func selectFormatter(format string, noColor bool) report.Formatter {
	switch format {
	case "json":
		return &report.JSONFormatter{}
	case "sarif":
		return &report.SARIFFormatter{}
	case "markdown":
		return &report.MarkdownFormatter{}
	default:
		return &report.TextFormatter{NoColor: noColor}
	}
}

// splitArgs separates flag arguments from positional arguments.
// Flags start with "-" and may consume the next argument as a value.
// Everything else is a positional argument.
func splitArgs(args []string) (flags, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-" {
			// Lone "-" means stdin, it's a positional arg.
			positional = append(positional, a)
		} else if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			// If this flag uses "=" for its value, it's self-contained.
			if strings.Contains(a, "=") {
				continue
			}
			// Bool flags (--no-color) don't consume next arg.
			if a == "--no-color" || a == "-no-color" {
				continue
			}
			// Other flags consume the next arg as their value.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, a)
		}
	}
	return
}

// loadAlertRules loads alert rules from a file path and/or a Prometheus/Mimir API.
// File rules take precedence; API rules are merged. Returns the raw YAML bytes
// for file input, or marshaled YAML from API rules.
func loadAlertRules(path, url, token, orgID string) ([]byte, error) {
	var fileRules []alertcoverage.AlertRule
	var fileYAML []byte

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading alerts file: %w", err)
		}
		parsed, err := alertcoverage.ParseRules(data)
		if err != nil {
			return nil, fmt.Errorf("parsing alerts file: %w", err)
		}
		fileRules = parsed
		fileYAML = data
	}

	if url != "" {
		result, err := alertcoverage.FetchRules(alertcoverage.ClientOptions{
			URL:   url,
			Token: token,
			OrgID: orgID,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching rules from API: %w", err)
		}

		if len(fileRules) > 0 {
			// Merge file + API rules and re-serialize as YAML for the analyze package.
			merged := alertcoverage.MergeRules(fileRules, result.Rules)
			return marshalRulesAsYAML(merged), nil
		}

		// API-only: serialize API rules as YAML.
		return marshalRulesAsYAML(result.Rules), nil
	}

	return fileYAML, nil
}

// marshalRulesAsYAML converts parsed AlertRules back into a Prometheus-format
// YAML structure that ParseRules can re-parse in the analyze package.
func marshalRulesAsYAML(alertRules []alertcoverage.AlertRule) []byte {
	// Group rules by their group name.
	groups := make(map[string][]alertcoverage.AlertRule)
	var order []string
	for _, r := range alertRules {
		if _, ok := groups[r.Group]; !ok {
			order = append(order, r.Group)
		}
		groups[r.Group] = append(groups[r.Group], r)
	}

	var b strings.Builder
	b.WriteString("groups:\n")
	for _, name := range order {
		b.WriteString("  - name: " + name + "\n")
		b.WriteString("    rules:\n")
		for _, r := range groups[name] {
			if r.Type == "alert" {
				b.WriteString("      - alert: " + r.Name + "\n")
			} else {
				b.WriteString("      - record: " + r.Name + "\n")
			}
			b.WriteString("        expr: " + r.Expr + "\n")
		}
	}
	return []byte(b.String())
}

func parseSeverity(s string) rules.Severity {
	switch strings.ToLower(s) {
	case "critical":
		return rules.SeverityCritical
	case "warning":
		return rules.SeverityWarning
	case "info":
		return rules.SeverityInfo
	default:
		return rules.SeverityWarning
	}
}
