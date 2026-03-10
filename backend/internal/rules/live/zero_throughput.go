package live

import (
	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
)

// ZeroThroughput fires when receiver accepted rate is zero across all signals
// for 3+ consecutive intervals, indicating nothing is flowing through the
// collector.
type ZeroThroughput struct{}

func (r *ZeroThroughput) ID() string { return "live-zero-throughput" }

func (r *ZeroThroughput) Description() string {
	return "No data flowing through any receiver for multiple intervals"
}

func (r *ZeroThroughput) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *ZeroThroughput) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *ZeroThroughput) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []rules.Finding {
	if store.Len() < 4 {
		return nil
	}
	window := store.Window()

	acceptedMetrics := []string{
		metrics.MetricReceiverAcceptedSpans,
		metrics.MetricReceiverAcceptedMetricPoints,
		metrics.MetricReceiverAcceptedLogRecords,
	}

	// Check the last 3 intervals for zero traffic across all signals.
	zeroIntervals := 0
	for i := len(window) - 1; i >= 1; i-- {
		anyTraffic := false
		for _, m := range acceptedMetrics {
			rate := sumRateAcrossLabels(window[i-1], window[i], m)
			if rate > 0 {
				anyTraffic = true
				break
			}
		}
		if anyTraffic {
			break
		}
		zeroIntervals++
	}

	if zeroIntervals < 3 {
		return nil
	}

	return []rules.Finding{{
		RuleID:     r.ID(),
		Title:      "No data flowing through the collector",
		Severity:   rules.SeverityWarning,
		Confidence: rules.ConfidenceMedium,
		Evidence:   "Zero accepted spans, metric points, and log records across all receivers for 3+ consecutive scrape intervals.",
		Implication: "The collector is running but not receiving any telemetry data. This may indicate that upstream applications are not instrumented, SDK endpoints are misconfigured, or network connectivity is broken between sources and the collector." +
			"\nHowever, zero traffic may be expected in development environments or during off-peak hours with no active workloads.",
		Snippet: `# Verify the OTLP receiver is configured:
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318`,
		Recommendation: "Verify that upstream applications are configured to send telemetry to this collector's receiver endpoints.",
	}}
}
