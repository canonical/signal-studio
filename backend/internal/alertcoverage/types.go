package alertcoverage

import "github.com/canonical/signal-studio/internal/filter"

// AlertRule represents a parsed alerting or recording rule.
type AlertRule struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "alert" or "record"
	Expr        string   `json:"expr"`
	MetricNames []string `json:"metricNames"`
	Group       string   `json:"group"`
	UsesAbsent  bool     `json:"usesAbsent"`
}

// AlertCoverageResult represents the filter impact on a single alert.
type AlertCoverageResult struct {
	AlertName  string              `json:"alertName"`
	AlertGroup string              `json:"alertGroup"`
	Expr       string              `json:"expr"`
	Metrics    []AlertMetricResult `json:"metrics"`
	Status     AlertStatus         `json:"status"`
}

// AlertMetricResult describes the filter outcome for a single metric
// referenced by an alert rule.
type AlertMetricResult struct {
	MetricName    string              `json:"metricName"`
	FilterOutcome filter.MatchOutcome `json:"filterOutcome"`
}

// AlertStatus describes the overall coverage status of an alert.
type AlertStatus string

const (
	AlertSafe          AlertStatus = "safe"
	AlertAtRisk        AlertStatus = "at_risk"
	AlertBroken        AlertStatus = "broken"
	AlertWouldActivate AlertStatus = "would_activate"
	AlertUnknown       AlertStatus = "unknown"
)

// CoverageReport holds the full alert coverage analysis results.
type CoverageReport struct {
	Results   []AlertCoverageResult `json:"results"`
	Summary   CoverageSummary       `json:"summary"`
	RulesYAML string                `json:"rulesYaml,omitempty"`
}

// CoverageSummary counts alerts by status.
type CoverageSummary struct {
	Total         int `json:"total"`
	Safe          int `json:"safe"`
	AtRisk        int `json:"atRisk"`
	Broken        int `json:"broken"`
	WouldActivate int `json:"wouldActivate"`
	Unknown       int `json:"unknown"`
}
