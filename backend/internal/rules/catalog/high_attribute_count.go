package catalog

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 2: High attribute count

// HighAttributeCount fires when a metric has more than 10 attribute keys.
type HighAttributeCount struct{}

func (r *HighAttributeCount) ID() string { return "catalog-high-attribute-count" }

func (r *HighAttributeCount) Description() string {
	return "Metric has more than 10 attribute keys"
}

func (r *HighAttributeCount) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *HighAttributeCount) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *HighAttributeCount) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	entries []tap.MetricEntry,
	_ []filter.FilterAnalysis,
) []rules.Finding {
	var findings []rules.Finding
	for _, e := range entries {
		if len(e.AttributeKeys) > 10 {
			findings = append(findings, rules.Finding{
				RuleID:     r.ID(),
				Title:      fmt.Sprintf("High attribute count on %s", e.Name),
				Severity:   rules.SeverityWarning,
				Confidence: rules.ConfidenceMedium,
				Evidence: fmt.Sprintf("Metric %q has %d attribute keys: %s",
					e.Name, len(e.AttributeKeys), strings.Join(e.AttributeKeys, ", ")),
				Implication:    "Each unique combination of attribute values creates a separate time series. High attribute counts multiply cardinality exponentially. " + fmt.Sprintf("Reducing attributes on %q could significantly lower series count.", e.Name) + "\nHowever, high attribute count may be normal for metrics that represent rich context. Check if all attributes are actively used in queries.",
				Scope:        fmt.Sprintf("metric:%s", e.Name),
				Snippet: `processors:
  transform:
    metric_statements:
      - context: datapoint
        statements:
          - delete_key(attributes, "unnecessary_key")`,
				Recommendation: "Use a transform processor to remove low-value attributes.",
			})
		}
	}
	return findings
}
