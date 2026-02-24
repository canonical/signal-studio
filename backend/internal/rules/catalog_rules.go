package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/simskij/signal-studio/internal/config"
	"github.com/simskij/signal-studio/internal/filter"
	"github.com/simskij/signal-studio/internal/tap"
)

// Rule 1: Internal metrics not filtered

// InternalMetricsNotFiltered fires when otelcol_* metrics are present in the
// catalog but no filter processor drops them.
type InternalMetricsNotFiltered struct{}

func (r *InternalMetricsNotFiltered) ID() string {
	return "catalog-internal-metrics-not-filtered"
}

func (r *InternalMetricsNotFiltered) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *InternalMetricsNotFiltered) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	entries []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []Finding {
	var internal []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name, "otelcol_") {
			internal = append(internal, e.Name)
		}
	}
	if len(internal) == 0 {
		return nil
	}

	// Check if any filter analysis already drops all internal metrics
	for _, a := range analyses {
		allDropped := true
		for _, name := range internal {
			dropped := false
			for _, res := range a.Results {
				if res.MetricName == name && res.Outcome == filter.OutcomeDropped {
					dropped = true
					break
				}
			}
			if !dropped {
				allDropped = false
				break
			}
		}
		if allDropped {
			return nil
		}
	}

	return []Finding{{
		RuleID:   r.ID(),
		Title:    "Internal collector metrics are being exported",
		Severity: SeverityWarning,
		Evidence: fmt.Sprintf("Found %d otelcol_* metrics: %s",
			len(internal), strings.Join(internal, ", ")),
		Explanation:  "The collector's own internal metrics (otelcol_*) are present in the exported metric stream.",
		WhyItMatters: "Internal metrics add volume and cost without providing value to application observability. If you are looking to observe the stability of the collector itself, this warning may be ignored.",
		Impact:       fmt.Sprintf("Filtering %d internal metrics would reduce exported metric cardinality.", len(internal)),
		Snippet: `processors:
  filter/internal:
    error_mode: ignore
    metrics:
      metric:
        - 'IsMatch(name, "^otelcol_.*")'`,
		Placement: "Add a filter processor to drop otelcol_* metrics in the metrics pipeline.",
	}}
}

// Rule 2: High attribute count

// HighAttributeCount fires when a metric has more than 10 attribute keys.
type HighAttributeCount struct{}

func (r *HighAttributeCount) ID() string { return "catalog-high-attribute-count" }

func (r *HighAttributeCount) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *HighAttributeCount) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	entries []tap.MetricEntry,
	_ []filter.FilterAnalysis,
) []Finding {
	var findings []Finding
	for _, e := range entries {
		if len(e.AttributeKeys) > 10 {
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Title:    fmt.Sprintf("High attribute count on %s", e.Name),
				Severity: SeverityWarning,
				Evidence: fmt.Sprintf("Metric %q has %d attribute keys: %s",
					e.Name, len(e.AttributeKeys), strings.Join(e.AttributeKeys, ", ")),
				Explanation:  "This metric has an unusually high number of attribute keys, which drives cardinality.",
				WhyItMatters: "Each unique combination of attribute values creates a separate time series. High attribute counts multiply cardinality exponentially.",
				Impact:       fmt.Sprintf("Reducing attributes on %q could significantly lower series count.", e.Name),
				Snippet: `processors:
  transform:
    metric_statements:
      - context: datapoint
        statements:
          - delete_key(attributes, "unnecessary_key")`,
				Placement: "Use a transform processor to remove low-value attributes.",
			})
		}
	}
	return findings
}

// Rule 3: Point count outlier

// PointCountOutlier fires when a metric's point count is >10x the mean and >1000.
type PointCountOutlier struct{}

func (r *PointCountOutlier) ID() string { return "catalog-point-count-outlier" }

func (r *PointCountOutlier) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *PointCountOutlier) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	entries []tap.MetricEntry,
	_ []filter.FilterAnalysis,
) []Finding {
	if len(entries) == 0 {
		return nil
	}

	var total int64
	for _, e := range entries {
		total += e.PointCount
	}
	mean := float64(total) / float64(len(entries))

	var findings []Finding
	for _, e := range entries {
		if float64(e.PointCount) > 10*mean && e.PointCount > 1000 {
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Title:    fmt.Sprintf("Point count outlier: %s", e.Name),
				Severity: SeverityWarning,
				Evidence: fmt.Sprintf("Metric %q has %d points (mean: %.0f)",
					e.Name, e.PointCount, mean),
				Explanation:  "This metric produces significantly more data points than others, suggesting high cardinality or frequent emission.",
				WhyItMatters: "Outlier metrics dominate storage and query cost disproportionately.",
				Impact:       fmt.Sprintf("Metric %q accounts for a disproportionate share of total data points.", e.Name),
				Snippet: `processors:
  filter/high-volume:
    error_mode: ignore
    metrics:
      metric:
        - 'name == "<metric_name>"'`,
				Placement: "Consider filtering, aggregating, or reducing the emission frequency of this metric.",
			})
		}
	}
	return findings
}

// Rule 4: Filter keeps everything

// FilterKeepsEverything fires when a filter processor matches no metrics for dropping.
type FilterKeepsEverything struct{}

func (r *FilterKeepsEverything) ID() string { return "catalog-filter-keeps-everything" }

func (r *FilterKeepsEverything) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *FilterKeepsEverything) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	_ []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []Finding {
	var findings []Finding
	for _, a := range analyses {
		if a.DroppedCount == 0 && a.UnknownCount == 0 && a.KeptCount > 0 && !a.HasUnsupported {
			findings = append(findings, Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Filter %q keeps all metrics", a.ProcessorName),
				Severity:     SeverityInfo,
				Evidence:     fmt.Sprintf("Processor %q in pipeline %q: %d kept, 0 dropped", a.ProcessorName, a.Pipeline, a.KeptCount),
				Explanation:  "This filter processor does not match any observed metrics for dropping.",
				WhyItMatters: "A filter that keeps everything adds processing overhead without reducing volume.",
				Impact:       "Review filter rules to ensure they match the intended metrics, or remove the processor.",
				Snippet:      "",
				Placement:    "Review the filter expressions or remove this processor if it is no longer needed.",
				Pipeline:     a.Pipeline,
			})
		}
	}
	return findings
}

// Rule 5: Filter drops everything

// FilterDropsEverything fires when a filter processor drops all observed metrics.
type FilterDropsEverything struct{}

func (r *FilterDropsEverything) ID() string { return "catalog-filter-drops-everything" }

func (r *FilterDropsEverything) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *FilterDropsEverything) EvaluateWithCatalog(
	_ *config.CollectorConfig,
	_ []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []Finding {
	var findings []Finding
	for _, a := range analyses {
		if a.KeptCount == 0 && a.DroppedCount > 0 {
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Title:    fmt.Sprintf("Filter %q drops all metrics", a.ProcessorName),
				Severity: SeverityCritical,
				Evidence: fmt.Sprintf("Processor %q in pipeline %q: 0 kept, %d dropped",
					a.ProcessorName, a.Pipeline, a.DroppedCount),
				Explanation:  "This filter processor drops every observed metric, meaning no metric data exits this pipeline.",
				WhyItMatters: "A filter that drops everything effectively disables the metrics pipeline.",
				Impact:       fmt.Sprintf("All %d observed metrics are being dropped.", a.DroppedCount),
				Snippet:      "",
				Placement:    "Review the filter rules — this is likely a misconfiguration.",
				Pipeline:     a.Pipeline,
			})
		}
	}
	return findings
}

// Rule 6: No filter with high volume

// NoFilterHighVolume fires when there are many metrics but no filter processor.
type NoFilterHighVolume struct{}

func (r *NoFilterHighVolume) ID() string { return "catalog-no-filter-high-volume" }

func (r *NoFilterHighVolume) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *NoFilterHighVolume) EvaluateWithCatalog(
	cfg *config.CollectorConfig,
	entries []tap.MetricEntry,
	analyses []filter.FilterAnalysis,
) []Finding {
	if len(analyses) > 0 {
		return nil
	}

	// Check if any metrics pipeline has a filter processor
	for _, p := range cfg.Pipelines {
		if p.Signal != config.SignalMetrics {
			continue
		}
		if hasProcessorType(p.Processors, "filter") {
			return nil
		}
	}

	if len(entries) <= 50 {
		return nil
	}

	return []Finding{{
		RuleID:       r.ID(),
		Title:        "High metric volume without a filter processor",
		Severity:     SeverityInfo,
		Evidence:     fmt.Sprintf("%d unique metrics observed with no filter processor in any metrics pipeline.", len(entries)),
		Explanation:  "A large number of unique metrics are flowing through the collector without any filtering.",
		WhyItMatters: "Without filtering, all metrics are exported regardless of value, increasing cost and noise.",
		Impact:       "Adding a filter processor can reduce exported volume by dropping low-value metrics.",
		Snippet: `processors:
  filter/metrics:
    error_mode: ignore
    metrics:
      metric:
        - 'IsMatch(name, "^otelcol_.*")'`,
		Placement: "Add a filter processor to the metrics pipeline to control which metrics are exported.",
	}}
}

// Rule 7: Many histograms

// ManyHistograms fires when histogram metrics dominate the catalog.
type ManyHistograms struct{}

func (r *ManyHistograms) ID() string { return "catalog-many-histograms" }

func (r *ManyHistograms) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *ManyHistograms) EvaluateWithCatalog(
	cfg *config.CollectorConfig,
	entries []tap.MetricEntry,
	_ []filter.FilterAnalysis,
) []Finding {
	if len(entries) == 0 {
		return nil
	}

	// Skip if a cumulativetodelta or transform processor exists
	for _, p := range cfg.Pipelines {
		if p.Signal != config.SignalMetrics {
			continue
		}
		for _, proc := range p.Processors {
			pt := config.ComponentType(proc)
			if pt == "cumulativetodelta" || pt == "transform" {
				return nil
			}
		}
	}

	var histCount int
	for _, e := range entries {
		if e.Type == tap.MetricTypeHistogram || e.Type == tap.MetricTypeExponentialHistogram {
			histCount++
		}
	}

	pct := float64(histCount) / float64(len(entries)) * 100
	if histCount <= 5 || pct <= 30 {
		return nil
	}

	return []Finding{{
		RuleID:   r.ID(),
		Title:    "Histogram-heavy metric stream",
		Severity: SeverityInfo,
		Evidence: fmt.Sprintf("%d of %d metrics (%.0f%%) are histograms.",
			histCount, len(entries), pct),
		Explanation:  "A large proportion of metrics are histograms, which produce many more time series per metric than gauges or sums.",
		WhyItMatters: "Each histogram bucket is a separate series. A single histogram with 10 buckets generates 10+ series.",
		Impact:       "Consider converting cumulative histograms to delta temporality or using exponential histograms to reduce bucket count.",
		Snippet: `processors:
  cumulativetodelta:
    include:
      match_type: strict
      metrics: [...]`,
		Placement: "Add a cumulativetodelta or transform processor to reduce histogram cardinality.",
	}}
}

// Rule 8: Short scrape interval

// ShortScrapeInterval fires when a receiver uses a sub-minute scrape/collection interval.
type ShortScrapeInterval struct{}

func (r *ShortScrapeInterval) ID() string { return "catalog-short-scrape-interval" }

func (r *ShortScrapeInterval) Evaluate(_ *config.CollectorConfig) []Finding { return nil }

func (r *ShortScrapeInterval) EvaluateWithCatalog(
	cfg *config.CollectorConfig,
	_ []tap.MetricEntry,
	_ []filter.FilterAnalysis,
) []Finding {
	var findings []Finding

	for name, recv := range cfg.Receivers {
		recvType := config.ComponentType(name)

		switch recvType {
		case "prometheus":
			findings = append(findings, checkPrometheusScrapeIntervals(r.ID(), name, recv.Config)...)
		case "hostmetrics":
			findings = append(findings, checkHostMetricsInterval(r.ID(), name, recv.Config)...)
		}
	}

	return findings
}

func checkPrometheusScrapeIntervals(ruleID, name string, raw map[string]any) []Finding {
	if raw == nil {
		return nil
	}
	cfgRaw, ok := raw["config"]
	if !ok {
		return nil
	}
	cfgMap, ok := cfgRaw.(map[string]any)
	if !ok {
		return nil
	}
	scrapeConfigsRaw, ok := cfgMap["scrape_configs"]
	if !ok {
		return nil
	}
	scrapeConfigs, ok := scrapeConfigsRaw.([]any)
	if !ok {
		return nil
	}

	var findings []Finding
	for _, scRaw := range scrapeConfigs {
		sc, ok := scRaw.(map[string]any)
		if !ok {
			continue
		}
		intervalStr, ok := sc["scrape_interval"].(string)
		if !ok {
			continue
		}
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			continue
		}
		if d < 60*time.Second {
			jobName, _ := sc["job_name"].(string)
			evidence := fmt.Sprintf("Receiver %q", name)
			if jobName != "" {
				evidence = fmt.Sprintf("Receiver %q job %q", name, jobName)
			}
			findings = append(findings, Finding{
				RuleID:       ruleID,
				Title:        fmt.Sprintf("Sub-minute scrape interval on %s", name),
				Severity:     SeverityInfo,
				Evidence:     fmt.Sprintf("%s has scrape_interval: %s", evidence, intervalStr),
				Explanation:  "This receiver uses a scrape interval shorter than 60 seconds.",
				WhyItMatters: "Sub-minute intervals should be used sparingly; unless the service is critical and operates in bursts, it will not provide additional value.",
				Impact:       "Longer scrape intervals reduce metric volume and collector load.",
				Snippet: fmt.Sprintf(`receivers:
  %s:
    config:
      scrape_configs:
        - scrape_interval: 60s`, name),
				Placement: "Increase the scrape interval to 60s or longer unless sub-minute resolution is required.",
			})
		}
	}
	return findings
}

func checkHostMetricsInterval(ruleID, name string, raw map[string]any) []Finding {
	if raw == nil {
		return nil
	}
	intervalStr, ok := raw["collection_interval"].(string)
	if !ok {
		return nil
	}
	d, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil
	}
	if d >= 60*time.Second {
		return nil
	}

	return []Finding{{
		RuleID:       ruleID,
		Title:        fmt.Sprintf("Sub-minute collection interval on %s", name),
		Severity:     SeverityInfo,
		Evidence:     fmt.Sprintf("Receiver %q has collection_interval: %s", name, intervalStr),
		Explanation:  "This receiver uses a collection interval shorter than 60 seconds.",
		WhyItMatters: "Sub-minute intervals should be used sparingly; unless the service is critical and operates in bursts, it will not provide additional value.",
		Impact:       "Longer collection intervals reduce metric volume and collector load.",
		Snippet: fmt.Sprintf(`receivers:
  %s:
    collection_interval: 60s`, name),
		Placement: "Increase the collection interval to 60s or longer unless sub-minute resolution is required.",
	}}
}
