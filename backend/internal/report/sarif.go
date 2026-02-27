package report

import (
	"io"

	"github.com/canonical/signal-studio/internal/analyze"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

const (
	toolName    = "signal-studio"
	toolInfoURI = "https://github.com/canonical/signal-studio"
)

// SARIFFormatter formats a report as SARIF v2.1.0.
type SARIFFormatter struct{}

func (f *SARIFFormatter) Format(report *analyze.Report, w io.Writer) error {
	sarifReport, err := sarif.New(sarif.Version210)
	if err != nil {
		return err
	}

	run := sarif.NewRunWithInformationURI(toolName, toolInfoURI)

	// Register all rule IDs that appear in findings as tool driver rules.
	ruleIndex := map[string]int{}
	for _, finding := range report.Findings {
		if _, exists := ruleIndex[finding.RuleID]; exists {
			continue
		}
		idx := len(ruleIndex)
		ruleIndex[finding.RuleID] = idx

		rule := run.AddRule(finding.RuleID)
		rule.WithShortDescription(sarif.NewMultiformatMessageString(finding.Title))
		if finding.Implication != "" {
			rule.WithFullDescription(sarif.NewMultiformatMessageString(finding.Implication))
		}
	}

	for _, finding := range report.Findings {
		result := sarif.NewRuleResult(finding.RuleID)
		result.WithLevel(sarifLevel(finding.Severity))
		result.WithMessage(sarif.NewTextMessage(finding.Implication))

		// Add logical location from scope.
		if finding.Scope != "" {
			loc := sarif.NewLocation()
			logLoc := sarif.NewLogicalLocation()
			logLoc.Name = &finding.Scope
			loc.LogicalLocations = append(loc.LogicalLocations, logLoc)
			result.WithLocations([]*sarif.Location{loc})
		}

		// Property bag for extra fields.
		props := sarif.Properties{}
		if finding.Confidence != "" {
			props["confidence"] = string(finding.Confidence)
		}
		if finding.Evidence != "" {
			props["evidence"] = finding.Evidence
		}
		if len(props) > 0 {
			result.Properties = props
		}

		// Suggested fix from snippet.
		if finding.Snippet != "" {
			fix := sarif.NewFix()
			fix.Description = sarif.NewTextMessage(finding.Snippet)
			result.Fixes = append(result.Fixes, fix)
		}

		run.AddResult(result)
	}

	sarifReport.AddRun(run)
	return sarifReport.Write(w)
}

func sarifLevel(s rules.Severity) string {
	switch s {
	case rules.SeverityCritical:
		return "error"
	case rules.SeverityWarning:
		return "warning"
	case rules.SeverityInfo:
		return "note"
	default:
		return "none"
	}
}
