package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R18: pprof extension enabled

type PprofExtensionEnabled struct{}

func (r *PprofExtensionEnabled) ID() string { return "pprof-extension-enabled" }

func (r *PprofExtensionEnabled) Description() string { return "pprof extension is enabled" }

func (r *PprofExtensionEnabled) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *PprofExtensionEnabled) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	for _, ext := range cfg.ServiceExtensions {
		if config.ComponentType(ext) == "pprof" {
			return []rules.Finding{{
				RuleID:       r.ID(),
				Title:        "pprof extension is enabled",
				Severity:     rules.SeverityInfo,
				Confidence:   rules.ConfidenceHigh,
				Evidence:     fmt.Sprintf("Service extensions include %q.", ext),
				Implication:    "In production, pprof can be used to gather operational intelligence or trigger expensive profiling that degrades performance. Disabling pprof in production reduces the attack surface.\nHowever, pprof is valuable for production debugging when behind proper access controls. Disabling it reduces observability of the collector itself.",

				Scope:        fmt.Sprintf("extension:%s", ext),
				Snippet: `# Remove pprof from service extensions:
service:
  extensions: [health_check]`,
				Recommendation: "Remove pprof from the service extensions list in production.",
			}}
		}
	}
	return nil
}
