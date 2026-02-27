package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// Rule 1: Missing memory_limiter processor

type MissingMemoryLimiter struct{}

func (r *MissingMemoryLimiter) ID() string { return "missing-memory-limiter" }

func (r *MissingMemoryLimiter) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if rules.HasProcessorType(p.Processors, "memory_limiter") {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Missing memory_limiter in %s pipeline", name),
			Severity:     rules.SeverityCritical,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     "",
			Implication:    "Without memory_limiter, the collector can experience uncontrolled memory growth and OOM kills under load. The processor enforces limits and applies backpressure when memory usage is high.\nHowever, if the collector runs behind a load balancer with its own backpressure mechanism, the risk may be partially mitigated.",

			Scope:        fmt.Sprintf("pipeline:%s", name),
			Snippet: `processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128`,
			Recommendation: "Add memory_limiter as the first processor in the pipeline.",
		})
	}
	return findings
}
