package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// ExporterInsecureTLS fires when an exporter has tls.insecure: true, sending
// telemetry data unencrypted over the network.
type ExporterInsecureTLS struct{}

func (r *ExporterInsecureTLS) ID() string { return "exporter-insecure-tls" }

func (r *ExporterInsecureTLS) Description() string {
	return "Exporter has TLS verification disabled"
}

func (r *ExporterInsecureTLS) DefaultSeverity() rules.Severity { return rules.SeverityCritical }

func (r *ExporterInsecureTLS) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if config.ComponentQualifier(name) == "signal-studio" {
			continue
		}
		if !hasNestedBool(comp.Config, "tls", "insecure", true) {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s uses insecure TLS", name),
			Severity:     rules.SeverityCritical,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Exporter %q has tls.insecure: true.", name),
			Implication:    "Unencrypted telemetry may contain sensitive information such as HTTP headers, database queries, or user identifiers. Transmitting this in plaintext exposes it to network interception. Enabling TLS encrypts data in transit and prevents eavesdropping.\nHowever, in development or when using a service mesh with mTLS, insecure TLS may be acceptable.",

			Scope:        fmt.Sprintf("exporter:%s", name),
			Snippet: fmt.Sprintf(`exporters:
  %s:
    tls:
      insecure: false
      # cert_file: /path/to/cert.pem
      # key_file: /path/to/key.pem`, name),
			Recommendation: "Remove tls.insecure or set it to false, and configure proper TLS certificates.",
		})
	}
	return findings
}
