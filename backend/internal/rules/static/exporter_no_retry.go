package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R21: Exporter retry_on_failure not enabled

type ExporterNoRetry struct{}

func (r *ExporterNoRetry) ID() string { return "exporter-no-retry" }

func (r *ExporterNoRetry) Description() string {
	return "Exporter has no retry_on_failure configured"
}

func (r *ExporterNoRetry) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *ExporterNoRetry) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if hasNestedBool(comp.Config, "retry_on_failure", "enabled", true) {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s has no retry on failure enabled", name),
			Severity:     rules.SeverityWarning,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Exporter %q does not have retry_on_failure.enabled: true.", name),
			Implication:    "Without retry, any transient network error causes permanent data loss for the affected batch. Enabling retry_on_failure with exponential backoff recovers from transient failures.\nHowever, retries increase memory usage and can cause duplicate data if the backend does not deduplicate. Idempotent backends handle this best.",

			Scope:        fmt.Sprintf("exporter:%s", name),
			Snippet: fmt.Sprintf(`exporters:
  %s:
    retry_on_failure:
      enabled: true
      max_elapsed_time: 300s`, name),
			Recommendation: "Add retry_on_failure configuration to the exporter.",
		})
	}
	return findings
}
