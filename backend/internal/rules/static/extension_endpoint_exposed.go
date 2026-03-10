package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// R31: Extension endpoint bound to wildcard address

type ExtensionEndpointExposed struct{}

func (r *ExtensionEndpointExposed) ID() string { return "extension-endpoint-exposed" }

func (r *ExtensionEndpointExposed) Description() string {
	return "Admin extension endpoint is bound to a wildcard address"
}

func (r *ExtensionEndpointExposed) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

// adminExtensionTypes are extension types that expose admin/debug endpoints.
var adminExtensionTypes = map[string]bool{
	"zpages":       true,
	"pprof":        true,
	"health_check": true,
}

func (r *ExtensionEndpointExposed) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for _, extName := range cfg.ServiceExtensions {
		extType := config.ComponentType(extName)
		if !adminExtensionTypes[extType] {
			continue
		}
		comp, ok := cfg.Extensions[extName]
		if !ok {
			continue
		}
		for _, ep := range extractEndpoints(comp.Config) {
			if isWildcardEndpoint(ep) {
				findings = append(findings, rules.Finding{
					RuleID:     r.ID(),
					Title:      fmt.Sprintf("Extension %s is exposed on a wildcard address", extName),
					Severity:   rules.SeverityWarning,
					Confidence: rules.ConfidenceHigh,
					Evidence:   fmt.Sprintf("Extension %q has endpoint %q, which binds to all network interfaces.", extName, ep),
					Implication: fmt.Sprintf("The %s extension exposes operational data on all interfaces, making it accessible from any network the host is connected to. This can leak internal state or enable profiling by unauthorized parties.", extType) +
						"\nHowever, in containerized environments behind a network policy or service mesh, wildcard binding may be acceptable for internal health probes.",
					Scope: fmt.Sprintf("extension:%s", extName),
					Snippet: fmt.Sprintf(`extensions:
  %s:
    endpoint: localhost%s`, extName, portFromEndpoint(ep)),
					Recommendation: fmt.Sprintf("Bind %s to localhost instead of 0.0.0.0.", extName),
				})
				break
			}
		}
	}
	return findings
}

// isWildcardEndpoint checks if an endpoint string binds to all interfaces.
func isWildcardEndpoint(ep string) bool {
	// Strip scheme if present.
	clean := ep
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(clean, prefix) {
			clean = clean[len(prefix):]
			break
		}
	}
	return strings.HasPrefix(clean, "0.0.0.0") ||
		strings.HasPrefix(clean, "::") ||
		strings.HasPrefix(clean, "[::]")
}

// portFromEndpoint extracts the :port suffix from an endpoint string.
func portFromEndpoint(ep string) string {
	idx := strings.LastIndex(ep, ":")
	if idx < 0 {
		return ""
	}
	port := ep[idx:]
	// Sanity check: must look like :1234
	if len(port) < 2 || port[1] < '0' || port[1] > '9' {
		return ""
	}
	return port
}
