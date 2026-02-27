package catalog

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 4: Filter keeps everything

// FilterKeepsEverything fires when a filter processor matches no metrics for dropping.
type FilterKeepsEverything struct{}

func (r *FilterKeepsEverything) ID() string { return "catalog-filter-keeps-everything" }

func (r *FilterKeepsEverything) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

func (r *FilterKeepsEverything) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	_ []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []rules.Finding {
	var findings []rules.Finding
	for _, a := range analyses {
		if a.DroppedCount == 0 && a.UnknownCount == 0 && a.PartialCount == 0 && a.KeptCount > 0 && !a.HasUnsupported {
			findings = append(findings, rules.Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Filter %q keeps all metrics", a.ProcessorName),
				Severity:     rules.SeverityInfo,
				Confidence:   rules.ConfidenceHigh,
				Evidence:     fmt.Sprintf("Processor %q in pipeline %q: %d kept, 0 dropped", a.ProcessorName, a.Pipeline, a.KeptCount),
				Implication:    "A filter that keeps everything adds processing overhead without reducing volume. Review filter rules to ensure they match the intended metrics, or remove the processor.\nHowever, the filter may be a placeholder for future rules or intentionally kept as a no-op for consistency.",

				Scope:        fmt.Sprintf("processor:%s", a.ProcessorName),
				Snippet:      "",
				Recommendation:    "Review the filter expressions or remove this processor if it is no longer needed.",
			})
		}
	}
	return findings
}
