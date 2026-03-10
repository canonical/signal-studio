package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// DebugLogFeedbackLoop fires when a logs pipeline has a debug/logging exporter
// and a filelog or journald receiver is also configured — creating a risk of
// infinite feedback where the debug output is re-collected as new log records.
type DebugLogFeedbackLoop struct{}

func (r *DebugLogFeedbackLoop) ID() string { return "debug-log-feedback-loop" }

func (r *DebugLogFeedbackLoop) Description() string {
	return "Debug exporter in logs pipeline risks infinite feedback via log collection"
}

func (r *DebugLogFeedbackLoop) DefaultSeverity() rules.Severity { return rules.SeverityCritical }

// logCollectorTypes are receiver types that collect logs from the host where
// the collector runs, making them candidates for re-ingesting debug output.
var logCollectorTypes = map[string]bool{
	"filelog":  true,
	"journald": true,
}

func (r *DebugLogFeedbackLoop) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	// Check if any receiver could re-collect the collector's own output.
	hasLogCollector := false
	for name := range cfg.Receivers {
		if logCollectorTypes[config.ComponentType(name)] {
			hasLogCollector = true
			break
		}
	}
	if !hasLogCollector {
		return nil
	}

	debugExporters := map[string]bool{"debug": true, "logging": true}
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if p.Signal != config.SignalLogs {
			continue
		}
		for _, exp := range p.Exporters {
			if !debugExporters[config.ComponentType(exp)] {
				continue
			}
			findings = append(findings, rules.Finding{
				RuleID:     r.ID(),
				Title:      fmt.Sprintf("Debug exporter in logs pipeline %s may cause infinite feedback", name),
				Severity:   rules.SeverityCritical,
				Confidence: rules.ConfidenceHigh,
				Evidence:   fmt.Sprintf("Pipeline %q exports logs via %q while a log-collecting receiver is configured.", name, exp),
				Implication: "The debug exporter writes every log record to stdout. If a filelog or journald receiver is collecting the collector's own output, each exported log generates a new log record that is re-ingested, creating an infinite loop. This causes runaway CPU, memory, and disk usage." +
					"\nHowever, if the log-collecting receiver is explicitly configured to exclude the collector's own log files, this loop cannot occur.",
				Scope: fmt.Sprintf("pipeline:%s", name),
				Snippet: fmt.Sprintf(`# Either remove the debug exporter from the logs pipeline:
service:
  pipelines:
    %s:
      exporters: [otlp/backend]

# Or exclude the collector's log file from filelog:
receivers:
  filelog:
    exclude:
      - /var/log/collector/*.log`, name),
				Recommendation: "Remove the debug exporter from the logs pipeline, or configure the log receiver to exclude the collector's own output.",
			})
			break
		}
	}
	return findings
}
