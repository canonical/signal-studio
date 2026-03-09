package static

import (
	"fmt"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// NoHealthCheckTraceFilter checks that traces pipelines have a filter processor
// that drops health check spans (e.g. /healthz, /readyz, /livez). Without such
// a filter, liveness and readiness probes generate high-volume, low-value traces.
type NoHealthCheckTraceFilter struct{}

func (r *NoHealthCheckTraceFilter) ID() string { return "no-health-check-trace-filter" }

func (r *NoHealthCheckTraceFilter) Description() string {
	return "Traces pipeline has no filter dropping health check spans"
}

func (r *NoHealthCheckTraceFilter) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *NoHealthCheckTraceFilter) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	healthPaths := []string{"/healthz", "/readyz", "/livez", "/health", "/ready"}

	var findings []rules.Finding
	for pName, p := range cfg.Pipelines {
		if p.Signal != config.SignalTraces {
			continue
		}

		hasHealthFilter := false
		for _, proc := range p.Processors {
			procType := config.ComponentType(proc)
			if procType != "filter" {
				continue
			}
			comp, ok := cfg.Processors[proc]
			if !ok || comp.Config == nil {
				continue
			}
			if filterDropsHealthSpans(comp.Config, healthPaths) {
				hasHealthFilter = true
				break
			}
		}

		if !hasHealthFilter {
			findings = append(findings, rules.Finding{
				RuleID:   r.ID(),
				Title:    fmt.Sprintf("No health check filter in %s pipeline", pName),
				Severity:   rules.SeverityInfo,
				Confidence: rules.ConfidenceHigh,
				Evidence:    fmt.Sprintf("Traces pipeline %q has no filter processor dropping health check spans (/healthz, /readyz).", pName),
				Implication:    "Kubernetes liveness and readiness probes generate a constant stream of spans for endpoints like /healthz and /readyz. Health check traces add storage cost and clutter trace search results without providing diagnostic value. Filtering health check spans typically reduces trace volume by 10-30%% in Kubernetes environments.\nHowever, if the application does not receive health check requests (e.g. non-HTTP services), this filter is unnecessary.",

				Scope:   fmt.Sprintf("pipeline:%s", pName),
				Snippet: `processors:
  filter/health:
    error_mode: ignore
    traces:
      span:
        - 'attributes["url.path"] == "/healthz"'
        - 'attributes["url.path"] == "/readyz"'
        - 'attributes["url.path"] == "/livez"'`,
				Recommendation: "Add as the first processor after memory_limiter in the traces pipeline.",
			})
		}
	}
	return findings
}
