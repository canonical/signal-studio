package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// ExporterNoCompression fires when an OTLP exporter does not have compression
// configured, wasting network bandwidth.
type ExporterNoCompression struct{}

func (r *ExporterNoCompression) ID() string { return "exporter-no-compression" }

func (r *ExporterNoCompression) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if comp.Config == nil {
			continue
		}
		// Only flag if explicitly set to "none" or "".
		// If the key is absent, the exporter uses the default (gzip), which is fine.
		c, ok := comp.Config["compression"].(string)
		if !ok {
			continue
		}
		if c != "none" && c != "" {
			continue
		}
		findings = append(findings, rules.Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s has compression disabled", name),
			Severity:     rules.SeverityInfo,
			Confidence:   rules.ConfidenceHigh,
			Evidence:     fmt.Sprintf("Exporter %q has compression: %q.", name, c),
			Implication:    "Telemetry data compresses well (often 5-10x). Disabling compression wastes network bandwidth and increases transfer time. Enabling gzip compression significantly reduces network usage with minimal CPU overhead.\nHowever, if the backend does not support gzip, disabling compression avoids decompression errors.",

			Scope:        fmt.Sprintf("exporter:%s", name),
			Snippet: fmt.Sprintf(`exporters:
  %s:
    compression: gzip`, name),
			Recommendation: "Set compression to gzip or remove the field to use the default.",
		})
	}
	return findings
}
