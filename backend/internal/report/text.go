package report

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/canonical/signal-studio/internal/alertcoverage"
	"github.com/canonical/signal-studio/internal/analyze"
	"github.com/canonical/signal-studio/internal/rules"
)

// ANSI color codes.
const (
	ansiRed     = "\033[31m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
)

// severityOrder defines the display order for severity groups.
var severityOrder = []rules.Severity{
	rules.SeverityCritical,
	rules.SeverityWarning,
	rules.SeverityInfo,
}

// TextFormatter formats a report as human-readable text with optional ANSI colors.
type TextFormatter struct {
	NoColor bool
}

func (f *TextFormatter) Format(report *analyze.Report, w io.Writer) error {
	useColor := !f.NoColor && isTerminal(w)

	grouped := groupBySeverity(report.Findings)

	first := true
	for _, sev := range severityOrder {
		findings, ok := grouped[sev]
		if !ok {
			continue
		}

		if !first {
			fmt.Fprintln(w)
		}
		first = false

		f.writeSeverityHeader(w, sev, useColor)
		fmt.Fprintln(w)

		for _, finding := range findings {
			f.writeFinding(w, finding, useColor)
		}
	}

	if len(report.Findings) > 0 {
		fmt.Fprintln(w)
	}

	f.writeSummary(w, report.Summary, useColor)

	if report.AlertCoverage != nil && len(report.AlertCoverage.Results) > 0 {
		fmt.Fprintln(w)
		f.writeAlertCoverage(w, report.AlertCoverage, useColor)
	}

	return nil
}

func (f *TextFormatter) writeSeverityHeader(w io.Writer, sev rules.Severity, color bool) {
	label := strings.ToUpper(string(sev))
	if color {
		label = f.colorSeverity(sev) + label + ansiReset
	}
	fmt.Fprintln(w, label)
}

func (f *TextFormatter) writeFinding(w io.Writer, finding rules.Finding, color bool) {
	ruleID := finding.RuleID
	scopePart := ""
	if color {
		ruleID = ansiBold + ruleID + ansiReset
		if finding.Scope != "" {
			scopePart = fmt.Sprintf(" %s[%s]%s", ansiMagenta, finding.Scope, ansiReset)
		}
	} else {
		if finding.Scope != "" {
			scopePart = fmt.Sprintf(" [%s]", finding.Scope)
		}
	}

	fmt.Fprintf(w, "%s%s\n", ruleID, scopePart)

	if finding.Evidence != "" {
		f.writeDetailSection(w, "Evidence", finding.Evidence, color)
	}

	if finding.Implication != "" {
		f.writeDetailSection(w, "Implication", finding.Implication, color)
	}

	if finding.Recommendation != "" {
		f.writeDetailSection(w, "Recommendation", finding.Recommendation, color)
	}

	fmt.Fprintln(w)
}

func (f *TextFormatter) writeDetailSection(w io.Writer, label, text string, color bool) {
	if color {
		fmt.Fprintf(w, "%s%s:%s %s\n", ansiBold, label, ansiReset, text)
	} else {
		fmt.Fprintf(w, "%s: %s\n", label, text)
	}
}


func (f *TextFormatter) writeSummary(w io.Writer, s analyze.Summary, color bool) {
	parts := []string{}
	if s.Critical > 0 {
		part := fmt.Sprintf("%d critical", s.Critical)
		if color {
			part = ansiRed + part + ansiReset
		}
		parts = append(parts, part)
	}
	if s.Warning > 0 {
		part := fmt.Sprintf("%d warning", s.Warning)
		if color {
			part = ansiYellow + part + ansiReset
		}
		parts = append(parts, part)
	}
	if s.Info > 0 {
		part := fmt.Sprintf("%d info", s.Info)
		if color {
			part = ansiCyan + part + ansiReset
		}
		parts = append(parts, part)
	}

	summary := fmt.Sprintf("%d findings", s.Total)
	if len(parts) > 0 {
		summary += " (" + strings.Join(parts, ", ") + ")"
	}
	fmt.Fprintln(w, summary)
}

func (f *TextFormatter) colorSeverity(s rules.Severity) string {
	switch s {
	case rules.SeverityCritical:
		return ansiBold + ansiRed
	case rules.SeverityWarning:
		return ansiYellow
	case rules.SeverityInfo:
		return ansiCyan
	default:
		return ""
	}
}

var alertStatusOrder = []alertcoverage.AlertStatus{
	alertcoverage.AlertBroken,
	alertcoverage.AlertWouldActivate,
	alertcoverage.AlertAtRisk,
	alertcoverage.AlertUnknown,
	alertcoverage.AlertSafe,
}

func (f *TextFormatter) writeAlertCoverage(w io.Writer, report *alertcoverage.CoverageReport, color bool) {
	header := "ALERT COVERAGE"
	if color {
		header = ansiBold + header + ansiReset
	}
	fmt.Fprintln(w, header)
	fmt.Fprintln(w)

	grouped := make(map[alertcoverage.AlertStatus][]alertcoverage.AlertCoverageResult)
	for _, r := range report.Results {
		grouped[r.Status] = append(grouped[r.Status], r)
	}

	for _, status := range alertStatusOrder {
		results, ok := grouped[status]
		if !ok {
			continue
		}
		for _, r := range results {
			badge := f.alertStatusBadge(r.Status, color)
			fmt.Fprintf(w, "%s %s [%s]\n", badge, r.AlertName, r.AlertGroup)
			for _, m := range r.Metrics {
				fmt.Fprintf(w, "  %s: %s\n", m.MetricName, m.FilterOutcome)
			}
			fmt.Fprintln(w)
		}
	}

	s := report.Summary
	fmt.Fprintf(w, "%d alerts (%d safe, %d at risk, %d broken, %d would activate, %d unknown)\n",
		s.Total, s.Safe, s.AtRisk, s.Broken, s.WouldActivate, s.Unknown)
}

func (f *TextFormatter) alertStatusBadge(status alertcoverage.AlertStatus, color bool) string {
	label := strings.ToUpper(string(status))
	if !color {
		return label
	}
	switch status {
	case alertcoverage.AlertBroken:
		return ansiRed + ansiBold + label + ansiReset
	case alertcoverage.AlertWouldActivate:
		return ansiCyan + label + ansiReset
	case alertcoverage.AlertAtRisk:
		return ansiYellow + label + ansiReset
	case alertcoverage.AlertUnknown:
		return ansiMagenta + label + ansiReset
	case alertcoverage.AlertSafe:
		return "\033[32m" + label + ansiReset // green
	default:
		return label
	}
}

// groupBySeverity groups findings by their severity level.
func groupBySeverity(findings []rules.Finding) map[rules.Severity][]rules.Finding {
	grouped := make(map[rules.Severity][]rules.Finding)
	for _, f := range findings {
		grouped[f.Severity] = append(grouped[f.Severity], f)
	}
	return grouped
}

// isTerminal checks if the writer is connected to a terminal.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}
