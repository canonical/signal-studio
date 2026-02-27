package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/canonical/signal-studio/internal/analyze"
)

// MarkdownFormatter formats a report as a markdown table.
type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Format(report *analyze.Report, w io.Writer) error {
	fmt.Fprintln(w, "## Signal Studio Analysis")
	fmt.Fprintln(w)

	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "No findings.")
		return nil
	}

	fmt.Fprintln(w, "| Severity | Rule | Finding | Scope |")
	fmt.Fprintln(w, "|---|---|---|---|")

	for _, finding := range report.Findings {
		// Replace newlines with <br> to preserve paragraph breaks in table cells.
		implication := strings.ReplaceAll(finding.Implication, "\n", "<br>")
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n",
			finding.Severity, finding.RuleID, implication, finding.Scope)
	}

	fmt.Fprintln(w)
	s := report.Summary
	fmt.Fprintf(w, "**Summary:** %d critical, %d warning, %d info\n",
		s.Critical, s.Warning, s.Info)

	return nil
}
