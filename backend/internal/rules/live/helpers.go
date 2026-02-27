package live

import (
	"math"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
)

// sumRateAcrossLabels computes the total rate for a metric, summing across all label combinations.
func sumRateAcrossLabels(prev, curr *metrics.Snapshot, metricName string) float64 {
	// Find all unique label sets for this metric in the current snapshot
	seen := make(map[string]bool)
	total := 0.0

	for _, s := range curr.Samples {
		if s.Name != metricName {
			continue
		}
		k := labelKey(s.Labels)
		if seen[k] {
			continue
		}
		seen[k] = true
		total += metrics.RatePerSecond(prev, curr, metricName, s.Labels)
	}
	return math.Max(total, 0)
}

// tracesPipelineHasSampler returns true if any traces pipeline includes a
// probabilistic_sampler or tail_sampling processor.
func tracesPipelineHasSampler(cfg *config.CollectorConfig) bool {
	if cfg == nil {
		return false
	}
	for _, p := range cfg.Pipelines {
		if p.Signal != config.SignalTraces {
			continue
		}
		for _, proc := range p.Processors {
			pt := config.ComponentType(proc)
			if pt == "probabilistic_sampler" || pt == "tail_sampling" {
				return true
			}
		}
	}
	return false
}

func labelKey(labels map[string]string) string {
	// Simple key for deduplication — order doesn't matter for small label sets
	var b strings.Builder
	for key, val := range labels {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(val)
		b.WriteByte(',')
	}
	return b.String()
}
