package rules

import (
	"fmt"
	"math"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
)

// LiveRule evaluates a collector config together with live metrics data.
type LiveRule interface {
	Rule
	EvaluateWithMetrics(cfg *config.CollectorConfig, store *metrics.Store) []Finding
}

// EvaluateWithMetrics runs all rules, using metrics data for LiveRules.
func (e *Engine) EvaluateWithMetrics(cfg *config.CollectorConfig, store *metrics.Store) []Finding {
	findings := []Finding{}
	for _, r := range e.rules {
		if lr, ok := r.(LiveRule); ok && store != nil && store.Len() >= 2 {
			findings = append(findings, lr.EvaluateWithMetrics(cfg, store)...)
		} else {
			findings = append(findings, r.Evaluate(cfg)...)
		}
	}
	return findings
}

// HighDropRate fires when (accepted - sent) / accepted > 10% for any signal,
// sustained over 2+ snapshot intervals.
type HighDropRate struct{}

func (r *HighDropRate) ID() string { return "live-high-drop-rate" }

func (r *HighDropRate) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *HighDropRate) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []Finding {
	if store.Len() < 3 {
		return nil
	}
	window := store.Window()
	type signalDef struct {
		name     string
		accepted string
		sent     string
	}
	signals := []signalDef{
		{"traces", metrics.MetricReceiverAcceptedSpans, metrics.MetricExporterSentSpans},
		{"metrics", metrics.MetricReceiverAcceptedMetricPoints, metrics.MetricExporterSentMetricPoints},
		{"logs", metrics.MetricReceiverAcceptedLogRecords, metrics.MetricExporterSentLogRecords},
	}

	var findings []Finding
	for _, sig := range signals {
		sustainedCount := 0
		var lastDropPct float64
		for i := 1; i < len(window); i++ {
			acceptedRate := sumRateAcrossLabels(window[i-1], window[i], sig.accepted)
			sentRate := sumRateAcrossLabels(window[i-1], window[i], sig.sent)
			if acceptedRate <= 0 {
				continue
			}
			dropPct := ((acceptedRate - sentRate) / acceptedRate) * 100
			if dropPct < 0 {
				dropPct = 0
			}
			if dropPct > 10 {
				sustainedCount++
				lastDropPct = dropPct
			} else {
				sustainedCount = 0
			}
		}
		if sustainedCount >= 2 {
			findings = append(findings, Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("High drop rate on %s pipeline", sig.name),
				Severity:     SeverityWarning,
				Evidence:     fmt.Sprintf("%.1f%% of %s are being dropped", lastDropPct, sig.name),
				Explanation:  fmt.Sprintf("The difference between accepted and sent %s exceeds 10%% over multiple intervals.", sig.name),
				WhyItMatters: "Unexpected drops indicate backpressure, misconfiguration, or overload in the pipeline.",
				Impact:       fmt.Sprintf("Approximately %.0f%% of %s data is being lost.", lastDropPct, sig.name),
				Snippet: `processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
  batch:
    send_batch_size: 512
    timeout: 5s`,
				Placement: "Review memory_limiter, batch, and queue settings.",
			})
		}
	}
	return findings
}

// LogVolumeDominance fires when log ingest rate exceeds 3x trace ingest rate.
type LogVolumeDominance struct{}

func (r *LogVolumeDominance) ID() string { return "live-log-volume-dominance" }

func (r *LogVolumeDominance) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *LogVolumeDominance) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []Finding {
	prev := store.Previous()
	curr := store.Latest()
	if prev == nil || curr == nil {
		return nil
	}

	logRate := sumRateAcrossLabels(prev, curr, metrics.MetricReceiverAcceptedLogRecords)
	traceRate := sumRateAcrossLabels(prev, curr, metrics.MetricReceiverAcceptedSpans)

	if traceRate <= 0 || logRate <= 3*traceRate {
		return nil
	}

	ratio := logRate / traceRate
	return []Finding{{
		RuleID:       r.ID(),
		Title:        "Logs dominating telemetry volume",
		Severity:     SeverityInfo,
		Evidence:     fmt.Sprintf("Log ingest: %.0f/s, Trace ingest: %.0f/s (%.1fx ratio)", logRate, traceRate, ratio),
		Explanation:  "Log records are being ingested at more than 3x the rate of trace spans.",
		WhyItMatters: "Log volume frequently dominates telemetry cost. Many logs are low-value (DEBUG/INFO level).",
		Impact:       fmt.Sprintf("Logs are consuming %.1fx more pipeline capacity than traces.", ratio),
		Snippet: `processors:
  filter/logs:
    error_mode: ignore
    logs:
      log_record:
        - 'severity_number < SEVERITY_NUMBER_WARN'`,
		Placement: "Add a severity filter processor to the logs pipeline before the batch processor.",
	}}
}

// QueueNearCapacity fires when exporter queue utilization exceeds 80%, sustained.
type QueueNearCapacity struct{}

func (r *QueueNearCapacity) ID() string { return "live-queue-near-capacity" }

func (r *QueueNearCapacity) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *QueueNearCapacity) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []Finding {
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

	var findings []Finding
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
			findings = append(findings, Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Exporter queue near capacity: %s", exp),
				Severity:     SeverityWarning,
				Evidence:     fmt.Sprintf("Queue utilization at %.0f%% for exporter %s", lastUtil, exp),
				Explanation:  "The exporter's sending queue is consistently above 80% capacity.",
				WhyItMatters: "A full queue causes data loss as new items are dropped when the queue overflows.",
				Impact:       "Risk of telemetry data loss if the queue reaches 100%.",
				Snippet: fmt.Sprintf(`exporters:
  %s:
    sending_queue:
      enabled: true
      num_consumers: 20
      queue_size: 10000`, exp),
				Placement: "Increase queue capacity or add more consumers to the exporter configuration.",
			})
		}
	}
	return findings
}

// ReceiverExporterMismatch fires when accepted >> sent for a signal, sustained.
type ReceiverExporterMismatch struct{}

func (r *ReceiverExporterMismatch) ID() string { return "live-receiver-exporter-mismatch" }

func (r *ReceiverExporterMismatch) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *ReceiverExporterMismatch) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []Finding {
	if store.Len() < 4 {
		return nil
	}
	window := store.Window()

	type signalDef struct {
		name     string
		accepted string
		sent     string
	}
	signals := []signalDef{
		{"traces", metrics.MetricReceiverAcceptedSpans, metrics.MetricExporterSentSpans},
		{"metrics", metrics.MetricReceiverAcceptedMetricPoints, metrics.MetricExporterSentMetricPoints},
		{"logs", metrics.MetricReceiverAcceptedLogRecords, metrics.MetricExporterSentLogRecords},
	}

	var findings []Finding
	for _, sig := range signals {
		sustainedCount := 0
		var lastAccepted, lastSent float64
		for i := 1; i < len(window); i++ {
			acceptedRate := sumRateAcrossLabels(window[i-1], window[i], sig.accepted)
			sentRate := sumRateAcrossLabels(window[i-1], window[i], sig.sent)
			if acceptedRate <= 0 {
				continue
			}
			if acceptedRate > 2*sentRate && sentRate >= 0 {
				sustainedCount++
				lastAccepted = acceptedRate
				lastSent = sentRate
			} else {
				sustainedCount = 0
			}
		}
		if sustainedCount >= 3 {
			findings = append(findings, Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Receiver-exporter mismatch on %s", sig.name),
				Severity:     SeverityWarning,
				Evidence:     fmt.Sprintf("Accepted: %.0f/s, Sent: %.0f/s for %s", lastAccepted, lastSent, sig.name),
				Explanation:  fmt.Sprintf("The receiver is accepting more than 2x the %s that are being exported, sustained across multiple intervals.", sig.name),
				WhyItMatters: "This suggests drops, pipeline blockage, or exporter failure.",
				Impact:       fmt.Sprintf("%.0f%% of %s may be lost between receiver and exporter.", ((lastAccepted-lastSent)/lastAccepted)*100, sig.name),
				Snippet:       "",
				Placement:    "Check processor dropped counters, exporter errors, and queue utilization.",
			})
		}
	}
	return findings
}

// sumRateAcrossLabels computes the total rate for a metric, summing across all label combinations.
func sumRateAcrossLabels(prev, curr *metrics.Snapshot, metricName string) float64 {
	// Find all unique label sets for this metric in the current snapshot
	type key struct{}
	seen := make(map[string]bool)
	total := 0.0

	for _, s := range curr.Samples {
		if s.Name != metricName {
			continue
		}
		k := labelKey(s.Labels)
		if seen[k] {
			continue
		}
		seen[k] = true
		total += metrics.RatePerSecond(prev, curr, metricName, s.Labels)
	}
	return math.Max(total, 0)
}

func labelKey(labels map[string]string) string {
	// Simple key for deduplication — order doesn't matter for small label sets
	var b strings.Builder
	for key, val := range labels {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(val)
		b.WriteByte(',')
	}
	return b.String()
}
