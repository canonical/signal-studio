package alertcoverage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ClientOptions configures the Prometheus/Mimir rules API client.
type ClientOptions struct {
	URL     string // e.g. "http://prometheus:9090"
	Token   string // optional bearer token
	OrgID   string // optional Mimir X-Scope-OrgID
	Timeout time.Duration
}

// prometheusRulesResponse is the response from GET /api/v1/rules.
type prometheusRulesResponse struct {
	Status string `json:"status"`
	Data   struct {
		Groups []prometheusAPIGroup `json:"groups"`
	} `json:"data"`
}

type prometheusAPIGroup struct {
	Name  string               `json:"name"`
	Rules []prometheusAPIRule  `json:"rules"`
}

type prometheusAPIRule struct {
	Type        string            `json:"type"` // "alerting" or "recording"
	Name        string            `json:"name"`
	Query       string            `json:"query"`
	Alert       string            `json:"alert,omitempty"`
	Record      string            `json:"record,omitempty"`
	Duration    float64           `json:"duration,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// FetchResult holds both parsed rules and reconstructed YAML from a remote API.
type FetchResult struct {
	Rules     []AlertRule
	RulesYAML string
}

// FetchRules fetches alert and recording rules from a Prometheus/Mimir API
// endpoint and returns them as parsed AlertRules along with reconstructed YAML.
func FetchRules(opts ClientOptions) (*FetchResult, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	url := strings.TrimRight(opts.URL, "/") + "/api/v1/rules"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if opts.Token != "" {
		req.Header.Set("Authorization", "Bearer "+opts.Token)
	}
	if opts.OrgID != "" {
		req.Header.Set("X-Scope-OrgID", opts.OrgID)
	}

	client := &http.Client{Timeout: opts.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching rules: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("rules API returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp prometheusRulesResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding rules response: %w", err)
	}

	if apiResp.Status != "success" {
		return nil, fmt.Errorf("rules API returned status %q", apiResp.Status)
	}

	rules, err := convertAPIRules(apiResp.Data.Groups)
	if err != nil {
		return nil, err
	}

	yamlStr := reconstructYAML(apiResp.Data.Groups)

	return &FetchResult{Rules: rules, RulesYAML: yamlStr}, nil
}

func convertAPIRules(groups []prometheusAPIGroup) ([]AlertRule, error) {
	var rules []AlertRule
	for _, g := range groups {
		for _, r := range g.Rules {
			expr := r.Query
			if expr == "" {
				continue
			}

			ar := AlertRule{
				Group: g.Name,
				Expr:  expr,
			}

			switch r.Type {
			case "alerting":
				ar.Name = r.Name
				ar.Type = "alert"
			case "recording":
				ar.Name = r.Name
				ar.Type = "record"
			default:
				continue
			}

			metrics, usesAbsent, err := ExtractMetrics(expr)
			if err != nil {
				// Graceful degradation: skip unparseable rules.
				continue
			}
			ar.MetricNames = metrics
			ar.UsesAbsent = usesAbsent

			rules = append(rules, ar)
		}
	}
	return rules, nil
}

// reconstructYAML builds a Prometheus-format rule file YAML from the API
// response, sorted stably by group name then alert name.
func reconstructYAML(groups []prometheusAPIGroup) string {
	// Sort groups by name.
	sorted := make([]prometheusAPIGroup, len(groups))
	copy(sorted, groups)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var b strings.Builder
	b.WriteString("groups:\n")

	for _, g := range sorted {
		// Sort rules within the group: alerts first (by name), then recording rules (by name).
		rules := make([]prometheusAPIRule, len(g.Rules))
		copy(rules, g.Rules)
		sort.Slice(rules, func(i, j int) bool {
			if rules[i].Type != rules[j].Type {
				return rules[i].Type == "alerting"
			}
			return rules[i].Name < rules[j].Name
		})

		b.WriteString("  - name: ")
		b.WriteString(g.Name)
		b.WriteString("\n    rules:\n")

		for _, r := range rules {
			if r.Query == "" {
				continue
			}
			if r.Type == "alerting" {
				b.WriteString("      - alert: ")
				b.WriteString(r.Name)
				b.WriteString("\n")
			} else if r.Type == "recording" {
				b.WriteString("      - record: ")
				b.WriteString(r.Name)
				b.WriteString("\n")
			} else {
				continue
			}

			b.WriteString("        expr: ")
			if strings.ContainsAny(r.Query, "\n{}[]") {
				b.WriteString("|\n")
				for _, line := range strings.Split(r.Query, "\n") {
					b.WriteString("          ")
					b.WriteString(line)
					b.WriteString("\n")
				}
			} else {
				b.WriteString(r.Query)
				b.WriteString("\n")
			}

			if r.Duration > 0 {
				secs := int(r.Duration)
				if secs >= 60 && secs%60 == 0 {
					fmt.Fprintf(&b, "        for: %dm\n", secs/60)
				} else {
					fmt.Fprintf(&b, "        for: %ds\n", secs)
				}
			}

			if len(r.Labels) > 0 {
				b.WriteString("        labels:\n")
				keys := sortedKeys(r.Labels)
				for _, k := range keys {
					fmt.Fprintf(&b, "          %s: %s\n", k, r.Labels[k])
				}
			}

			if len(r.Annotations) > 0 {
				b.WriteString("        annotations:\n")
				keys := sortedKeys(r.Annotations)
				for _, k := range keys {
					fmt.Fprintf(&b, "          %s: %s\n", k, r.Annotations[k])
				}
			}
		}
	}

	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// MergeRules merges multiple sets of rules, deduplicating by name+group.
func MergeRules(sets ...[]AlertRule) []AlertRule {
	seen := make(map[string]struct{})
	var merged []AlertRule
	for _, set := range sets {
		for _, r := range set {
			key := r.Group + "/" + r.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, r)
		}
	}
	return merged
}
