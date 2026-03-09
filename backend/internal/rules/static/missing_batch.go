package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// Rule 2: Missing batch processor

type MissingBatch struct{}

func (r *MissingBatch) ID() string { return "missing-batch" }

func (r *MissingBatch) Description() string {
	return "Pipeline has no batch processor"
}

func (r *MissingBatch) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *MissingBatch) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if rules.HasProcessorType(p.Processors, "batch") {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:      r.ID(),
			Title:       fmt.Sprintf("Missing batch processor in %s pipeline", name),
			Severity:    rules.SeverityWarning,
			Confidence:  rules.ConfidenceHigh,
			Evidence:    fmt.Sprintf("Pipeline %q processors: %s", name, strings.Join(p.Processors, ", ")),
			Implication: "Batching reduces exporter overhead and improves throughput stability by grouping telemetry before export. Reduces number of export requests and improves throughput.\nHowever, batching adds latency; real-time alerting pipelines may prefer smaller batches or no batching.",

			Scope: fmt.Sprintf("pipeline:%s", name),
			Snippet: `processors:
  batch:
    timeout: 5s
    send_batch_size: 512`,
			Recommendation: "Add batch as the last processor in the pipeline (after memory_limiter and any filtering).",
		})
	}
	return findings
}
