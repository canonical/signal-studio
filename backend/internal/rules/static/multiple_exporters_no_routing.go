package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// Rule 9: Multiple exporters without routing clarity

type MultipleExportersNoRouting struct{}

func (r *MultipleExportersNoRouting) ID() string { return "multiple-exporters-no-routing" }

func (r *MultipleExportersNoRouting) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if len(p.Exporters) < 2 {
			continue
		}
		if hasRoutingConnector(cfg, p.Exporters) {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:     r.ID(),
			Title:      "Multiple exporters lack routing",
			Severity:   rules.SeverityInfo,
			Confidence: rules.ConfidenceHigh,
			Evidence: fmt.Sprintf("Pipeline %q has %d exporters [%s] but no routing connector.",
				name, len(p.Exporters), strings.Join(p.Exporters, ", ")),
			Implication: "This can unintentionally duplicate telemetry, increase cost, and complicate troubleshooting. Adding a routing connector or splitting into separate pipelines gives explicit control over data flow.\nHowever, duplicating data to multiple backends may be intentional for redundancy or migration purposes.",

			Scope: fmt.Sprintf("pipeline:%s", name),
			Snippet: `connectors:
  routing:
    match_once: true
    default_pipelines: [traces/primary]
    table:
      - condition: attributes["env"] == "production"
        pipelines: [traces/secondary]

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [routing]
    traces/primary:
      receivers: [routing]
      exporters: [otlp/primary]
    traces/secondary:
      receivers: [routing]
      exporters: [otlp/secondary]`,
			Recommendation: "Replace the multi-exporter pipeline with a routing connector that fans out to sub-pipelines.",
		})
	}
	return findings
}

// hasRoutingConnector checks if any exporter in the list is a routing connector.
func hasRoutingConnector(cfg *config.CollectorConfig, exporters []string) bool {
	for _, exp := range exporters {
		if config.ComponentType(exp) == "routing" {
			if _, ok := cfg.Connectors[exp]; ok {
				return true
			}
		}
	}
	return false
}
