package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// Rule 10: No log severity filtering

type NoLogSeverityFilter struct{}

func (r *NoLogSeverityFilter) ID() string { return "no-log-severity-filter" }

func (r *NoLogSeverityFilter) Description() string {
	return "Logs pipeline has no severity filtering"
}

func (r *NoLogSeverityFilter) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *NoLogSeverityFilter) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	filterTypes := []string{"filter", "transform"}

	var findings []rules.Finding
	for name, p := range cfg.Pipelines {
		if p.Signal != config.SignalLogs {
			continue
		}
		hasFilter := false
		for _, proc := range p.Processors {
			procType := config.ComponentType(proc)
			for _, ft := range filterTypes {
				if procType == ft {
					hasFilter = true
					break
				}
			}
		}
		if hasFilter {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("No log severity filtering in %s pipeline", name),
			Severity:     rules.SeverityInfo,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Logs pipeline %q has no filter or transform processor. Processors: [%s]", name, strings.Join(p.Processors, ", ")),
			Implication:    "DEBUG and INFO log floods are common cost drivers. Filtering by severity reduces volume significantly. Filtering out DEBUG logs typically reduces log volume by 30-60%.\nHowever, if all log sources already emit only important severity levels, adding a filter is unnecessary overhead.",

			Scope:        fmt.Sprintf("pipeline:%s", name),
			Snippet: `processors:
  filter/severity:
    error_mode: ignore
    logs:
      log_record:
        - 'severity_number < SEVERITY_NUMBER_INFO'`,
			Recommendation: "Add after memory_limiter, before batch processor.",
		})
	}
	return findings
}
