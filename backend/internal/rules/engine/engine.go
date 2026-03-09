//go:generate go run github.com/canonical/signal-studio/cmd/docgen -rules ../../../../docs/rules.md -api ../../../../docs/api.md -readme ../../../../README.md

package engine

import (
	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/rules/catalog"
	"github.com/canonical/signal-studio/internal/rules/live"
	"github.com/canonical/signal-studio/internal/rules/static"
	"github.com/canonical/signal-studio/internal/tap"
)

// Engine runs a set of rules against a collector configuration.
type Engine struct {
	rules []rules.Rule
}

// NewEngine creates an engine with the provided rules.
func NewEngine(r ...rules.Rule) *Engine {
	return &Engine{rules: r}
}

// NewDefaultEngine creates an engine with all built-in rules.
func NewDefaultEngine() *Engine {
	var all []rules.Rule
	all = append(all, static.AllRules()...)
	all = append(all, live.AllRules()...)
	all = append(all, catalog.AllRules()...)
	return NewEngine(all...)
}

// Rules returns the engine's rule set.
func (e *Engine) Rules() []rules.Rule {
	return e.rules
}

// Evaluate runs all rules and returns the combined findings.
func (e *Engine) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	findings := []rules.Finding{}
	for _, r := range e.rules {
		findings = append(findings, r.Evaluate(cfg)...)
	}
	return findings
}

// EvaluateWithMetrics runs all rules, using metrics data for LiveRules.
func (e *Engine) EvaluateWithMetrics(cfg *config.CollectorConfig, store *metrics.Store) []rules.Finding {
	findings := []rules.Finding{}
	for _, r := range e.rules {
		if lr, ok := r.(rules.LiveRule); ok && store != nil && store.Len() >= 2 {
			findings = append(findings, lr.EvaluateWithMetrics(cfg, store)...)
		} else {
			findings = append(findings, r.Evaluate(cfg)...)
		}
	}
	return findings
}

// EvaluateWithCatalog runs only CatalogRule implementors, skipping plain Rules
// (which are already evaluated by Evaluate or EvaluateWithMetrics).
func (e *Engine) EvaluateWithCatalog(
	cfg *config.CollectorConfig,
	entries []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []rules.Finding {
	findings := []rules.Finding{}
	for _, r := range e.rules {
		if cr, ok := r.(rules.CatalogRule); ok {
			findings = append(findings, cr.EvaluateWithCatalog(cfg, entries, analyses)...)
		}
	}
	return findings
}
