package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R19: memory_limiter without limit_mib or limit_percentage

type MemoryLimiterWithoutLimits struct{}

func (r *MemoryLimiterWithoutLimits) ID() string { return "memory-limiter-without-limits" }

func (r *MemoryLimiterWithoutLimits) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Processors {
		if config.ComponentType(name) != "memory_limiter" {
			continue
		}
		_, hasLimitMib := comp.Config["limit_mib"]
		_, hasLimitPct := comp.Config["limit_percentage"]
		if hasLimitMib || hasLimitPct {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("memory_limiter %s has no limit configured", name),
			Severity:     rules.SeverityCritical,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Processor %q has neither limit_mib nor limit_percentage set.", name),
			Implication:    "Without limit_mib or limit_percentage, the memory limiter does nothing, creating a false sense of security. Setting a limit enables actual memory protection and backpressure.\nHowever, if memory limits are managed externally (e.g. cgroup limits in Kubernetes), the processor may still provide value via check_interval alone.",

			Scope:        fmt.Sprintf("processor:%s", name),
			Snippet: fmt.Sprintf(`processors:
  %s:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128`, name),
			Recommendation: "Add limit_mib or limit_percentage to the memory_limiter configuration.",
		})
	}
	return findings
}
