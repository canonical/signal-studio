package filter

import "regexp"

// MatchOutcome describes whether a metric is kept, dropped, or unknown.
type MatchOutcome string

const (
	OutcomeKept    MatchOutcome = "kept"
	OutcomeDropped MatchOutcome = "dropped"
	OutcomeUnknown MatchOutcome = "unknown"
)

// MatchResult describes the filter outcome for a single metric name.
type MatchResult struct {
	MetricName string       `json:"metricName"`
	Outcome    MatchOutcome `json:"outcome"`
	MatchedBy  string       `json:"matchedBy,omitempty"`
}

// FilterAnalysis is the full result of analyzing a filter against metric names.
type FilterAnalysis struct {
	ProcessorName  string        `json:"processorName"`
	Pipeline       string        `json:"pipeline"`
	Style          string        `json:"style"`
	Results        []MatchResult `json:"results"`
	KeptCount      int           `json:"keptCount"`
	DroppedCount   int           `json:"droppedCount"`
	UnknownCount   int           `json:"unknownCount"`
	HasUnsupported bool          `json:"hasUnsupported"`
}

// AnalyzeFilter tests each metric name against the filter config rules and
// returns a FilterAnalysis.
func AnalyzeFilter(fc FilterConfig, metricNames []string) FilterAnalysis {
	fa := FilterAnalysis{
		ProcessorName: fc.ProcessorName,
		Pipeline:      fc.Pipeline,
		Style:         fc.Style,
	}

	hasUnsupported := false
	for _, r := range fc.Rules {
		if r.MatchType == MatchTypeUnsupported {
			hasUnsupported = true
			break
		}
	}
	fa.HasUnsupported = hasUnsupported

	for _, name := range metricNames {
		mr := matchMetric(fc, name)
		fa.Results = append(fa.Results, mr)
		switch mr.Outcome {
		case OutcomeKept:
			fa.KeptCount++
		case OutcomeDropped:
			fa.DroppedCount++
		case OutcomeUnknown:
			fa.UnknownCount++
		}
	}

	return fa
}

// matchMetric determines the outcome for a single metric name.
func matchMetric(fc FilterConfig, name string) MatchResult {
	if fc.Style == "ottl" {
		return matchOTTL(fc.Rules, name)
	}
	return matchLegacy(fc.Rules, name)
}

// matchOTTL applies OTTL filter rules. In the filter processor, OTTL expressions
// that match cause the metric to be dropped.
func matchOTTL(rules []FilterRule, name string) MatchResult {
	for _, r := range rules {
		if r.MatchType == MatchTypeUnsupported {
			return MatchResult{MetricName: name, Outcome: OutcomeUnknown, MatchedBy: r.Raw}
		}
		if matchesPattern(name, r.Pattern, r.MatchType) {
			return MatchResult{MetricName: name, Outcome: OutcomeDropped, MatchedBy: r.Raw}
		}
	}
	return MatchResult{MetricName: name, Outcome: OutcomeKept}
}

// matchLegacy applies legacy include/exclude rules.
// Logic: include first (if any), then exclude applied to survivors.
func matchLegacy(rules []FilterRule, name string) MatchResult {
	var includeRules, excludeRules []FilterRule
	for _, r := range rules {
		switch r.Action {
		case ActionInclude:
			includeRules = append(includeRules, r)
		case ActionExclude:
			excludeRules = append(excludeRules, r)
		}
	}

	// Phase 1: include filter — if include rules exist, metric must match at least one
	if len(includeRules) > 0 {
		matched := false
		var matchedBy string
		for _, r := range includeRules {
			if matchesPattern(name, r.Pattern, r.MatchType) {
				matched = true
				matchedBy = r.Pattern
				break
			}
		}
		if !matched {
			return MatchResult{MetricName: name, Outcome: OutcomeDropped, MatchedBy: "not in include list"}
		}
		_ = matchedBy // included — continue to exclude check
	}

	// Phase 2: exclude filter — if metric matches an exclude rule, it's dropped
	for _, r := range excludeRules {
		if matchesPattern(name, r.Pattern, r.MatchType) {
			return MatchResult{MetricName: name, Outcome: OutcomeDropped, MatchedBy: r.Pattern}
		}
	}

	return MatchResult{MetricName: name, Outcome: OutcomeKept}
}

// matchesPattern checks if name matches pattern according to the match type.
func matchesPattern(name, pattern string, mt MatchType) bool {
	switch mt {
	case MatchTypeStrict, MatchTypeOTTLNameEq:
		return name == pattern
	case MatchTypeRegexp, MatchTypeOTTLIsMatch:
		matched, err := regexp.MatchString("^(?:"+pattern+")$", name)
		return err == nil && matched
	default:
		return false
	}
}
