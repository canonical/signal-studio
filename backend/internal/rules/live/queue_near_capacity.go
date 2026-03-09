package live

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
)

// QueueNearCapacity fires when exporter queue utilization exceeds 80%, sustained.
type QueueNearCapacity struct{}

func (r *QueueNearCapacity) ID() string { return "live-queue-near-capacity" }

func (r *QueueNearCapacity) Description() string { return "Exporter queue utilization exceeds 80%" }

func (r *QueueNearCapacity) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *QueueNearCapacity) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *QueueNearCapacity) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []rules.Finding {
	if store.Len() < 3 {
		return nil
	}
	window := store.Window()

	// Collect unique exporters from latest snapshot
	latest := window[len(window)-1]
	exporters := make(map[string]struct{})
	for _, s := range latest.Samples {
		if s.Name == metrics.MetricExporterQueueSize {
			if exp, ok := s.Labels["exporter"]; ok {
				exporters[exp] = struct{}{}
			}
		}
	}

	var findings []rules.Finding
	for exp := range exporters {
		lbls := map[string]string{"exporter": exp}
		sustainedCount := 0
		var lastUtil float64
		for _, snap := range window {
			qSize, sizeOk := metrics.GaugeValue(snap, metrics.MetricExporterQueueSize, lbls)
			qCap, capOk := metrics.GaugeValue(snap, metrics.MetricExporterQueueCapacity, lbls)
			if !sizeOk || !capOk || qCap <= 0 {
				continue
			}
			util := (qSize / qCap) * 100
			if util > 80 {
				sustainedCount++
				lastUtil = util
			} else {
				sustainedCount = 0
			}
		}
		if sustainedCount >= 2 {
			findings = append(findings, rules.Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Exporter queue near capacity: %s", exp),
				Severity:     rules.SeverityWarning,
				Confidence:   rules.ConfidenceMedium,
				Evidence:     fmt.Sprintf("Queue utilization at %.0f%% for exporter %s", lastUtil, exp),
				Implication:    "A full queue causes data loss as new items are dropped when the queue overflows. Risk of telemetry data loss if the queue reaches 100%.\nHowever, queue spikes may be transient during backend maintenance or network blips. Sustained high utilization is more concerning.",

				Scope:        fmt.Sprintf("exporter:%s", exp),
				Snippet: fmt.Sprintf(`exporters:
  %s:
    sending_queue:
      enabled: true
      num_consumers: 20
      queue_size: 10000`, exp),
				Recommendation: "Increase queue capacity or add more consumers to the exporter configuration.",
			})
		}
	}
	return findings
}
