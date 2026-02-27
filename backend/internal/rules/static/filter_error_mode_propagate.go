package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R24: filter/transform processor with error_mode propagate

type FilterErrorModePropagateRule struct{}

func (r *FilterErrorModePropagateRule) ID() string { return "filter-error-mode-propagate" }

func (r *FilterErrorModePropagateRule) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	riskyTypes := map[string]bool{"filter": true, "transform": true}
	var findings []rules.Finding
	for name, comp := range cfg.Processors {
		if !riskyTypes[comp.Type] {
			continue
		}
		// Check if error_mode is set to something other than propagate
		if em, ok := comp.Config["error_mode"]; ok {
			if s, ok := em.(string); ok && s != "propagate" {
				continue
			}
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Processor %s uses error_mode propagate", name),
			Severity:     rules.SeverityWarning,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Processor %q has error_mode set to propagate (or unset, which defaults to propagate).", name),
			Implication:    "A single typo in a filter condition can cause 100%% data loss. Setting error_mode to ignore logs the error and continues processing. Changing to error_mode: ignore prevents silent total data loss from condition errors.\nHowever, if strict error handling is desired (e.g. to catch condition typos during development), propagate mode is appropriate.",

			Scope:        fmt.Sprintf("processor:%s", name),
			Snippet: fmt.Sprintf(`processors:
  %s:
    error_mode: ignore`, name),
			Recommendation: "Add error_mode: ignore to the processor configuration.",
		})
	}
	return findings
}
