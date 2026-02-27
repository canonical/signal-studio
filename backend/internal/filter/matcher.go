package filter

import "regexp"

// MatchOutcome describes whether a metric is kept, dropped, or unknown.
type MatchOutcome string

const (
	OutcomeKept    MatchOutcome = "kept"
	OutcomeDropped MatchOutcome = "dropped"
	OutcomeUnknown MatchOutcome = "unknown"
	OutcomePartial MatchOutcome = "partial"
)

// MatchResult describes the filter outcome for a single metric name.
type MatchResult struct {
	MetricName   string       `json:"metricName"`
	Outcome      MatchOutcome `json:"outcome"`
	MatchedBy    string       `json:"matchedBy,omitempty"`
	DroppedRatio float64      `json:"droppedRatio,omitempty"`
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
	PartialCount   int           `json:"partialCount"`
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

// AttrMeta is a lightweight attribute descriptor for filter matching.
// It avoids a dependency from filter → tap.
type AttrMeta struct {
	Key          string   `json:"key"`
	Level        string   `json:"level"` // "resource", "scope", "datapoint"
	SampleValues []string `json:"sampleValues"`
	Capped       bool     `json:"capped"`
}

// MetricAttributeInfo describes a metric and its observed attributes.
type MetricAttributeInfo struct {
	Name       string     `json:"name"`
	Attributes []AttrMeta `json:"attributes"`
}

// AnalyzeFilterWithAttributes tests each metric against the filter rules,
// using attribute metadata for attribute-based rules.
func AnalyzeFilterWithAttributes(fc FilterConfig, metricInfos []MetricAttributeInfo) FilterAnalysis {
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

	for _, info := range metricInfos {
		mr := matchMetricWithAttrs(fc, info)
		fa.Results = append(fa.Results, mr)
		switch mr.Outcome {
		case OutcomeKept:
			fa.KeptCount++
		case OutcomeDropped:
			fa.DroppedCount++
		case OutcomeUnknown:
			fa.UnknownCount++
		case OutcomePartial:
			fa.PartialCount++
		}
	}

	return fa
}

// matchMetricWithAttrs determines the outcome for a single metric using attribute info.
func matchMetricWithAttrs(fc FilterConfig, info MetricAttributeInfo) MatchResult {
	if fc.Style == "ottl" {
		return matchOTTLWithAttrs(fc.Rules, info)
	}
	// Legacy style doesn't use attributes — delegate to name-only matching.
	return matchLegacy(fc.Rules, info.Name)
}

// matchOTTLWithAttrs applies OTTL filter rules using attribute metadata.
func matchOTTLWithAttrs(rules []FilterRule, info MetricAttributeInfo) MatchResult {
	for _, r := range rules {
		switch r.MatchType {
		case MatchTypeUnsupported:
			return MatchResult{MetricName: info.Name, Outcome: OutcomeUnknown, MatchedBy: r.Raw}

		case MatchTypeOTTLNameEq, MatchTypeOTTLIsMatch:
			if matchesPattern(info.Name, r.Pattern, r.MatchType) {
				return MatchResult{MetricName: info.Name, Outcome: OutcomeDropped, MatchedBy: r.Raw}
			}

		case MatchTypeOTTLResourceAttr:
			ar := matchAttrEquality(info.Attributes, "resource", r.AttrKey, r.AttrValue)
			if mr, ok := matchResultFromAttrResult(info.Name, ar, r.Raw, true); ok {
				return mr
			}

		case MatchTypeOTTLDatapointAttr:
			ar := matchAttrEquality(info.Attributes, "datapoint", r.AttrKey, r.AttrValue)
			if mr, ok := matchResultFromAttrResult(info.Name, ar, r.Raw, false); ok {
				return mr
			}

		case MatchTypeOTTLResourceAttrMatch:
			ar := matchAttrRegex(info.Attributes, "resource", r.AttrKey, r.Pattern)
			if mr, ok := matchResultFromAttrResult(info.Name, ar, r.Raw, true); ok {
				return mr
			}

		case MatchTypeOTTLDatapointAttrMatch:
			ar := matchAttrRegex(info.Attributes, "datapoint", r.AttrKey, r.Pattern)
			if mr, ok := matchResultFromAttrResult(info.Name, ar, r.Raw, false); ok {
				return mr
			}

		case MatchTypeOTTLHasAttrKey:
			outcome := matchHasAttrKey(info.Attributes, "datapoint", r.AttrKey)
			if outcome != OutcomeKept {
				return MatchResult{MetricName: info.Name, Outcome: outcome, MatchedBy: r.Raw}
			}

		case MatchTypeOTTLHasAttr:
			ar := matchAttrEquality(info.Attributes, "datapoint", r.AttrKey, r.AttrValue)
			if mr, ok := matchResultFromAttrResult(info.Name, ar, r.Raw, false); ok {
				return mr
			}
		}
	}
	return MatchResult{MetricName: info.Name, Outcome: OutcomeKept}
}

// attrMatchResult holds the detailed result of an attribute match including counts.
type attrMatchResult struct {
	Outcome      MatchOutcome
	MatchedCount int
	TotalCount   int
}

// matchResultFromAttrResult converts an attrMatchResult to a MatchResult.
// forceNonPartial is true for resource-level matchers where partial→dropped.
// Returns (result, true) if the outcome is not kept; (zero, false) if kept.
func matchResultFromAttrResult(name string, ar attrMatchResult, raw string, forceNonPartial bool) (MatchResult, bool) {
	if ar.Outcome == OutcomeKept {
		return MatchResult{}, false
	}
	if ar.Outcome == OutcomeUnknown {
		return MatchResult{MetricName: name, Outcome: OutcomeUnknown, MatchedBy: raw}, true
	}
	// ar.Outcome is either dropped or partial
	if forceNonPartial && ar.Outcome == OutcomePartial {
		// Resource-level: any match means the whole metric is dropped.
		return MatchResult{MetricName: name, Outcome: OutcomeDropped, MatchedBy: raw}, true
	}
	mr := MatchResult{MetricName: name, Outcome: ar.Outcome, MatchedBy: raw}
	if ar.Outcome == OutcomePartial && ar.TotalCount > 0 {
		mr.DroppedRatio = float64(ar.MatchedCount) / float64(ar.TotalCount)
	}
	return mr, true
}

// matchAttrEquality checks if a specific attribute key at a given level has the specified value.
// Returns an attrMatchResult with match counts for partial outcome support.
func matchAttrEquality(attrs []AttrMeta, level, key, value string) attrMatchResult {
	for _, a := range attrs {
		if a.Key == key && a.Level == level {
			matched := 0
			for _, sv := range a.SampleValues {
				if sv == value {
					matched++
				}
			}
			total := len(a.SampleValues)
			if matched == 0 {
				if a.Capped {
					return attrMatchResult{Outcome: OutcomeUnknown}
				}
				return attrMatchResult{Outcome: OutcomeKept}
			}
			if matched == total && !a.Capped {
				return attrMatchResult{Outcome: OutcomeDropped, MatchedCount: matched, TotalCount: total}
			}
			// Some matched (or all matched but capped — there may be unmatched values we didn't see)
			return attrMatchResult{Outcome: OutcomePartial, MatchedCount: matched, TotalCount: total}
		}
	}
	// Attribute key not found at all — can't match
	return attrMatchResult{Outcome: OutcomeKept}
}

// matchAttrRegex checks if any sample value of the attribute matches the regex pattern.
// Returns an attrMatchResult with match counts for partial outcome support.
func matchAttrRegex(attrs []AttrMeta, level, key, pattern string) attrMatchResult {
	for _, a := range attrs {
		if a.Key == key && a.Level == level {
			re, err := regexp.Compile("^(?:" + pattern + ")$")
			if err != nil {
				return attrMatchResult{Outcome: OutcomeUnknown}
			}
			matched := 0
			for _, sv := range a.SampleValues {
				if re.MatchString(sv) {
					matched++
				}
			}
			total := len(a.SampleValues)
			if matched == 0 {
				if a.Capped {
					return attrMatchResult{Outcome: OutcomeUnknown}
				}
				return attrMatchResult{Outcome: OutcomeKept}
			}
			if matched == total && !a.Capped {
				return attrMatchResult{Outcome: OutcomeDropped, MatchedCount: matched, TotalCount: total}
			}
			return attrMatchResult{Outcome: OutcomePartial, MatchedCount: matched, TotalCount: total}
		}
	}
	return attrMatchResult{Outcome: OutcomeKept}
}

// matchHasAttrKey checks if a specific attribute key exists at the given level.
func matchHasAttrKey(attrs []AttrMeta, level, key string) MatchOutcome {
	for _, a := range attrs {
		if a.Key == key && a.Level == level {
			return OutcomeDropped
		}
	}
	return OutcomeKept
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
