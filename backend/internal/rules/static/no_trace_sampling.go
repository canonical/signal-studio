package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// Rule 3: No trace sampling configured

type NoTraceSampling struct{}

func (r *NoTraceSampling) ID() string { return "no-trace-sampling" }

func (r *NoTraceSampling) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	samplingProcessors := []string{
		"probabilistic_sampler",
		"tail_sampling",
	}

	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if p.Signal != config.SignalTraces {
			continue
		}
		hasSampling := false
		for _, proc := range p.Processors {
			procType := config.ComponentType(proc)
			for _, sp := range samplingProcessors {
				if procType == sp {
					hasSampling = true
					break
				}
			}
		}
		if hasSampling {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("No sampling configured in %s pipeline", name),
			Severity:     rules.SeverityWarning,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Traces pipeline %q has no sampling processor. Processors: [%s]", name, strings.Join(p.Processors, ", ")),
			Implication:    "High trace volume is a primary cost driver. Sampling reduces volume while preserving representative data. Probabilistic sampling at 20% would reduce trace export volume by ~80%.\nHowever, some environments require 100% trace capture for compliance or debugging. Sampling is not always appropriate.",

			Scope:        fmt.Sprintf("pipeline:%s", name),
			Snippet: `processors:
  probabilistic_sampler:
    sampling_percentage: 20`,
			Recommendation: "Add after memory_limiter but before batch processor.",
		})
	}
	return findings
}
