package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R20: Exporter sending_queue not enabled

type ExporterNoSendingQueue struct{}

func (r *ExporterNoSendingQueue) ID() string { return "exporter-no-sending-queue" }

func (r *ExporterNoSendingQueue) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if hasNestedBool(comp.Config, "sending_queue", "enabled", true) {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s has no sending queue enabled", name),
			Severity:     rules.SeverityWarning,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Exporter %q does not have sending_queue.enabled: true.", name),
			Implication:    "Without a sending queue, transient backend unavailability causes immediate data loss. The queue buffers data during failures. Enabling sending_queue provides resilience against transient backend failures.\nHowever, sending queues consume memory. In memory-constrained environments, a smaller queue_size or no queue may be a deliberate trade-off.",

			Scope:        fmt.Sprintf("exporter:%s", name),
			Snippet: fmt.Sprintf(`exporters:
  %s:
    sending_queue:
      enabled: true
      queue_size: 5000`, name),
			Recommendation: "Add sending_queue configuration to the exporter.",
		})
	}
	return findings
}
