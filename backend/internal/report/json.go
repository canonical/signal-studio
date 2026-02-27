package report

import (
	"encoding/json"
	"io"

	"github.com/canonical/signal-studio/internal/analyze"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
)

// jsonOutput is the envelope for JSON output with a version field.
type jsonOutput struct {
	FormatVersion  string                 `json:"formatVersion"`
	Findings       []rules.Finding        `json:"findings"`
	FilterAnalyses []filter.FilterAnalysis `json:"filterAnalyses,omitempty"`
	Summary        analyze.Summary        `json:"summary"`
}

// JSONFormatter formats a report as JSON.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(report *analyze.Report, w io.Writer) error {
	out := jsonOutput{
		FormatVersion:  "1",
		Findings:       report.Findings,
		FilterAnalyses: report.FilterAnalyses,
		Summary:        report.Summary,
	}
	if out.Findings == nil {
		out.Findings = []rules.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
