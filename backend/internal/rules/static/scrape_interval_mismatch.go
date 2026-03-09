package static

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// ScrapeIntervalMismatch fires when receivers in the same metrics pipeline use
// different scrape or collection intervals, causing uneven data density.
type ScrapeIntervalMismatch struct{}

func (r *ScrapeIntervalMismatch) ID() string { return "scrape-interval-mismatch" }

func (r *ScrapeIntervalMismatch) Description() string {
	return "Receivers in the same metrics pipeline use different scrape intervals"
}

func (r *ScrapeIntervalMismatch) DefaultSeverity() rules.Severity { return rules.SeverityWarning }

func (r *ScrapeIntervalMismatch) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding
	for pName, p := range cfg.Pipelines {
		if p.Signal != config.SignalMetrics {
			continue
		}
		intervals := collectReceiverIntervals(cfg, p.Receivers)
		if len(intervals) < 2 {
			continue
		}

		// Deduplicate
		unique := map[time.Duration]bool{}
		for _, iv := range intervals {
			unique[iv.interval] = true
		}
		if len(unique) < 2 {
			continue
		}

		sort.Slice(intervals, func(i, j int) bool {
			return intervals[i].interval < intervals[j].interval
		})
		var parts []string
		for _, iv := range intervals {
			parts = append(parts, fmt.Sprintf("%s (%s)", iv.receiver, iv.interval))
		}
		shortest := intervals[0]
		longest := intervals[len(intervals)-1]

		findings = append(findings, rules.Finding{
			RuleID:     r.ID(),
			Title:      fmt.Sprintf("Mismatched scrape intervals in %s pipeline", pName),
			Severity:   rules.SeverityWarning,
			Confidence: rules.ConfidenceHigh,
			Evidence:   fmt.Sprintf("Receivers have different intervals: %s", strings.Join(parts, ", ")),
			Implication:    fmt.Sprintf(
				"Receiver %s collects every %s while %s collects every %s. "+
					"Mismatched intervals cause inconsistent granularity on dashboards and can trigger misleading alerts when data from different receivers is correlated. "+
					"Aligning intervals makes data easier to reason about and avoids sparse-series artifacts.",
				shortest.receiver, shortest.interval, longest.receiver, longest.interval) + "\nHowever, different intervals may be intentional if receivers monitor sources with different update frequencies.",

			Scope:  fmt.Sprintf("pipeline:%s", pName),
			Snippet: fmt.Sprintf(`receivers:
  %s:
    collection_interval: %s`, longest.receiver, shortest.interval),
			Recommendation: "Align collection/scrape intervals across receivers in the same pipeline.",
		})
	}
	return findings
}
