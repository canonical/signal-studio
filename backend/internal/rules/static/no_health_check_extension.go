package static

import (
	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// NoHealthCheckExtension fires when no health_check extension is configured.
type NoHealthCheckExtension struct{}

func (r *NoHealthCheckExtension) ID() string { return "no-health-check-extension" }

func (r *NoHealthCheckExtension) Description() string {
	return "Configuration has no health_check extension"
}

func (r *NoHealthCheckExtension) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *NoHealthCheckExtension) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	for name := range cfg.Extensions {
		if config.ComponentType(name) == "health_check" {
			return nil
		}
	}
	// Also check if referenced in service extensions
	for _, ext := range cfg.ServiceExtensions {
		if config.ComponentType(ext) == "health_check" {
			return nil
		}
	}
	return []rules.Finding{{
		RuleID:       r.ID(),
		Title:        "No health check extension configured",
		Severity:     rules.SeverityWarning,
		Confidence:   rules.ConfidenceHigh,
		Evidence:     "No health_check extension found in the configuration.",
		Implication:    "Without a health check, orchestrators like Kubernetes cannot detect collector failures, leading to silent data loss as traffic continues to route to an unhealthy instance. Adding a health check extension enables automated failure detection and recovery.\nHowever, if the collector runs outside an orchestrator (e.g. bare metal with manual monitoring), a health check extension may be unnecessary.",

		Scope:        "all pipelines",
		Snippet: `extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]`,
		Recommendation: "Add a health_check extension and reference it in the service extensions list.",
	}}
}
