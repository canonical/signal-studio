package static

import (
	"fmt"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// ConnectorLoop detects cycles in the pipeline graph formed by connectors.
// A loop causes infinite data circulation and eventual resource exhaustion.
type ConnectorLoop struct{}

func (r *ConnectorLoop) ID() string { return "connector-loop" }

func (r *ConnectorLoop) Description() string {
	return "Connectors form a pipeline cycle causing infinite data circulation"
}

func (r *ConnectorLoop) DefaultSeverity() rules.Severity { return rules.SeverityCritical }

func (r *ConnectorLoop) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	if len(cfg.Connectors) == 0 {
		return nil
	}

	// Build adjacency: for each pipeline, find pipelines it feeds via connectors.
	adj := map[string][]string{}
	// Map connector name → pipelines that receive from it.
	connReceivers := map[string][]string{}
	for pName, p := range cfg.Pipelines {
		for _, recv := range p.Receivers {
			if _, ok := cfg.Connectors[recv]; ok {
				connReceivers[recv] = append(connReceivers[recv], pName)
			}
		}
		_ = pName // ensure pName used
	}
	for pName, p := range cfg.Pipelines {
		for _, exp := range p.Exporters {
			if _, ok := cfg.Connectors[exp]; ok {
				adj[pName] = append(adj[pName], connReceivers[exp]...)
			}
		}
	}

	// DFS cycle detection.
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // finished
	)
	color := map[string]int{}
	parent := map[string]string{}
	var cycles [][]string

	var dfs func(u string)
	dfs = func(u string) {
		color[u] = gray
		for _, v := range adj[u] {
			if color[v] == gray {
				// Found a cycle — reconstruct the path.
				cycle := []string{v}
				for cur := u; cur != v; cur = parent[cur] {
					cycle = append(cycle, cur)
				}
				cycle = append(cycle, v)
				// Reverse to get forward order.
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				cycles = append(cycles, cycle)
			} else if color[v] == white {
				parent[v] = u
				dfs(v)
			}
		}
		color[u] = black
	}

	for pName := range cfg.Pipelines {
		if color[pName] == white {
			dfs(pName)
		}
	}

	if len(cycles) == 0 {
		return nil
	}

	var findings []rules.Finding
	for _, cycle := range cycles {
		path := strings.Join(cycle, " → ")
		findings = append(findings, rules.Finding{
			RuleID:     r.ID(),
			Title:      fmt.Sprintf("Pipeline loop detected: %s", path),
			Severity:   rules.SeverityCritical,
			Confidence: rules.ConfidenceHigh,
			Evidence:   fmt.Sprintf("Pipelines form a cycle via connectors: %s", path),
			Implication: "A pipeline loop causes infinite data amplification, leading to unbounded memory growth, CPU saturation, and eventual collector crash. Break the loop by removing or reconfiguring the connector that closes the cycle." +
				"However, verify the cycle path carefully; complex connector topologies can be hard to visualize.",

			Scope:          "all pipelines",
			Snippet:        "# Review the connector routing to ensure no circular dependencies exist.",
			Recommendation: "Restructure the pipeline topology to eliminate the cycle.",
		})
	}
	return findings
}
