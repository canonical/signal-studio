package rules

import "github.com/canonical/signal-studio/internal/config"

// Severity indicates the importance of a finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Finding represents a single diagnostic result from a rule evaluation.
type Finding struct {
	RuleID       string   `json:"ruleId"`
	Title        string   `json:"title"`
	Severity     Severity `json:"severity"`
	Evidence     string   `json:"evidence"`
	Explanation  string   `json:"explanation"`
	WhyItMatters string   `json:"whyItMatters"`
	Impact       string   `json:"impact"`
	Snippet      string   `json:"snippet"`
	Placement    string   `json:"placement"`
	Pipeline     string   `json:"pipeline,omitempty"`
}

// Rule evaluates a collector config and returns any findings.
type Rule interface {
	ID() string
	Evaluate(cfg *config.CollectorConfig) []Finding
}

// Engine runs a set of rules against a collector configuration.
type Engine struct {
	rules []Rule
}

// NewEngine creates an engine with the provided rules.
func NewEngine(rules ...Rule) *Engine {
	return &Engine{rules: rules}
}

// NewDefaultEngine creates an engine with all built-in static rules.
func NewDefaultEngine() *Engine {
	return NewEngine(
		&MissingMemoryLimiter{},
		&MissingBatch{},
		&NoTraceSampling{},
		&UnusedComponents{},
		&MultipleExportersNoRouting{},
		&NoLogSeverityFilter{},
		&MemoryLimiterNotFirst{},
		&BatchBeforeSampling{},
		&BatchNotNearEnd{},
		&ReceiverEndpointWildcard{},
		&DebugExporterInPipeline{},
		&PprofExtensionEnabled{},
		&MemoryLimiterWithoutLimits{},
		&ExporterNoSendingQueue{},
		&ExporterNoRetry{},
		&UndefinedComponentRef{},
		&EmptyPipeline{},
		&FilterErrorModePropagateRule{},
		&ScrapeIntervalMismatch{},
		&ExporterInsecureTLS{},
		&NoHealthCheckExtension{},
		&ExporterEndpointLocalhost{},
		&ExporterNoCompression{},
		&TailSamplingWithoutMemoryLimiter{},
		&ConnectorLoop{},
		&NoHealthCheckTraceFilter{},
		// Live rules (require metrics data)
		&HighDropRate{},
		&LogVolumeDominance{},
		&QueueNearCapacity{},
		&ReceiverExporterMismatch{},
		// Catalog rules (require tap catalog + filter analyses)
		&InternalMetricsNotFiltered{},
		&HighAttributeCount{},
		&PointCountOutlier{},
		&FilterKeepsEverything{},
		&FilterDropsEverything{},
		&NoFilterHighVolume{},
		&ManyHistograms{},
		&ShortScrapeInterval{},
	)
}

// Evaluate runs all rules and returns the combined findings.
func (e *Engine) Evaluate(cfg *config.CollectorConfig) []Finding {
	findings := []Finding{}
	for _, r := range e.rules {
		findings = append(findings, r.Evaluate(cfg)...)
	}
	return findings
}
