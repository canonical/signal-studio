package alertcoverage

import (
	"regexp"
	"strings"

	"github.com/canonical/signal-studio/internal/filter"
)

// Analyze cross-references parsed alert rules against filter analysis results
// and returns a CoverageReport. If knownMetrics is non-nil (tap is active),
// metrics not present in any filter AND not in the catalog are marked unknown.
func Analyze(rules []AlertRule, analyses []filter.FilterAnalysis, knownMetrics map[string]struct{}) *CoverageReport {
	// Build a lookup from metric name to its worst (most restrictive) outcome
	// across all filter processors.
	outcomes := buildOutcomeMap(analyses)

	var results []AlertCoverageResult
	for _, r := range rules {
		if r.Type != "alert" {
			continue
		}
		result := evaluateAlert(r, outcomes, knownMetrics)
		results = append(results, result)
	}

	return &CoverageReport{
		Results: results,
		Summary: buildCoverageSummary(results),
	}
}

// outcomeMap maps metric names to their composed filter outcome.
type outcomeMap map[string]filter.MatchOutcome

func buildOutcomeMap(analyses []filter.FilterAnalysis) outcomeMap {
	om := make(outcomeMap)
	for _, a := range analyses {
		for _, r := range a.Results {
			existing, ok := om[r.MetricName]
			if !ok {
				om[r.MetricName] = r.Outcome
				continue
			}
			// Compose: a metric must survive ALL filter processors.
			// If any processor drops it, it's dropped.
			om[r.MetricName] = composeOutcome(existing, r.Outcome)
		}
	}
	return om
}

// composeOutcome composes two filter outcomes sequentially.
// A metric must survive all processors, so dropped wins over kept.
func composeOutcome(a, b filter.MatchOutcome) filter.MatchOutcome {
	if a == filter.OutcomeDropped || b == filter.OutcomeDropped {
		return filter.OutcomeDropped
	}
	if a == filter.OutcomePartial || b == filter.OutcomePartial {
		return filter.OutcomePartial
	}
	if a == filter.OutcomeUnknown || b == filter.OutcomeUnknown {
		return filter.OutcomeUnknown
	}
	return filter.OutcomeKept
}

func evaluateAlert(r AlertRule, outcomes outcomeMap, knownMetrics map[string]struct{}) AlertCoverageResult {
	result := AlertCoverageResult{
		AlertName:  r.Name,
		AlertGroup: r.Group,
		Expr:       r.Expr,
	}

	for _, name := range r.MetricNames {
		mr := AlertMetricResult{MetricName: name}

		if strings.HasPrefix(name, "~") {
			// Regex pattern — match against all known metrics.
			mr.FilterOutcome = resolveRegexOutcome(name[1:], outcomes)
		} else if outcome, ok := outcomes[name]; ok {
			mr.FilterOutcome = outcome
		} else if knownMetrics != nil {
			// Tap is active — check if we've actually seen this metric.
			if _, seen := knownMetrics[name]; seen {
				// Metric exists in catalog and no filter touches it → safe.
				mr.FilterOutcome = filter.OutcomeKept
			} else {
				// Metric not in catalog and not in any filter → unknown.
				mr.FilterOutcome = filter.OutcomeUnknown
			}
		} else {
			// No tap data available, no filter references it — assume safe.
			mr.FilterOutcome = filter.OutcomeKept
		}

		result.Metrics = append(result.Metrics, mr)
	}

	result.Status = deriveStatus(result.Metrics, r.UsesAbsent)
	return result
}

// resolveRegexOutcome matches a __name__ regex pattern against all known
// metric names and returns the worst outcome among matches.
func resolveRegexOutcome(pattern string, outcomes outcomeMap) filter.MatchOutcome {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return filter.OutcomeUnknown
	}

	matched := false
	result := filter.OutcomeKept
	for name, outcome := range outcomes {
		if re.MatchString(name) {
			matched = true
			result = composeOutcome(result, outcome)
		}
	}

	if !matched {
		return filter.OutcomeUnknown
	}
	return result
}

// deriveStatus determines the overall alert status from individual metric outcomes.
func deriveStatus(metrics []AlertMetricResult, usesAbsent bool) AlertStatus {
	if len(metrics) == 0 {
		return AlertUnknown
	}

	hasDropped := false
	hasPartial := false
	hasUnknown := false

	for _, m := range metrics {
		switch m.FilterOutcome {
		case filter.OutcomeDropped:
			hasDropped = true
		case filter.OutcomePartial:
			hasPartial = true
		case filter.OutcomeUnknown:
			hasUnknown = true
		}
	}

	if hasDropped {
		if usesAbsent {
			return AlertWouldActivate
		}
		return AlertBroken
	}
	if hasPartial {
		return AlertAtRisk
	}
	if hasUnknown {
		return AlertUnknown
	}
	return AlertSafe
}

func buildCoverageSummary(results []AlertCoverageResult) CoverageSummary {
	s := CoverageSummary{Total: len(results)}
	for _, r := range results {
		switch r.Status {
		case AlertSafe:
			s.Safe++
		case AlertAtRisk:
			s.AtRisk++
		case AlertBroken:
			s.Broken++
		case AlertWouldActivate:
			s.WouldActivate++
		case AlertUnknown:
			s.Unknown++
		}
	}
	return s
}
