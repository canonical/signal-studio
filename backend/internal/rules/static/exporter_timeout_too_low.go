package static

import (
	"fmt"
	"time"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R28: Exporter timeout too low for production backends

type ExporterTimeoutTooLow struct{}

func (r *ExporterTimeoutTooLow) ID() string { return "exporter-timeout-too-low" }

func (r *ExporterTimeoutTooLow) Description() string {
	return "OTLP exporter timeout is below 10 seconds"
}

func (r *ExporterTimeoutTooLow) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *ExporterTimeoutTooLow) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		// Skip localhost targets — low timeouts are fine for local relays.
		for _, ep := range extractEndpoints(comp.Config) {
			if isLocalhostEndpoint(ep) {
				goto next
			}
		}
		{
			timeout := exporterTimeout(comp.Config)
			if timeout >= 10*time.Second {
				continue
			}
			evidence := fmt.Sprintf("Exporter %q uses the default 5s timeout.", name)
			if timeout > 0 {
				evidence = fmt.Sprintf("Exporter %q has timeout set to %s.", name, timeout)
			}
			findings = append(findings, rules.Finding{
				RuleID:     r.ID(),
				Title:      fmt.Sprintf("Exporter %s timeout may be too low for production", name),
				Severity:   rules.SeverityInfo,
				Confidence: rules.ConfidenceMedium,
				Evidence:   evidence,
				Implication: "A low timeout causes the exporter to abandon requests to slow backends prematurely, triggering retries that amplify load. Cross-region or heavily loaded backends often need more than 5 seconds to respond." +
					"\nHowever, for latency-sensitive setups where fast failure is preferred over waiting, a lower timeout may be intentional.",
				Scope: fmt.Sprintf("exporter:%s", name),
				Snippet: fmt.Sprintf(`exporters:
  %s:
    timeout: 30s`, name),
				Recommendation: "Set timeout to at least 10s for production backends, or 30s for cross-region exports.",
			})
		}
	next:
	}
	return findings
}

// exporterTimeout extracts the timeout duration from an exporter config.
// Returns 0 if no timeout is explicitly set (meaning the 5s default applies).
func exporterTimeout(cfg map[string]any) time.Duration {
	if cfg == nil {
		return 0
	}
	s, ok := cfg["timeout"].(string)
	if !ok {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
