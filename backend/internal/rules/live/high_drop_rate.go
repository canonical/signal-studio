package live

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
)

// HighDropRate fires when (accepted - sent) / accepted > 10% for any signal,
// sustained over 2+ snapshot intervals.
// For traces pipelines with a sampler processor, drops are expected and the
// rule is skipped.
type HighDropRate struct{}

func (r *HighDropRate) ID() string { return "live-high-drop-rate" }

func (r *HighDropRate) Description() string { return "Sustained drop rate above 10% on any signal" }

func (r *HighDropRate) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *HighDropRate) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *HighDropRate) EvaluateWithMetrics(cfg *config.CollectorConfig, store *metrics.Store) []rules.Finding {
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

	hasSampler := tracesPipelineHasSampler(cfg)

	var findings []rules.Finding
	for _, sig := range signals {
		if sig.name == "traces" && hasSampler {
			continue
		}
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
			findings = append(findings, rules.Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("High drop rate on %s pipeline", sig.name),
				Severity:     rules.SeverityWarning,
				Confidence:   rules.ConfidenceMedium,
				Evidence:     fmt.Sprintf("%.1f%% of %s are being dropped", lastDropPct, sig.name),
				Implication:    "Unexpected drops indicate backpressure, misconfiguration, or overload in the pipeline. " + fmt.Sprintf("Approximately %.0f%% of %s data is being lost.", lastDropPct, sig.name) + "\nHowever, drop rate may be expected if a sampling processor is intentionally configured upstream, or during brief traffic spikes.",
				Scope:        fmt.Sprintf("signal:%s", sig.name),
				Snippet: `processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
  batch:
    send_batch_size: 512
    timeout: 5s`,
				Recommendation: "Review memory_limiter, batch, and queue settings.",
			})
		}
	}
	return findings
}
