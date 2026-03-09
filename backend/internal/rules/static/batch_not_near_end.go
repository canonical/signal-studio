package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R13: batch processor not near end of pipeline

type BatchNotNearEnd struct{}

func (r *BatchNotNearEnd) ID() string { return "batch-not-near-end" }

func (r *BatchNotNearEnd) Description() string {
	return "Batch processor is not near the end of the pipeline"
}

func (r *BatchNotNearEnd) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *BatchNotNearEnd) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if len(p.Processors) < 2 {
			continue
		}
		batchIdx := -1
		for i, proc := range p.Processors {
			if config.ComponentType(proc) == "batch" {
				batchIdx = i
				break
			}
		}
		if batchIdx == -1 {
			continue
		}
		// Allow batch at last or second-to-last position
		if batchIdx >= len(p.Processors)-2 {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Batch processor not near end of %s pipeline", name),
			Severity:     rules.SeverityInfo,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Pipeline %q processors: [%s] — batch at index %d of %d.", name, strings.Join(p.Processors, ", "), batchIdx, len(p.Processors)),
			Implication:    "Processors after batch operate on individual items, negating the batching benefit for compression and connection efficiency. Moving batch to the end of the pipeline optimizes export efficiency.\nHowever, processors after batch that only add attributes or set fields still work correctly; the concern is primarily about filtering or sampling after batch.",

			Scope:        fmt.Sprintf("pipeline:%s", name),
			Snippet:      "# Move batch to the last (or second-to-last) position in the processors list.",
			Recommendation:    "Move batch to the end of the pipeline, after all filtering and transformation processors.",
		})
	}
	return findings
}
