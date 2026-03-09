package catalog

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 5: Filter drops everything

// FilterDropsEverything fires when a filter processor drops all observed metrics.
type FilterDropsEverything struct{}

func (r *FilterDropsEverything) ID() string { return "catalog-filter-drops-everything" }

func (r *FilterDropsEverything) Description() string {
	return "Filter processor drops all observed metrics"
}

func (r *FilterDropsEverything) DefaultSeverity() rules.Severity { return rules.SeverityCritical }

func (r *FilterDropsEverything) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *FilterDropsEverything) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	_ []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []rules.Finding {
	var findings []rules.Finding
	for _, a := range analyses {
		if a.KeptCount == 0 && a.DroppedCount > 0 {
			findings = append(findings, rules.Finding{
				RuleID:     r.ID(),
				Title:      fmt.Sprintf("Filter %q drops all metrics", a.ProcessorName),
				Severity:   rules.SeverityCritical,
				Confidence: rules.ConfidenceHigh,
				Evidence: fmt.Sprintf(
					"Processor %q in pipeline %q: 0 kept, %d dropped",
					a.ProcessorName,
					a.Pipeline,
					a.DroppedCount,
				),
				Implication: "A filter that drops everything effectively disables the metrics pipeline. " + fmt.Sprintf("All %d observed metrics are being dropped.", a.DroppedCount) +
					"However, if all observed metrics are intentionally unwanted, dropping everything is correct. Verify that expected metrics have been seen.",
				Scope:          fmt.Sprintf("processor:%s", a.ProcessorName),
				Snippet:        "",
				Recommendation: "Review the filter rules — this is likely a misconfiguration.",
			})
		}
	}
	return findings
}
