package filter

import (
	"github.com/canonical/signal-studio/internal/config"
)

// Action represents what happens to a matching metric.
type Action string

const (
	ActionInclude Action = "include"
	ActionExclude Action = "exclude"
	ActionDrop    Action = "drop"
)

// MatchType describes how a pattern is matched.
type MatchType string

const (
	MatchTypeRegexp       MatchType = "regexp"
	MatchTypeStrict       MatchType = "strict"
	MatchTypeOTTLNameEq   MatchType = "ottl_name_eq"
	MatchTypeOTTLIsMatch  MatchType = "ottl_ismatch"
	MatchTypeUnsupported  MatchType = "unsupported"
)

// FilterRule represents a single filter condition.
type FilterRule struct {
	Raw       string    `json:"raw"`
	Action    Action    `json:"action"`
	MatchType MatchType `json:"matchType"`
	Pattern   string    `json:"pattern"`
}

// FilterConfig describes a filter processor extracted from a CollectorConfig.
type FilterConfig struct {
	ProcessorName string       `json:"processorName"`
	Pipeline      string       `json:"pipeline"`
	Style         string       `json:"style"` // "legacy" or "ottl"
	Rules         []FilterRule `json:"rules"`
}

// ExtractFilterConfigs finds all filter processors in the config and extracts their rules.
func ExtractFilterConfigs(cfg *config.CollectorConfig) []FilterConfig {
	var results []FilterConfig

	for name, comp := range cfg.Processors {
		if config.ComponentType(name) != "filter" {
			continue
		}

		pipelines := pipelinesForProcessor(cfg, name)

		fc := extractSingleFilter(name, comp.Config)
		for _, p := range pipelines {
			clone := fc
			clone.Pipeline = p
			results = append(results, clone)
		}
		if len(pipelines) == 0 {
			results = append(results, fc)
		}
	}

	return results
}

// pipelinesForProcessor returns the pipeline names that reference a processor.
func pipelinesForProcessor(cfg *config.CollectorConfig, procName string) []string {
	var pipes []string
	for pName, p := range cfg.Pipelines {
		for _, proc := range p.Processors {
			if proc == procName {
				pipes = append(pipes, pName)
				break
			}
		}
	}
	return pipes
}

// extractSingleFilter parses the filter config from a processor's raw config map.
func extractSingleFilter(name string, raw map[string]any) FilterConfig {
	fc := FilterConfig{
		ProcessorName: name,
	}

	if raw == nil {
		return fc
	}

	metricsRaw, ok := raw["metrics"]
	if !ok {
		return fc
	}
	metricsMap, ok := metricsRaw.(map[string]any)
	if !ok {
		return fc
	}

	// Check for OTTL style: metrics.metric[] (list of string expressions)
	if metricRaw, ok := metricsMap["metric"]; ok {
		if exprs, ok := metricRaw.([]any); ok && len(exprs) > 0 {
			fc.Style = "ottl"
			for _, e := range exprs {
				if s, ok := e.(string); ok {
					fc.Rules = append(fc.Rules, parseOTTLNameExpression(s))
				}
			}
			return fc
		}
	}

	// Legacy style: metrics.include / metrics.exclude
	fc.Style = "legacy"

	if includeRaw, ok := metricsMap["include"]; ok {
		if includeMap, ok := includeRaw.(map[string]any); ok {
			rules := parseLegacyBlock(includeMap, ActionInclude)
			fc.Rules = append(fc.Rules, rules...)
		}
	}

	if excludeRaw, ok := metricsMap["exclude"]; ok {
		if excludeMap, ok := excludeRaw.(map[string]any); ok {
			rules := parseLegacyBlock(excludeMap, ActionExclude)
			fc.Rules = append(fc.Rules, rules...)
		}
	}

	return fc
}

// parseLegacyBlock parses an include/exclude block with match_type and metric_names.
func parseLegacyBlock(block map[string]any, action Action) []FilterRule {
	matchType := MatchTypeStrict
	if mt, ok := block["match_type"].(string); ok {
		switch mt {
		case "regexp":
			matchType = MatchTypeRegexp
		case "strict":
			matchType = MatchTypeStrict
		default:
			matchType = MatchTypeStrict
		}
	}

	namesRaw, ok := block["metric_names"]
	if !ok {
		return nil
	}
	names, ok := namesRaw.([]any)
	if !ok {
		return nil
	}

	var rules []FilterRule
	for _, n := range names {
		if s, ok := n.(string); ok {
			rules = append(rules, FilterRule{
				Raw:       s,
				Action:    action,
				MatchType: matchType,
				Pattern:   s,
			})
		}
	}
	return rules
}
