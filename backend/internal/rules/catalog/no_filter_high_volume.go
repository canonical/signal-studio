package catalog

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 6: No filter with high volume

// NoFilterHighVolume fires when there are many metrics but no filter processor.
type NoFilterHighVolume struct{}

func (r *NoFilterHighVolume) ID() string { return "catalog-no-filter-high-volume" }

func (r *NoFilterHighVolume) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *NoFilterHighVolume) EvaluateWithCatalog(
	cfg *config.CollectorConfig,
	entries []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []rules.Finding {
	if len(analyses) > 0 {
		return nil
	}

	// Check if any metrics pipeline has a filter processor
	for _, p := range cfg.Pipelines {
		if p.Signal != config.SignalMetrics {
			continue
		}
		if rules.HasProcessorType(p.Processors, "filter") {
			return nil
		}
	}

	if len(entries) <= 50 {
		return nil
	}

	return []rules.Finding{{
		RuleID:       r.ID(),
		Title:        "High metric volume without a filter processor",
		Severity:     rules.SeverityInfo,
		Confidence:   rules.ConfidenceMedium,
		Evidence:     fmt.Sprintf("%d unique metrics observed with no filter processor in any metrics pipeline.", len(entries)),
		Implication:    "Without filtering, all metrics are exported regardless of value, increasing cost and noise. Adding a filter processor can reduce exported volume by dropping low-value metrics.\nHowever, if all metrics are actively used, adding a filter would cause data loss. Review metric usage before filtering.",

		Scope:        "all metrics",
		Snippet: `processors:
  filter/metrics:
    error_mode: ignore
    metrics:
      metric:
        - 'IsMatch(name, "^otelcol_.*")'`,
		Recommendation: "Add a filter processor to the metrics pipeline to control which metrics are exported.",
	}}
}
