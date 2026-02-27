package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R15: Debug exporter in pipeline

type DebugExporterInPipeline struct{}

func (r *DebugExporterInPipeline) ID() string { return "debug-exporter-in-pipeline" }

func (r *DebugExporterInPipeline) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	debugExporters := map[string]bool{"debug": true, "logging": true}
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		for _, exp := range p.Exporters {
			if debugExporters[config.ComponentType(exp)] {
				findings = append(findings, rules.Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Debug exporter %s in %s pipeline", exp, name),
					Severity:     rules.SeverityWarning,
					Confidence:   rules.ConfidenceHigh,
					Evidence:     fmt.Sprintf("Pipeline %q includes exporter %q.", name, exp),
					Implication:    "The debug exporter prints all telemetry to stdout, causing performance overhead in production and potentially exposing sensitive data in log files. Removing the debug exporter reduces I/O overhead and prevents sensitive data leakage.\nHowever, the debug exporter may be intentionally enabled for troubleshooting. Consider using verbosity: basic to limit output.",

					Scope:        fmt.Sprintf("pipeline:%s", name),
					Snippet:      fmt.Sprintf("# Remove %q from the exporters list in the %s pipeline.", exp, name),
					Recommendation:    "Remove the debug exporter from production pipeline configurations.",
				})
			}
		}
	}
	return findings
}
