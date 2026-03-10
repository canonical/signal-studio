package live

import (
	"fmt"
	"math"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
)

// ReceiverBackpressure fires when receiver accepted rate drops sharply (>50%)
// between consecutive intervals, suggesting the collector is applying
// backpressure via memory_limiter.
type ReceiverBackpressure struct{}

func (r *ReceiverBackpressure) ID() string { return "live-receiver-backpressure" }

func (r *ReceiverBackpressure) Description() string {
	return "Receiver accepted rate dropped sharply, indicating backpressure"
}

func (r *ReceiverBackpressure) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *ReceiverBackpressure) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *ReceiverBackpressure) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []rules.Finding {
	if store.Len() < 4 {
		return nil
	}
	window := store.Window()

	acceptedMetrics := []struct {
		name   string
		signal string
	}{
		{metrics.MetricReceiverAcceptedSpans, "traces"},
		{metrics.MetricReceiverAcceptedMetricPoints, "metrics"},
		{metrics.MetricReceiverAcceptedLogRecords, "logs"},
	}

	var findings []rules.Finding
	for _, am := range acceptedMetrics {
		// Compute rates for consecutive intervals.
		rates := make([]float64, 0, len(window)-1)
		for i := 1; i < len(window); i++ {
			rates = append(rates, sumRateAcrossLabels(window[i-1], window[i], am.name))
		}

		// Need at least 3 intervals to detect a sustained drop.
		if len(rates) < 3 {
			continue
		}

		// Check the last two intervals for a sharp drop compared to the
		// baseline (average of earlier intervals).
		baseline := 0.0
		baselineCount := len(rates) - 2
		if baselineCount <= 0 {
			continue
		}
		for _, r := range rates[:baselineCount] {
			baseline += r
		}
		baseline /= float64(baselineCount)

		if baseline < 10 {
			// Too little traffic to be meaningful.
			continue
		}

		// Check if the last two intervals are both below 50% of baseline.
		recent1 := rates[len(rates)-2]
		recent2 := rates[len(rates)-1]
		dropPct1 := ((baseline - recent1) / baseline) * 100
		dropPct2 := ((baseline - recent2) / baseline) * 100

		if dropPct1 < 50 || dropPct2 < 50 {
			continue
		}

		avgDrop := (dropPct1 + dropPct2) / 2
		findings = append(findings, rules.Finding{
			RuleID:     r.ID(),
			Title:      fmt.Sprintf("Receiver backpressure detected on %s", am.signal),
			Severity:   rules.SeverityWarning,
			Confidence: rules.ConfidenceMedium,
			Evidence: fmt.Sprintf("Accepted %s rate dropped from %.0f/s to %.0f/s (%.0f%% reduction).",
				am.signal, math.Round(baseline), math.Round(recent2), avgDrop),
			Implication: "A sharp drop in accepted rate typically means the collector is applying backpressure, often triggered by memory_limiter. This indicates the collector is under memory pressure and may be refusing incoming data." +
				"\nHowever, a rate drop can also reflect a legitimate decrease in upstream traffic. Check collector memory usage to confirm.",
			Scope: fmt.Sprintf("signal:%s", am.signal),
			Snippet: `processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 1024
    spike_limit_mib: 256`,
			Recommendation: "Check collector memory usage. Consider increasing memory limits or scaling horizontally.",
		})
	}
	return findings
}
