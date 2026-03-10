package live

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
)

// ExporterSustainedFailures fires when any exporter has a non-zero send failure
// rate sustained over 3+ consecutive snapshot intervals.
type ExporterSustainedFailures struct{}

func (r *ExporterSustainedFailures) ID() string { return "live-exporter-sustained-failures" }

func (r *ExporterSustainedFailures) Description() string {
	return "Exporter is experiencing sustained send failures"
}

func (r *ExporterSustainedFailures) DefaultSeverity() rules.Severity { return rules.SeverityCritical }

func (r *ExporterSustainedFailures) Evaluate(_ *config.CollectorConfig) []rules.Finding {
	return nil
}

func (r *ExporterSustainedFailures) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []rules.Finding {
	if store.Len() < 4 {
		return nil
	}
	window := store.Window()

	failedMetrics := []struct {
		name   string
		signal string
	}{
		{metrics.MetricExporterSendFailedSpans, "traces"},
		{metrics.MetricExporterSendFailedMetricPts, "metrics"},
		{metrics.MetricExporterSendFailedLogRecs, "logs"},
	}

	// Collect unique exporters from the latest snapshot.
	latest := window[len(window)-1]
	exporters := make(map[string]struct{})
	for _, s := range latest.Samples {
		if exp, ok := s.Labels["exporter"]; ok {
			exporters[exp] = struct{}{}
		}
	}

	var findings []rules.Finding
	for exp := range exporters {
		lbls := map[string]string{"exporter": exp}
		for _, fm := range failedMetrics {
			sustainedCount := 0
			var lastRate float64
			for i := 1; i < len(window); i++ {
				rate := metrics.RatePerSecond(window[i-1], window[i], fm.name, lbls)
				if rate > 0 {
					sustainedCount++
					lastRate = rate
				} else {
					sustainedCount = 0
				}
			}
			if sustainedCount >= 3 {
				findings = append(findings, rules.Finding{
					RuleID:     r.ID(),
					Title:      fmt.Sprintf("Exporter %s is failing to send %s", exp, fm.signal),
					Severity:   rules.SeverityCritical,
					Confidence: rules.ConfidenceHigh,
					Evidence:   fmt.Sprintf("Exporter %q has a sustained %s send failure rate of %.1f/s.", exp, fm.signal, lastRate),
					Implication: fmt.Sprintf("The exporter is actively losing %s data. This typically indicates the backend is unreachable, rejecting requests, or timing out. Data sent during this period is permanently lost unless a persistent queue is configured.", fm.signal) +
						"\nHowever, brief failure bursts can occur during backend deployments or network maintenance. Sustained failures over minutes indicate a real problem.",
					Scope: fmt.Sprintf("exporter:%s", exp),
					Snippet: fmt.Sprintf(`exporters:
  %s:
    retry_on_failure:
      enabled: true
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      queue_size: 10000`, exp),
					Recommendation: "Check backend connectivity and ensure retry and sending queue are configured.",
				})
				break // One finding per exporter is enough.
			}
		}
	}
	return findings
}
