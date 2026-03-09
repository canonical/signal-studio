package rules

import (
	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/tap"
)

// Severity indicates the importance of a finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Confidence indicates how certain a rule is about its finding.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Finding represents a single diagnostic result from a rule evaluation.
type Finding struct {
	RuleID         string     `json:"ruleId"`
	Title          string     `json:"title"`
	Severity       Severity   `json:"severity"`
	Confidence     Confidence `json:"confidence"`
	Evidence       string     `json:"evidence"`
	Implication    string     `json:"implication"`
	Recommendation string     `json:"recommendation"`
	Snippet        string     `json:"snippet"`
	Scope          string     `json:"scope,omitempty"`
}

// Rule evaluates a collector config and returns any findings.
type Rule interface {
	ID() string
	Description() string
	DefaultSeverity() Severity
	Evaluate(cfg *config.CollectorConfig) []Finding
}

// LiveRule evaluates a collector config together with live metrics data.
type LiveRule interface {
	Rule
	EvaluateWithMetrics(cfg *config.CollectorConfig, store *metrics.Store) []Finding
}

// CatalogRule evaluates a collector config together with live catalog data
// and filter analyses from the OTLP sampling tap.
type CatalogRule interface {
	Rule
	EvaluateWithCatalog(
		cfg *config.CollectorConfig,
		entries []tap.MetricEntry,
		analyses []filter.FilterAnalysis,
	) []Finding
}
