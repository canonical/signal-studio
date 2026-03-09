package catalog

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/tap"
)

// Rule 9: Loop device metrics

// LoopDeviceMetrics fires when system.disk.* metrics contain loop devices,
// which are typically snap mounts or ISO mounts and add cardinality without value.
type LoopDeviceMetrics struct{}

func (r *LoopDeviceMetrics) ID() string { return "catalog-loop-device-metrics" }

func (r *LoopDeviceMetrics) Description() string {
	return "Loop device entries present in system.disk metrics"
}

func (r *LoopDeviceMetrics) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *LoopDeviceMetrics) Evaluate(_ *config.CollectorConfig) []rules.Finding { return nil }

var loopDevicePattern = regexp.MustCompile(`^loop\d+$`)

func (r *LoopDeviceMetrics) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	entries []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []rules.Finding {
	var affected []string
	var loopValues []string
	loopSeen := make(map[string]struct{})

	for _, e := range entries {
		if !strings.HasPrefix(e.Name, "system.disk.") {
			continue
		}
		for _, attr := range e.Attributes {
			if attr.Key != "device" || attr.Level != tap.AttributeLevelDatapoint {
				continue
			}
			hasLoop := false
			for _, v := range attr.SampleValues {
				if loopDevicePattern.MatchString(v) {
					hasLoop = true
					if _, ok := loopSeen[v]; !ok {
						loopSeen[v] = struct{}{}
						loopValues = append(loopValues, v)
					}
				}
			}
			if hasLoop {
				affected = append(affected, e.Name)
			}
		}
	}

	if len(affected) == 0 {
		return nil
	}

	// Check if any filter analysis already addresses loop devices on the affected metrics
	if loopDevicesAlreadyFiltered(affected, analyses) {
		return nil
	}

	return []rules.Finding{{
		RuleID:     r.ID(),
		Title:      "Loop devices in system.disk metrics",
		Severity:   rules.SeverityInfo,
		Confidence: rules.ConfidenceHigh,
		Evidence: fmt.Sprintf("%d system.disk.* metrics contain loop devices (%s): %s",
			len(affected), strings.Join(loopValues, ", "), strings.Join(affected, ", ")),
		Implication: fmt.Sprintf("Each loop device adds data points to every system.disk metric without providing actionable disk monitoring data. Filtering %d loop device(s) would reduce data points across %d metrics.", len(loopValues), len(affected)) + "\nHowever, loop devices may be relevant in environments that use loop mounts for application storage.",
		Scope:        "receiver:hostmetrics",
		Snippet: `processors:
  filter/loop-devices:
    error_mode: ignore
    metrics:
      datapoint:
        - 'IsMatch(attributes["device"], "^loop\\d+$")'`,
		Recommendation: "Add a filter processor to drop data points from loop devices in the metrics pipeline.",
	}}
}

// loopDevicesAlreadyFiltered checks if any filter analysis already drops or
// partially drops data points for the affected system.disk.* metrics.
func loopDevicesAlreadyFiltered(affected []string, analyses []filter.FilterAnalysis) bool {
	if len(analyses) == 0 {
		return false
	}
	for _, a := range analyses {
		allAddressed := true
		for _, name := range affected {
			addressed := false
			for _, res := range a.Results {
				if res.MetricName == name && (res.Outcome == filter.OutcomeDropped || res.Outcome == filter.OutcomePartial) {
					addressed = true
					break
				}
			}
			if !addressed {
				allAddressed = false
				break
			}
		}
		if allAddressed {
			return true
		}
	}
	return false
}
