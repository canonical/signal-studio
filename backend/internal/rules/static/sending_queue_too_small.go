package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R29: Sending queue explicitly configured too small

type SendingQueueTooSmall struct{}

func (r *SendingQueueTooSmall) ID() string { return "sending-queue-too-small" }

func (r *SendingQueueTooSmall) Description() string {
	return "Exporter sending queue is explicitly set below 500"
}

func (r *SendingQueueTooSmall) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *SendingQueueTooSmall) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if !hasNestedBool(comp.Config, "sending_queue", "enabled", true) {
			continue
		}
		size := queueSize(comp.Config)
		if size <= 0 || size >= 500 {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:     r.ID(),
			Title:      fmt.Sprintf("Exporter %s sending queue is small (%d)", name, size),
			Severity:   rules.SeverityInfo,
			Confidence: rules.ConfidenceMedium,
			Evidence:   fmt.Sprintf("Exporter %q has sending_queue.queue_size set to %d.", name, size),
			Implication: fmt.Sprintf("A queue of %d entries provides only seconds of buffering at moderate throughput. If the backend experiences a brief outage, the queue fills quickly and data is dropped.", size) +
				"\nHowever, in memory-constrained environments a smaller queue may be a deliberate trade-off to limit memory usage.",
			Scope: fmt.Sprintf("exporter:%s", name),
			Snippet: fmt.Sprintf(`exporters:
  %s:
    sending_queue:
      enabled: true
      queue_size: 5000`, name),
			Recommendation: "Increase queue_size to at least 500, or consider 5000 for high-throughput pipelines.",
		})
	}
	return findings
}

// queueSize extracts sending_queue.queue_size from an exporter config.
// Returns -1 if not set.
func queueSize(cfg map[string]any) int {
	if cfg == nil {
		return -1
	}
	sq, ok := cfg["sending_queue"]
	if !ok {
		return -1
	}
	m, ok := sq.(map[string]any)
	if !ok {
		return -1
	}
	v, ok := m["queue_size"]
	if !ok {
		return -1
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return -1
	}
}
