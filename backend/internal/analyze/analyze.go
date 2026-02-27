package analyze

import (
	"github.com/canonical/signal-studio/internal/alertcoverage"
	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/rules/engine"
)

// Report holds the results of a static analysis run.
type Report struct {
	Config         *config.CollectorConfig      `json:"config"`
	Findings       []rules.Finding              `json:"findings"`
	FilterAnalyses []filter.FilterAnalysis       `json:"filterAnalyses,omitempty"`
	AlertCoverage  *alertcoverage.CoverageReport `json:"alertCoverage,omitempty"`
	Summary        Summary                      `json:"summary"`
}

// Summary counts findings by severity.
type Summary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

// Options controls the analysis behavior.
type Options struct {
	MinSeverity    rules.Severity
	AlertRulesYAML []byte // optional alert rules for coverage analysis
}

// Run performs static analysis on the given YAML bytes and returns a Report.
func Run(yamlBytes []byte, opts Options) (*Report, error) {
	cfg, err := config.Parse(yamlBytes)
	if err != nil {
		return nil, err
	}

	eng := engine.NewDefaultEngine()
	findings := eng.Evaluate(cfg)

	// Run filter analysis (name-only, no catalog data in CLI mode)
	fcs := filter.ExtractFilterConfigs(cfg)
	var filterAnalyses []filter.FilterAnalysis
	for _, fc := range fcs {
		if len(fc.Rules) == 0 {
			continue
		}
		// In static mode we have no catalog, so we extract metric names
		// from the filter rules themselves for basic analysis
		filterAnalyses = append(filterAnalyses, filter.FilterAnalysis{
			ProcessorName: fc.ProcessorName,
			Pipeline:      fc.Pipeline,
			Style:         fc.Style,
		})
	}

	// Run alert coverage analysis if alert rules provided.
	var coverageReport *alertcoverage.CoverageReport
	if len(opts.AlertRulesYAML) > 0 {
		alertRules, err := alertcoverage.ParseRules(opts.AlertRulesYAML)
		if err == nil && len(alertRules) > 0 {
			coverageReport = alertcoverage.Analyze(alertRules, filterAnalyses, nil)
		}
	}

	// Apply severity filter
	filtered := filterBySeverity(findings, opts.MinSeverity)

	summary := buildSummary(filtered)

	return &Report{
		Config:         cfg,
		Findings:       filtered,
		FilterAnalyses: filterAnalyses,
		AlertCoverage:  coverageReport,
		Summary:        summary,
	}, nil
}

// SeverityRank returns the numeric rank for a severity (higher = more severe).
func SeverityRank(s rules.Severity) int {
	switch s {
	case rules.SeverityCritical:
		return 3
	case rules.SeverityWarning:
		return 2
	case rules.SeverityInfo:
		return 1
	default:
		return 0
	}
}

// filterBySeverity keeps only findings at or above the minimum severity.
func filterBySeverity(findings []rules.Finding, minSeverity rules.Severity) []rules.Finding {
	if minSeverity == "" || minSeverity == rules.SeverityInfo {
		return findings
	}
	minRank := SeverityRank(minSeverity)
	var result []rules.Finding
	for _, f := range findings {
		if SeverityRank(f.Severity) >= minRank {
			result = append(result, f)
		}
	}
	return result
}

func buildSummary(findings []rules.Finding) Summary {
	s := Summary{Total: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case rules.SeverityCritical:
			s.Critical++
		case rules.SeverityWarning:
			s.Warning++
		case rules.SeverityInfo:
			s.Info++
		}
	}
	return s
}

// ExceedsThreshold returns true if any finding meets or exceeds the given severity threshold.
func ExceedsThreshold(findings []rules.Finding, threshold rules.Severity) bool {
	threshRank := SeverityRank(threshold)
	for _, f := range findings {
		if SeverityRank(f.Severity) >= threshRank {
			return true
		}
	}
	return false
}
