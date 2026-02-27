package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R22: Pipeline references undefined component

type UndefinedComponentRef struct{}

func (r *UndefinedComponentRef) ID() string { return "undefined-component-ref" }

func (r *UndefinedComponentRef) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		for _, rcv := range p.Receivers {
			_, inReceivers := cfg.Receivers[rcv]
			_, inConnectors := cfg.Connectors[rcv]
			if !inReceivers && !inConnectors {
				findings = append(findings, rules.Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined receiver %s in %s pipeline", rcv, name),
					Severity:     rules.SeverityCritical,
					Confidence:   rules.ConfidenceHigh,
					Evidence:     fmt.Sprintf("Pipeline %q references receiver %q which is not defined in receivers or connectors.", name, rcv),
					Implication:    "The collector will fail to start with an undefined component reference. Define the receiver/connector or remove it from the pipeline.\nHowever, if the configuration uses environment variable substitution or includes, the component may be defined elsewhere.",

					Scope:        fmt.Sprintf("pipeline:%s", name),
					Snippet:      fmt.Sprintf("receivers:\n  %s:\n    # Add configuration here", rcv),
					Recommendation:    "Add the missing receiver to the receivers or connectors section, or remove it from the pipeline.",
				})
			}
		}
		for _, proc := range p.Processors {
			if _, ok := cfg.Processors[proc]; !ok {
				findings = append(findings, rules.Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined processor %s in %s pipeline", proc, name),
					Severity:     rules.SeverityCritical,
					Confidence:   rules.ConfidenceHigh,
					Evidence:     fmt.Sprintf("Pipeline %q references processor %q which is not defined.", name, proc),
					Implication:    "The collector will fail to start with an undefined component reference. Define the processor or remove it from the pipeline.\nHowever, if the configuration uses environment variable substitution or includes, the component may be defined elsewhere.",

					Scope:        fmt.Sprintf("pipeline:%s", name),
					Snippet:      fmt.Sprintf("processors:\n  %s:\n    # Add configuration here", proc),
					Recommendation:    "Add the missing processor to the processors section or remove it from the pipeline.",
				})
			}
		}
		for _, exp := range p.Exporters {
			_, inExporters := cfg.Exporters[exp]
			_, inConnectors := cfg.Connectors[exp]
			if !inExporters && !inConnectors {
				findings = append(findings, rules.Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined exporter %s in %s pipeline", exp, name),
					Severity:     rules.SeverityCritical,
					Confidence:   rules.ConfidenceHigh,
					Evidence:     fmt.Sprintf("Pipeline %q references exporter %q which is not defined in exporters or connectors.", name, exp),
					Implication:    "The collector will fail to start with an undefined component reference. Define the exporter/connector or remove it from the pipeline.\nHowever, if the configuration uses environment variable substitution or includes, the component may be defined elsewhere.",

					Scope:        fmt.Sprintf("pipeline:%s", name),
					Snippet:      fmt.Sprintf("exporters:\n  %s:\n    # Add configuration here", exp),
					Recommendation:    "Add the missing exporter to the exporters or connectors section, or remove it from the pipeline.",
				})
			}
		}
	}
	return findings
}
