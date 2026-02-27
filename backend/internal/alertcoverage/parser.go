package alertcoverage

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// prometheusRuleFile is the standard Prometheus/Thanos rule file structure.
type prometheusRuleFile struct {
	Groups []ruleGroup `yaml:"groups"`
}

type ruleGroup struct {
	Name  string `yaml:"name"`
	Rules []rule `yaml:"rules"`
}

type rule struct {
	Alert  string `yaml:"alert"`
	Record string `yaml:"record"`
	Expr   string `yaml:"expr"`
}

// k8sCRD is a Kubernetes PrometheusRule CRD wrapper.
type k8sCRD struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		Groups []ruleGroup `yaml:"groups"`
	} `yaml:"spec"`
}

// ParseRules parses a YAML string containing alert/recording rules.
// It auto-detects the format: standard Prometheus rule file or Kubernetes
// PrometheusRule CRD.
func ParseRules(data []byte) ([]AlertRule, error) {
	groups, err := parseGroups(data)
	if err != nil {
		return nil, err
	}
	return extractRules(groups)
}

func parseGroups(data []byte) ([]ruleGroup, error) {
	if isCRD(data) {
		return parseCRD(data)
	}
	return parseStandard(data)
}

func isCRD(data []byte) bool {
	// Quick check: look for apiVersion and kind: PrometheusRule
	s := string(data)
	return strings.Contains(s, "apiVersion") && strings.Contains(s, "PrometheusRule")
}

func parseCRD(data []byte) ([]ruleGroup, error) {
	var crd k8sCRD
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return nil, fmt.Errorf("parsing CRD: %w", err)
	}
	return crd.Spec.Groups, nil
}

func parseStandard(data []byte) ([]ruleGroup, error) {
	var rf prometheusRuleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing rules file: %w", err)
	}
	return rf.Groups, nil
}

func extractRules(groups []ruleGroup) ([]AlertRule, error) {
	var rules []AlertRule
	var parseErrors []string

	for _, g := range groups {
		for _, r := range g.Rules {
			if r.Expr == "" {
				continue
			}

			ar := AlertRule{
				Group: g.Name,
				Expr:  r.Expr,
			}

			if r.Alert != "" {
				ar.Name = r.Alert
				ar.Type = "alert"
			} else if r.Record != "" {
				ar.Name = r.Record
				ar.Type = "record"
			} else {
				continue
			}

			metrics, usesAbsent, err := ExtractMetrics(r.Expr)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("rule %q: %v", ar.Name, err))
				continue
			}
			ar.MetricNames = metrics
			ar.UsesAbsent = usesAbsent

			rules = append(rules, ar)
		}
	}

	if len(rules) == 0 && len(parseErrors) > 0 {
		return nil, fmt.Errorf("all rules failed to parse: %s", strings.Join(parseErrors, "; "))
	}

	return rules, nil
}
