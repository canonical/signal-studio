package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// ExporterEndpointLocalhost fires when a network exporter points to localhost,
// which is almost always a development leftover. Exporters named with the
// "signal-studio" qualifier are excluded — they target the OTLP sampling tap.
type ExporterEndpointLocalhost struct{}

func (r *ExporterEndpointLocalhost) ID() string { return "exporter-endpoint-localhost" }

func (r *ExporterEndpointLocalhost) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if config.ComponentQualifier(name) == "signal-studio" {
			continue
		}
		if comp.Config == nil {
			continue
		}
		endpoint, ok := comp.Config["endpoint"].(string)
		if !ok {
			continue
		}
		if !isLocalhostEndpoint(endpoint) {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s points to localhost", name),
			Severity:     rules.SeverityInfo,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Exporter %q has endpoint: %s", name, endpoint),
			Implication:    "In production, the exporter should point to the actual backend address. Sending to localhost means telemetry data is either lost or never leaves the host. Update the endpoint to the production backend address.\nHowever, localhost is valid when the backend runs as a sidecar or on the same host.",

			Scope:        fmt.Sprintf("exporter:%s", name),
			Snippet: fmt.Sprintf(`exporters:
  %s:
    endpoint: <backend-host>:4317`, name),
			Recommendation: "Replace the localhost endpoint with the production backend address.",
		})
	}
	return findings
}
