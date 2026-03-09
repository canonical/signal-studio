package catalog

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 7: Many histograms

// ManyHistograms fires when histogram metrics dominate the catalog.
type ManyHistograms struct{}

func (r *ManyHistograms) ID() string { return "catalog-many-histograms" }

func (r *ManyHistograms) Description() string {
	return "Histogram metrics dominate the metric stream"
}

func (r *ManyHistograms) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *ManyHistograms) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *ManyHistograms) EvaluateWithCatalog(
	cfg *config.CollectorConfig,
	entries []tap.MetricEntry,
	_ []filter.FilterAnalysis,
) []rules.Finding {
	if len(entries) == 0 {
		return nil
	}

	// Skip if a cumulativetodelta or transform processor exists
	for _, p := range cfg.Pipelines {
		if p.Signal != config.SignalMetrics {
			continue
		}
		for _, proc := range p.Processors {
			pt := config.ComponentType(proc)
			if pt == "cumulativetodelta" || pt == "transform" {
				return nil
			}
		}
	}

	var histCount int
	for _, e := range entries {
		if e.Type == tap.MetricTypeHistogram || e.Type == tap.MetricTypeExponentialHistogram {
			histCount++
		}
	}

	pct := float64(histCount) / float64(len(entries)) * 100
	if histCount <= 5 || pct <= 30 {
		return nil
	}

	return []rules.Finding{{
		RuleID:     r.ID(),
		Title:      "Histogram-heavy metric stream",
		Severity:   rules.SeverityInfo,
		Confidence: rules.ConfidenceMedium,
		Evidence: fmt.Sprintf("%d of %d metrics (%.0f%%) are histograms.",
			histCount, len(entries), pct),
		Implication:    "Each histogram bucket is a separate series. A single histogram with 10 buckets generates 10+ series. Consider converting cumulative histograms to delta temporality or using exponential histograms to reduce bucket count.\nHowever, histograms provide valuable distribution data. Converting to delta or reducing buckets loses percentile accuracy.",

		Scope:        "all metrics",
		Snippet: `processors:
  cumulativetodelta:
    include:
      match_type: strict
      metrics: [...]`,
		Recommendation: "Add a cumulativetodelta or transform processor to reduce histogram cardinality.",
	}}
}
