package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R23: Empty pipeline

type EmptyPipeline struct{}

func (r *EmptyPipeline) ID() string { return "empty-pipeline" }

func (r *EmptyPipeline) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if len(p.Receivers) == 0 {
			findings = append(findings, rules.Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Pipeline %s has no receivers", name),
				Severity:     rules.SeverityCritical,
				Confidence:   rules.ConfidenceHigh,
				Evidence:     fmt.Sprintf("Pipeline %q has an empty receivers list.", name),
				Implication:    "A pipeline without receivers will never receive any data. Add at least one receiver to make this pipeline functional.\nHowever, the pipeline may be a placeholder for future use or part of a staged rollout.",

				Scope:        fmt.Sprintf("pipeline:%s", name),
				Snippet:      fmt.Sprintf("service:\n  pipelines:\n    %s:\n      receivers: [otlp]", name),
				Recommendation:    "Add receivers to the pipeline.",
			})
		}
		if len(p.Exporters) == 0 {
			findings = append(findings, rules.Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Pipeline %s has no exporters", name),
				Severity:     rules.SeverityCritical,
				Confidence:   rules.ConfidenceHigh,
				Evidence:     fmt.Sprintf("Pipeline %q has an empty exporters list.", name),
				Implication:    "A pipeline without exporters receives and processes data but silently drops it all. Add at least one exporter to prevent silent data loss.\nHowever, the pipeline may be a placeholder for future use or part of a staged rollout.",

				Scope:        fmt.Sprintf("pipeline:%s", name),
				Snippet:      fmt.Sprintf("service:\n  pipelines:\n    %s:\n      exporters: [otlp]", name),
				Recommendation:    "Add exporters to the pipeline.",
			})
		}
	}
	return findings
}
