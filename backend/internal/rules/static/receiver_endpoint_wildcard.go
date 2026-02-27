package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R14: Receiver endpoint bound to 0.0.0.0

type ReceiverEndpointWildcard struct{}

func (r *ReceiverEndpointWildcard) ID() string { return "receiver-endpoint-wildcard" }

func (r *ReceiverEndpointWildcard) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Receivers {
		endpoints := extractEndpoints(comp.Config)
		for _, ep := range endpoints {
			if strings.Contains(ep, "0.0.0.0") {
				findings = append(findings, rules.Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Receiver %s binds to 0.0.0.0", name),
					Severity:     rules.SeverityWarning,
					Confidence:   rules.ConfidenceHigh,
					Evidence:     "",
					Implication:    "Binding to 0.0.0.0 exposes the receiver to untrusted networks, increasing the attack surface for DoS and data injection. Binding to localhost or a specific interface restricts access to trusted sources.\nHowever, in containerized environments, binding to 0.0.0.0 may be necessary when the container network interface is not known in advance.",

					Scope:        fmt.Sprintf("receiver:%s", name),
					Snippet:      endpointSnippet(name, ep),
					Recommendation:    "Change the endpoint to localhost or a specific trusted interface.",
				})
			}
		}
	}
	return findings
}

// endpointSnippet returns a correctly structured YAML snippet for the receiver.
// OTLP receivers nest endpoints under protocols/grpc and protocols/http;
// other receivers use a flat endpoint key.
func endpointSnippet(name, currentEndpoint string) string {
	recvType := config.ComponentType(name)
	localhost := strings.Replace(currentEndpoint, "0.0.0.0", "localhost", 1)

	if recvType == "otlp" {
		return fmt.Sprintf(`receivers:
  %s:
    protocols:
      grpc:
        endpoint: %s
      http:
        endpoint: %s`, name, localhost, strings.Replace(localhost, "4317", "4318", 1))
	}
	return fmt.Sprintf(`receivers:
  %s:
    endpoint: %s`, name, localhost)
}
