package catalog

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 3: Point count outlier

// PointCountOutlier fires when a metric's point count is >10x the mean and >1000.
type PointCountOutlier struct{}

func (r *PointCountOutlier) ID() string { return "catalog-point-count-outlier" }

func (r *PointCountOutlier) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *PointCountOutlier) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	entries []tap.MetricEntry,
	_ []filter.FilterAnalysis,
) []rules.Finding {
	if len(entries) == 0 {
		return nil
	}

	var total int64
	for _, e := range entries {
		total += e.PointCount
	}
	mean := float64(total) / float64(len(entries))

	var findings []rules.Finding
	for _, e := range entries {
		if float64(e.PointCount) > 10*mean && e.PointCount > 1000 {
			findings = append(findings, rules.Finding{
				RuleID:     r.ID(),
				Title:      fmt.Sprintf("Point count outlier: %s", e.Name),
				Severity:   rules.SeverityWarning,
				Confidence: rules.ConfidenceMedium,
				Evidence: fmt.Sprintf("Metric %q has %d points (mean: %.0f)",
					e.Name, e.PointCount, mean),
				Implication:    "Outlier metrics dominate storage and query cost disproportionately. " + fmt.Sprintf("Metric %q accounts for a disproportionate share of total data points.", e.Name) + "\nHowever, this metric may look like an outlier during startup when not all metrics have been observed yet.",
				Scope:        fmt.Sprintf("metric:%s", e.Name),
				Snippet: `processors:
  filter/high-volume:
    error_mode: ignore
    metrics:
      metric:
        - 'name == "<metric_name>"'`,
				Recommendation: "Consider filtering, aggregating, or reducing the emission frequency of this metric.",
			})
		}
	}
	return findings
}
