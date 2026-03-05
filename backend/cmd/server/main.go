package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/canonical/signal-studio/internal/alertcoverage"
	"github.com/canonical/signal-studio/internal/analyze"
	"github.com/canonical/signal-studio/internal/api"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/report"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Exit codes following the ESLint/yamllint/Ruff convention.
const (
	exitOK        = 0
	exitFindings  = 1
	exitToolError = 2
)

func main() {
	if len(os.Args) < 2 {
		runServe(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "analyze":
		os.Exit(runAnalyze(os.Args[2:]))
	case "serve":
		runServe(os.Args[2:])
	default:
		// If the first arg doesn't match a subcommand, default to serve
		// for backward compatibility.
		runServe(os.Args[1:])
	}
}

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

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	metricsURL := fs.String("metrics-url", "", "Auto-connect to Collector metrics endpoint on startup")
	_ = fs.String("metrics-token", "", "Bearer token for metrics endpoint")
	_ = fs.String("rules-url", "", "Auto-connect to Prometheus/Mimir rules endpoint on startup")
	_ = fs.String("rules-token", "", "Bearer token for rules endpoint")
	_ = fs.String("rules-org-id", "", "X-Scope-OrgID for Mimir multi-tenant setups")

	if err := fs.Parse(args); err != nil {
		log.Fatalf("invalid flags: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	scrapeInterval := 10 * time.Second
	if v := os.Getenv("SCRAPE_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 5 && n <= 30 {
			scrapeInterval = time.Duration(n) * time.Second
		}
	}

	mgr := metrics.NewManager(scrapeInterval)
	tapDisabled := strings.EqualFold(os.Getenv("TAP_DISABLED"), "true")
	tapMgr := tap.NewManager(tapDisabled)

	if tapDisabled {
		log.Println("OTLP sampling tap disabled via TAP_DISABLED=true")
	}

	// Auto-connect to metrics endpoint if flag provided.
	if *metricsURL != "" {
		cfg := metrics.ScrapeConfig{URL: *metricsURL}
		if err := mgr.Connect(cfg); err != nil {
			log.Printf("warning: failed to auto-connect metrics: %v", err)
		} else {
			log.Printf("auto-connected to metrics endpoint: %s", *metricsURL)
		}
	}

	router := api.NewRouter(mgr, tapMgr, newStaticHandler())

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	// Start server in a goroutine.
	go func() {
		log.Printf("signal-studio listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Block until SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	mgr.Disconnect()
	tapMgr.Stop()
	log.Println("shutdown complete")
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
func marshalRulesAsYAML(rules []alertcoverage.AlertRule) []byte {
	// Group rules by their group name.
	groups := make(map[string][]alertcoverage.AlertRule)
	var order []string
	for _, r := range rules {
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
