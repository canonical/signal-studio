package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// TailSamplingWithoutMemoryLimiter fires when a traces pipeline has
// tail_sampling but no memory_limiter, risking uncontrolled memory growth.
type TailSamplingWithoutMemoryLimiter struct{}

func (r *TailSamplingWithoutMemoryLimiter) ID() string {
	return "tail-sampling-without-memory-limiter"
}

func (r *TailSamplingWithoutMemoryLimiter) Description() string {
	return "tail_sampling processor without memory_limiter in traces pipeline"
}

func (r *TailSamplingWithoutMemoryLimiter) DefaultSeverity() rules.Severity {
	return rules.SeverityWarning
}

func (r *TailSamplingWithoutMemoryLimiter) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if p.Signal != config.SignalTraces {
			continue
		}
		hasTailSampling := false
		hasMemoryLimiter := false
		for _, proc := range p.Processors {
			pt := config.ComponentType(proc)
			if pt == "tail_sampling" {
				hasTailSampling = true
			}
			if pt == "memory_limiter" {
				hasMemoryLimiter = true
			}
		}
		if !hasTailSampling || hasMemoryLimiter {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Tail sampling without memory limiter in %s pipeline", name),
			Severity:     rules.SeverityWarning,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Pipeline %q has tail_sampling but no memory_limiter processor.", name),
			Implication:    "Without a memory limiter, a traffic spike can cause unbounded memory growth and OOM kills. Tail sampling is particularly memory-intensive because it buffers entire traces. Adding memory_limiter before tail_sampling provides backpressure and prevents OOM crashes.\nHowever, if the trace volume is known to be low and stable, the OOM risk may be negligible.",

			Scope:        fmt.Sprintf("pipeline:%s", name),
			Snippet: `processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
  tail_sampling:
    decision_wait: 10s
    policies: [...]`,
			Recommendation: "Add memory_limiter as the first processor in the pipeline, before tail_sampling.",
		})
	}
	return findings
}
