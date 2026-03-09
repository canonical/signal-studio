package live

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
)

// LogVolumeDominance fires when log ingest rate exceeds 3x trace ingest rate.
type LogVolumeDominance struct{}

func (r *LogVolumeDominance) ID() string { return "live-log-volume-dominance" }

func (r *LogVolumeDominance) Description() string { return "Log ingest rate exceeds 3x trace ingest rate" }

func (r *LogVolumeDominance) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *LogVolumeDominance) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *LogVolumeDominance) EvaluateWithMetrics(_ *config.CollectorConfig, store *metrics.Store) []rules.Finding {
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
	return []rules.Finding{{
		RuleID:       r.ID(),
		Title:        "Logs dominating telemetry volume",
		Severity:     rules.SeverityInfo,
		Confidence:   rules.ConfidenceMedium,
		Evidence:     fmt.Sprintf("Log ingest: %.0f/s, Trace ingest: %.0f/s (%.1fx ratio)", logRate, traceRate, ratio),
		Implication:    "Log volume frequently dominates telemetry cost. Many logs are low-value (DEBUG/INFO level). " + fmt.Sprintf("Logs are consuming %.1fx more pipeline capacity than traces.", ratio) + "\nHowever, high log volume may be expected during incidents or for log-heavy workloads. Check if the ratio is stable over time.",
		Scope:        "signal:logs",
		Snippet: `processors:
  filter/logs:
    error_mode: ignore
    logs:
      log_record:
        - 'severity_number < SEVERITY_NUMBER_WARN'`,
		Recommendation: "Add a severity filter processor to the logs pipeline before the batch processor.",
	}}
}
