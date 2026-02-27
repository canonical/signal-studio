package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// Rule 8: Unused components

type UnusedComponents struct{}

func (r *UnusedComponents) ID() string { return "unused-components" }

func (r *UnusedComponents) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	usedReceivers := make(map[string]bool)
	usedProcessors := make(map[string]bool)
	usedExporters := make(map[string]bool)

	for _, p := range cfg.Pipelines {
		for _, r := range p.Receivers {
			usedReceivers[r] = true
		}
		for _, proc := range p.Processors {
			usedProcessors[proc] = true
		}
		for _, e := range p.Exporters {
			usedExporters[e] = true
		}
	}

	var findings []rules.Finding

	for name := range cfg.Receivers {
		if !usedReceivers[name] {
			findings = append(findings, unusedFinding(r.ID(), "receiver", name))
		}
	}
	for name := range cfg.Processors {
		if !usedProcessors[name] {
			findings = append(findings, unusedFinding(r.ID(), "processor", name))
		}
	}
	for name := range cfg.Exporters {
		if !usedExporters[name] {
			findings = append(findings, unusedFinding(r.ID(), "exporter", name))
		}
	}

	return findings
}

func unusedFinding(ruleID, kind, name string) rules.Finding {
	return rules.Finding{
		RuleID:       ruleID,
		Title:        fmt.Sprintf("Unused %s: %s", kind, name),
		Severity:     rules.SeverityInfo,
		Confidence:   rules.ConfidenceHigh,
		Evidence:     fmt.Sprintf("%s %q is defined but not referenced by any pipeline.", kind, name),
		Implication:    "Unused components add confusion and increase configuration drift over time. Removing unused components simplifies the configuration.\nHowever, the component may be referenced by an external orchestration tool or conditionally included via environment variable substitution.",

		Scope:        fmt.Sprintf("%s:%s", kind, name),
		Snippet:      fmt.Sprintf("# Remove the unused %s:\n# %s: ...", kind, name),
		Recommendation:    fmt.Sprintf("Remove the %q block from the %ss section.", name, kind),
	}
}
