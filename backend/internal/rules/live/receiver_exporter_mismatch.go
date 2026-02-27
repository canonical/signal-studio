package live

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
)

// ReceiverExporterMismatch fires when accepted >> sent for a signal, sustained.
// For traces pipelines with a sampler processor, the mismatch is expected and
// the rule is skipped.
type ReceiverExporterMismatch struct{}

func (r *ReceiverExporterMismatch) ID() string { return "live-receiver-exporter-mismatch" }

func (r *ReceiverExporterMismatch) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *ReceiverExporterMismatch) EvaluateWithMetrics(cfg *config.CollectorConfig, store *metrics.Store) []rules.Finding {
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

	hasSampler := tracesPipelineHasSampler(cfg)

	var findings []rules.Finding
	for _, sig := range signals {
		if sig.name == "traces" && hasSampler {
			continue
		}
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
			findings = append(findings, rules.Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Receiver-exporter mismatch on %s", sig.name),
				Severity:     rules.SeverityWarning,
				Confidence:   rules.ConfidenceMedium,
				Evidence:     fmt.Sprintf("Accepted: %.0f/s, Sent: %.0f/s for %s", lastAccepted, lastSent, sig.name),
				Implication: fmt.Sprintf("This suggests drops, pipeline blockage, or exporter failure. %.0f%% of %s may be lost between receiver and exporter.", ((lastAccepted-lastSent)/lastAccepted)*100, sig.name) + "\nHowever, a mismatch may be expected if processors intentionally filter or aggregate data, reducing the volume between receiver and exporter.",
				Scope:        fmt.Sprintf("signal:%s", sig.name),
				Snippet:      "",
				Recommendation:    "Check processor dropped counters, exporter errors, and queue utilization.",
			})
		}
	}
	return findings
}
