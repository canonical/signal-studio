package static

import (
	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R30: No spanmetrics connector when both traces and metrics pipelines exist

type NoSpanMetricsConnector struct{}

func (r *NoSpanMetricsConnector) ID() string { return "no-span-metrics-connector" }

func (r *NoSpanMetricsConnector) Description() string {
	return "Traces and metrics pipelines exist but no spanmetrics connector is configured"
}

func (r *NoSpanMetricsConnector) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *NoSpanMetricsConnector) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	// Check for spanmetrics connector already defined.
	for name := range cfg.Connectors {
		if config.ComponentType(name) == "spanmetrics" {
			return nil
		}
	}

	hasTraces := false
	hasMetrics := false
	for _, p := range cfg.Pipelines {
		if p.Signal == config.SignalTraces {
			hasTraces = true
		}
		if p.Signal == config.SignalMetrics {
			hasMetrics = true
		}
	}
	if !hasTraces || !hasMetrics {
		return nil
	}

	return []rules.Finding{{
		RuleID:     r.ID(),
		Title:      "No spanmetrics connector configured",
		Severity:   rules.SeverityInfo,
		Confidence: rules.ConfidenceLow,
		Evidence:   "Configuration has traces and metrics pipelines but no spanmetrics connector.",
		Implication: "The spanmetrics connector derives RED metrics (rate, errors, duration) from trace data without additional instrumentation. Adding it can provide service-level metrics at no extra collection cost." +
			"\nHowever, spanmetrics adds cardinality to the metrics pipeline. If RED metrics are already collected via application instrumentation or an external system, this connector is unnecessary.",
		Snippet: `connectors:
  spanmetrics:
    histogram:
      explicit:
        boundaries: [2ms, 4ms, 6ms, 8ms, 10ms, 50ms, 100ms, 200ms, 500ms, 1s, 5s]
    dimensions:
      - name: http.method
      - name: http.status_code`,
		Recommendation: "Consider adding a spanmetrics connector to derive RED metrics from traces.",
	}}
}
