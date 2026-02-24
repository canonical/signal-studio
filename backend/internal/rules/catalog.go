package rules

import (
	"github.com/simskij/signal-studio/internal/config"
	"github.com/simskij/signal-studio/internal/filter"
	"github.com/simskij/signal-studio/internal/tap"
)

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

// EvaluateWithCatalog runs only CatalogRule implementors, skipping plain Rules
// (which are already evaluated by Evaluate or EvaluateWithMetrics).
func (e *Engine) EvaluateWithCatalog(
	cfg *config.CollectorConfig,
	entries []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []Finding {
	findings := []Finding{}
	for _, r := range e.rules {
		if cr, ok := r.(CatalogRule); ok {
			findings = append(findings, cr.EvaluateWithCatalog(cfg, entries, analyses)...)
		}
	}
	return findings
}
