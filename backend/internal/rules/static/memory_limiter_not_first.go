package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R11: memory_limiter not first processor

type MemoryLimiterNotFirst struct{}

func (r *MemoryLimiterNotFirst) ID() string { return "memory-limiter-not-first" }

func (r *MemoryLimiterNotFirst) Description() string {
	return "memory_limiter is not the first processor in the pipeline"
}

func (r *MemoryLimiterNotFirst) DefaultSeverity() rules.Severity { return rules.SeverityCritical }

func (r *MemoryLimiterNotFirst) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if len(p.Processors) == 0 {
			continue
		}
		// Find index of memory_limiter
		idx := -1
		for i, proc := range p.Processors {
			if config.ComponentType(proc) == "memory_limiter" {
				idx = i
				break
			}
		}
		if idx <= 0 { // not found (-1) or already first (0)
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("memory_limiter is not the first processor in %s pipeline", name),
			Severity:     rules.SeverityCritical,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Pipeline %q processors: [%s] — memory_limiter is at index %d.", name, strings.Join(p.Processors, ", "), idx),
			Implication:    "If other processors (like batch) come first, data accumulates in memory before the limiter can apply backpressure, leading to OOM crashes. Moving memory_limiter to the first position ensures backpressure is applied before any buffering occurs.\nHowever, in rare cases a lightweight transform processor before memory_limiter may be acceptable if it does not buffer data.",

			Scope:        fmt.Sprintf("pipeline:%s", name),
			Snippet: fmt.Sprintf(`service:
  pipelines:
    %s:
      processors: [memory_limiter, ...]`, name),
			Recommendation: "Move memory_limiter to the first position in the processors list.",
		})
	}
	return findings
}
