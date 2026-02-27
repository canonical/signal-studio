package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R12: batch processor before sampling

type BatchBeforeSampling struct{}

func (r *BatchBeforeSampling) ID() string { return "batch-before-sampling" }

func (r *BatchBeforeSampling) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	samplers := map[string]bool{
		"tail_sampling":         true,
		"probabilistic_sampler": true,
	}
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		batchIdx := -1
		firstSamplerIdx := -1
		for i, proc := range p.Processors {
			pt := config.ComponentType(proc)
			if pt == "batch" && batchIdx == -1 {
				batchIdx = i
			}
			if samplers[pt] && firstSamplerIdx == -1 {
				firstSamplerIdx = i
			}
		}
		if batchIdx == -1 || firstSamplerIdx == -1 || batchIdx > firstSamplerIdx {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:     r.ID(),
			Title:      fmt.Sprintf("Batch processor before sampling in %s pipeline", name),
			Severity:   rules.SeverityCritical,
			Confidence: rules.ConfidenceHigh,
			Evidence:   fmt.Sprintf("Pipeline %q processors: [%s] — batch at index %d, sampler at index %d.", name, strings.Join(p.Processors, ", "), batchIdx, firstSamplerIdx),
			Implication: "Batch can split spans from the same trace into different batches, causing " +
				"the sampler to see incomplete traces and make incorrect decisions. Moving batch after " +
				"sampling ensures trace-complete sampling decisions.\n\n" +
				"However, for probabilistic (head) sampling, batch ordering matters less since decisions " +
				"are per-span, not per-trace.",

			Scope: fmt.Sprintf("pipeline:%s", name),
			Snippet: fmt.Sprintf(`service:
  pipelines:
    %s:
      processors: [memory_limiter, tail_sampling, batch]`, name),
			Recommendation: "Move batch processor after all sampling processors.",
		})
	}
	return findings
}
