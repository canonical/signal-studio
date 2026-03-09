package catalog

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 1: Internal metrics not filtered

// InternalMetricsNotFiltered fires when otelcol_* metrics are present in the
// catalog but no filter processor drops them.
type InternalMetricsNotFiltered struct{}

func (r *InternalMetricsNotFiltered) ID() string {
	return "catalog-internal-metrics-not-filtered"
}

func (r *InternalMetricsNotFiltered) Description() string {
	return "Internal otelcol_* metrics are exported without filtering"
}

func (r *InternalMetricsNotFiltered) DefaultSeverity() rules.Severity {
	return rules.SeverityWarning
}

func (r *InternalMetricsNotFiltered) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *InternalMetricsNotFiltered) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	entries []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []rules.Finding {
	var internal []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name, "otelcol_") {
			internal = append(internal, e.Name)
		}
	}
	if len(internal) == 0 {
		return nil
	}

	// Check if any filter analysis already drops all internal metrics
	for _, a := range analyses {
		allDropped := true
		for _, name := range internal {
			dropped := false
			for _, res := range a.Results {
				if res.MetricName == name && res.Outcome == filter.OutcomeDropped {
					dropped = true
					break
				}
			}
			if !dropped {
				allDropped = false
				break
			}
		}
		if allDropped {
			return nil
		}
	}

	return []rules.Finding{{
		RuleID:   r.ID(),
		Title:    "Internal collector metrics are being exported",
		Severity:   rules.SeverityWarning,
		Confidence: rules.ConfidenceHigh,
		Evidence: fmt.Sprintf("Found %d otelcol_* metrics: %s",
			len(internal), strings.Join(internal, ", ")),
		Implication: fmt.Sprintf("Internal metrics add volume and cost without providing value to application observability. If you are looking to observe the stability of the collector itself, this warning may be ignored. Filtering %d internal metrics would reduce exported metric cardinality.", len(internal)) + "\nHowever, internal metrics may be needed for self-monitoring. Filter only if they are not used by any dashboard or alert.",
		Scope:        "processor:filter",
		Snippet: `processors:
  filter/internal:
    error_mode: ignore
    metrics:
      metric:
        - 'IsMatch(name, "^otelcol_.*")'`,
		Recommendation: "Add a filter processor to drop otelcol_* metrics in the metrics pipeline.",
	}}
}
