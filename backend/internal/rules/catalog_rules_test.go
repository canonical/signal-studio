package rules

import (
	"fmt"
	"testing"
	"time"

	"github.com/simskij/signal-studio/internal/config"
	"github.com/simskij/signal-studio/internal/filter"
	"github.com/simskij/signal-studio/internal/tap"
)

func entry(name string, typ tap.MetricType, attrKeys []string, pointCount int64) tap.MetricEntry {
	return tap.MetricEntry{
		Name:          name,
		Type:          typ,
		AttributeKeys: attrKeys,
		PointCount:    pointCount,
		FirstSeenAt:   time.Now(),
		LastSeenAt:    time.Now(),
	}
}

func emptyCfg() *config.CollectorConfig {
	return &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}
}

// --- Rule 1: InternalMetricsNotFiltered ---

func TestInternalMetricsNotFiltered_Fires(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("otelcol_receiver_accepted_spans", tap.MetricTypeSum, nil, 100),
		entry("otelcol_exporter_sent_spans", tap.MetricTypeSum, nil, 100),
		entry("http_server_duration", tap.MetricTypeHistogram, nil, 50),
	}

	rule := &InternalMetricsNotFiltered{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "catalog-internal-metrics-not-filtered" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("severity = %q, want warning", findings[0].Severity)
	}
}

func TestInternalMetricsNotFiltered_NoInternalMetrics(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("http_server_duration", tap.MetricTypeHistogram, nil, 50),
	}

	rule := &InternalMetricsNotFiltered{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestInternalMetricsNotFiltered_AlreadyDropped(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("otelcol_receiver_accepted_spans", tap.MetricTypeSum, nil, 100),
	}
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName: "filter/internal",
			Pipeline:      "metrics",
			Results: []filter.MatchResult{
				{MetricName: "otelcol_receiver_accepted_spans", Outcome: filter.OutcomeDropped},
			},
		},
	}

	rule := &InternalMetricsNotFiltered{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, analyses)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when internal metrics already dropped, got %d", len(findings))
	}
}

func TestInternalMetricsNotFiltered_PartiallyDropped(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("otelcol_receiver_accepted_spans", tap.MetricTypeSum, nil, 100),
		entry("otelcol_exporter_sent_spans", tap.MetricTypeSum, nil, 100),
	}
	// Only one internal metric is dropped, the other is not
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName: "filter/internal",
			Pipeline:      "metrics",
			Results: []filter.MatchResult{
				{MetricName: "otelcol_receiver_accepted_spans", Outcome: filter.OutcomeDropped},
			},
		},
	}

	rule := &InternalMetricsNotFiltered{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, analyses)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when only partially dropped, got %d", len(findings))
	}
}

// --- Rule 2: HighAttributeCount ---

func TestHighAttributeCount_Fires(t *testing.T) {
	keys := make([]string, 12)
	for i := range keys {
		keys[i] = "key_" + string(rune('a'+i))
	}
	entries := []tap.MetricEntry{
		entry("high_attr_metric", tap.MetricTypeSum, keys, 100),
	}

	rule := &HighAttributeCount{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "catalog-high-attribute-count" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
}

func TestHighAttributeCount_NoFire(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("normal_metric", tap.MetricTypeSum, []string{"a", "b", "c"}, 100),
	}

	rule := &HighAttributeCount{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestHighAttributeCount_ExactlyTen(t *testing.T) {
	keys := make([]string, 10)
	for i := range keys {
		keys[i] = "key_" + string(rune('a'+i))
	}
	entries := []tap.MetricEntry{
		entry("border_metric", tap.MetricTypeSum, keys, 100),
	}

	rule := &HighAttributeCount{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for exactly 10 keys, got %d", len(findings))
	}
}

// --- Rule 3: PointCountOutlier ---

func TestPointCountOutlier_Fires(t *testing.T) {
	// 99 entries at 10 points + 1 outlier at 100000.
	// Total = 990 + 100000 = 100990, mean = 1009.9
	// 10 * mean = 10099, outlier (100000) > 10099 and > 1000 → fires
	entries := make([]tap.MetricEntry, 0, 100)
	for i := 0; i < 99; i++ {
		entries = append(entries, entry(fmt.Sprintf("normal_%d", i), tap.MetricTypeSum, nil, 10))
	}
	entries = append(entries, entry("outlier", tap.MetricTypeSum, nil, 100000))

	rule := &PointCountOutlier{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "catalog-point-count-outlier" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
}

func TestPointCountOutlier_NoFire_SimilarCounts(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("m1", tap.MetricTypeSum, nil, 100),
		entry("m2", tap.MetricTypeSum, nil, 150),
		entry("m3", tap.MetricTypeSum, nil, 200),
	}

	rule := &PointCountOutlier{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestPointCountOutlier_NoFire_HighButBelowThreshold(t *testing.T) {
	// Mean is 500, 10x mean = 5000, but max is only 800
	entries := []tap.MetricEntry{
		entry("m1", tap.MetricTypeSum, nil, 500),
		entry("m2", tap.MetricTypeSum, nil, 500),
		entry("m3", tap.MetricTypeSum, nil, 800),
	}

	rule := &PointCountOutlier{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestPointCountOutlier_NoFire_EmptyEntries(t *testing.T) {
	rule := &PointCountOutlier{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestPointCountOutlier_NoFire_Under1000(t *testing.T) {
	// Mean is 10, outlier is 150 which is >10x mean but <1000
	entries := []tap.MetricEntry{
		entry("m1", tap.MetricTypeSum, nil, 10),
		entry("m2", tap.MetricTypeSum, nil, 10),
		entry("outlier", tap.MetricTypeSum, nil, 150),
	}

	rule := &PointCountOutlier{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when outlier < 1000, got %d", len(findings))
	}
}

// --- Rule 4: FilterKeepsEverything ---

func TestFilterKeepsEverything_Fires(t *testing.T) {
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName:  "filter/metrics",
			Pipeline:       "metrics",
			KeptCount:      10,
			DroppedCount:   0,
			UnknownCount:   0,
			HasUnsupported: false,
		},
	}

	rule := &FilterKeepsEverything{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, analyses)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("severity = %q, want info", findings[0].Severity)
	}
}

func TestFilterKeepsEverything_NoFire_SomeDropped(t *testing.T) {
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName: "filter/metrics",
			Pipeline:      "metrics",
			KeptCount:     8,
			DroppedCount:  2,
		},
	}

	rule := &FilterKeepsEverything{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, analyses)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestFilterKeepsEverything_NoFire_HasUnsupported(t *testing.T) {
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName:  "filter/metrics",
			Pipeline:       "metrics",
			KeptCount:      10,
			DroppedCount:   0,
			UnknownCount:   0,
			HasUnsupported: true,
		},
	}

	rule := &FilterKeepsEverything{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, analyses)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when HasUnsupported, got %d", len(findings))
	}
}

func TestFilterKeepsEverything_NoFire_HasUnknown(t *testing.T) {
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName: "filter/metrics",
			Pipeline:      "metrics",
			KeptCount:     8,
			UnknownCount:  2,
		},
	}

	rule := &FilterKeepsEverything{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, analyses)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when there are unknown outcomes, got %d", len(findings))
	}
}

// --- Rule 5: FilterDropsEverything ---

func TestFilterDropsEverything_Fires(t *testing.T) {
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName: "filter/drop-all",
			Pipeline:      "metrics",
			KeptCount:     0,
			DroppedCount:  15,
		},
	}

	rule := &FilterDropsEverything{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, analyses)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("severity = %q, want critical", findings[0].Severity)
	}
	if findings[0].Pipeline != "metrics" {
		t.Errorf("pipeline = %q, want metrics", findings[0].Pipeline)
	}
}

func TestFilterDropsEverything_NoFire_SomeKept(t *testing.T) {
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName: "filter/partial",
			Pipeline:      "metrics",
			KeptCount:     5,
			DroppedCount:  10,
		},
	}

	rule := &FilterDropsEverything{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, analyses)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestFilterDropsEverything_NoFire_NothingDropped(t *testing.T) {
	analyses := []filter.FilterAnalysis{
		{
			ProcessorName: "filter/noop",
			Pipeline:      "metrics",
			KeptCount:     0,
			DroppedCount:  0,
		},
	}

	rule := &FilterDropsEverything{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), nil, analyses)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when nothing dropped, got %d", len(findings))
	}
}

// --- Rule 6: NoFilterHighVolume ---

func TestNoFilterHighVolume_Fires(t *testing.T) {
	entries := make([]tap.MetricEntry, 51)
	for i := range entries {
		entries[i] = entry("metric_"+string(rune('a'+i%26))+string(rune('0'+i/26)), tap.MetricTypeSum, nil, 10)
	}

	cfg := &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines: map[string]config.Pipeline{
			"metrics": {Signal: config.SignalMetrics, Processors: []string{"batch"}},
		},
	}

	rule := &NoFilterHighVolume{}
	findings := rule.EvaluateWithCatalog(cfg, entries, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "catalog-no-filter-high-volume" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
}

func TestNoFilterHighVolume_NoFire_HasAnalyses(t *testing.T) {
	entries := make([]tap.MetricEntry, 100)
	for i := range entries {
		entries[i] = entry("metric_"+string(rune('a'+i%26)), tap.MetricTypeSum, nil, 10)
	}
	analyses := []filter.FilterAnalysis{
		{ProcessorName: "filter/test", Pipeline: "metrics", KeptCount: 50, DroppedCount: 50},
	}

	rule := &NoFilterHighVolume{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, analyses)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when filter analyses exist, got %d", len(findings))
	}
}

func TestNoFilterHighVolume_NoFire_LowVolume(t *testing.T) {
	entries := make([]tap.MetricEntry, 30)
	for i := range entries {
		entries[i] = entry("metric_"+string(rune('a'+i%26)), tap.MetricTypeSum, nil, 10)
	}

	rule := &NoFilterHighVolume{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings with <=50 entries, got %d", len(findings))
	}
}

func TestNoFilterHighVolume_NoFire_HasFilterProcessor(t *testing.T) {
	entries := make([]tap.MetricEntry, 51)
	for i := range entries {
		entries[i] = entry("metric_"+string(rune('a'+i%26))+string(rune('0'+i/26)), tap.MetricTypeSum, nil, 10)
	}

	cfg := &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines: map[string]config.Pipeline{
			"metrics": {Signal: config.SignalMetrics, Processors: []string{"filter/internal", "batch"}},
		},
	}

	rule := &NoFilterHighVolume{}
	findings := rule.EvaluateWithCatalog(cfg, entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when filter processor exists, got %d", len(findings))
	}
}

// --- Rule 7: ManyHistograms ---

func TestManyHistograms_Fires(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("hist_1", tap.MetricTypeHistogram, nil, 100),
		entry("hist_2", tap.MetricTypeHistogram, nil, 100),
		entry("hist_3", tap.MetricTypeHistogram, nil, 100),
		entry("hist_4", tap.MetricTypeHistogram, nil, 100),
		entry("hist_5", tap.MetricTypeHistogram, nil, 100),
		entry("hist_6", tap.MetricTypeExponentialHistogram, nil, 100),
		entry("gauge_1", tap.MetricTypeGauge, nil, 100),
		entry("sum_1", tap.MetricTypeSum, nil, 100),
	}

	rule := &ManyHistograms{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "catalog-many-histograms" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
}

func TestManyHistograms_NoFire_LowCount(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("hist_1", tap.MetricTypeHistogram, nil, 100),
		entry("hist_2", tap.MetricTypeHistogram, nil, 100),
		entry("gauge_1", tap.MetricTypeGauge, nil, 100),
	}

	rule := &ManyHistograms{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings with <=5 histograms, got %d", len(findings))
	}
}

func TestManyHistograms_NoFire_LowPercentage(t *testing.T) {
	entries := make([]tap.MetricEntry, 0, 25)
	for i := 0; i < 6; i++ {
		entries = append(entries, entry(
			"hist_"+string(rune('a'+i)), tap.MetricTypeHistogram, nil, 100))
	}
	for i := 0; i < 19; i++ {
		entries = append(entries, entry(
			"gauge_"+string(rune('a'+i)), tap.MetricTypeGauge, nil, 100))
	}
	// 6 histograms out of 25 = 24%, below 30%

	rule := &ManyHistograms{}
	findings := rule.EvaluateWithCatalog(emptyCfg(), entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when histogram pct <=30%%, got %d", len(findings))
	}
}

func TestManyHistograms_NoFire_HasTransformProcessor(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("hist_1", tap.MetricTypeHistogram, nil, 100),
		entry("hist_2", tap.MetricTypeHistogram, nil, 100),
		entry("hist_3", tap.MetricTypeHistogram, nil, 100),
		entry("hist_4", tap.MetricTypeHistogram, nil, 100),
		entry("hist_5", tap.MetricTypeHistogram, nil, 100),
		entry("hist_6", tap.MetricTypeHistogram, nil, 100),
		entry("gauge_1", tap.MetricTypeGauge, nil, 100),
	}

	cfg := &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines: map[string]config.Pipeline{
			"metrics": {Signal: config.SignalMetrics, Processors: []string{"transform"}},
		},
	}

	rule := &ManyHistograms{}
	findings := rule.EvaluateWithCatalog(cfg, entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings with transform processor, got %d", len(findings))
	}
}

func TestManyHistograms_NoFire_HasCumulativeToDelta(t *testing.T) {
	entries := []tap.MetricEntry{
		entry("hist_1", tap.MetricTypeHistogram, nil, 100),
		entry("hist_2", tap.MetricTypeHistogram, nil, 100),
		entry("hist_3", tap.MetricTypeHistogram, nil, 100),
		entry("hist_4", tap.MetricTypeHistogram, nil, 100),
		entry("hist_5", tap.MetricTypeHistogram, nil, 100),
		entry("hist_6", tap.MetricTypeHistogram, nil, 100),
		entry("gauge_1", tap.MetricTypeGauge, nil, 100),
	}

	cfg := &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines: map[string]config.Pipeline{
			"metrics": {Signal: config.SignalMetrics, Processors: []string{"cumulativetodelta"}},
		},
	}

	rule := &ManyHistograms{}
	findings := rule.EvaluateWithCatalog(cfg, entries, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings with cumulativetodelta processor, got %d", len(findings))
	}
}

// --- Rule 8: ShortScrapeInterval ---

func TestShortScrapeInterval_Prometheus_Fires(t *testing.T) {
	cfg := &config.CollectorConfig{
		Receivers: map[string]config.ComponentConfig{
			"prometheus": {
				Type: "prometheus",
				Name: "prometheus",
				Config: map[string]any{
					"config": map[string]any{
						"scrape_configs": []any{
							map[string]any{
								"job_name":        "myapp",
								"scrape_interval": "15s",
							},
						},
					},
				},
			},
		},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}

	rule := &ShortScrapeInterval{}
	findings := rule.EvaluateWithCatalog(cfg, nil, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "catalog-short-scrape-interval" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("severity = %q, want info", findings[0].Severity)
	}
}

func TestShortScrapeInterval_Prometheus_NoFire_60s(t *testing.T) {
	cfg := &config.CollectorConfig{
		Receivers: map[string]config.ComponentConfig{
			"prometheus": {
				Type: "prometheus",
				Name: "prometheus",
				Config: map[string]any{
					"config": map[string]any{
						"scrape_configs": []any{
							map[string]any{
								"job_name":        "myapp",
								"scrape_interval": "60s",
							},
						},
					},
				},
			},
		},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}

	rule := &ShortScrapeInterval{}
	findings := rule.EvaluateWithCatalog(cfg, nil, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for 60s interval, got %d", len(findings))
	}
}

func TestShortScrapeInterval_HostMetrics_Fires(t *testing.T) {
	cfg := &config.CollectorConfig{
		Receivers: map[string]config.ComponentConfig{
			"hostmetrics": {
				Type: "hostmetrics",
				Name: "hostmetrics",
				Config: map[string]any{
					"collection_interval": "10s",
				},
			},
		},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}

	rule := &ShortScrapeInterval{}
	findings := rule.EvaluateWithCatalog(cfg, nil, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestShortScrapeInterval_HostMetrics_NoFire_2m(t *testing.T) {
	cfg := &config.CollectorConfig{
		Receivers: map[string]config.ComponentConfig{
			"hostmetrics": {
				Type: "hostmetrics",
				Name: "hostmetrics",
				Config: map[string]any{
					"collection_interval": "2m",
				},
			},
		},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}

	rule := &ShortScrapeInterval{}
	findings := rule.EvaluateWithCatalog(cfg, nil, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for 2m interval, got %d", len(findings))
	}
}

func TestShortScrapeInterval_MultipleJobs(t *testing.T) {
	cfg := &config.CollectorConfig{
		Receivers: map[string]config.ComponentConfig{
			"prometheus": {
				Type: "prometheus",
				Name: "prometheus",
				Config: map[string]any{
					"config": map[string]any{
						"scrape_configs": []any{
							map[string]any{
								"job_name":        "fast",
								"scrape_interval": "5s",
							},
							map[string]any{
								"job_name":        "normal",
								"scrape_interval": "60s",
							},
						},
					},
				},
			},
		},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}

	rule := &ShortScrapeInterval{}
	findings := rule.EvaluateWithCatalog(cfg, nil, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (only the fast job), got %d", len(findings))
	}
}

func TestShortScrapeInterval_NoConfig(t *testing.T) {
	cfg := &config.CollectorConfig{
		Receivers: map[string]config.ComponentConfig{
			"prometheus": {
				Type: "prometheus",
				Name: "prometheus",
			},
		},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}

	rule := &ShortScrapeInterval{}
	findings := rule.EvaluateWithCatalog(cfg, nil, nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for nil config, got %d", len(findings))
	}
}

// --- EvaluateWithCatalog integration ---

func TestEvaluateWithCatalog_MixedRules(t *testing.T) {
	// Engine with one catalog rule and one plain rule
	engine := NewEngine(
		&InternalMetricsNotFiltered{},
		&MissingMemoryLimiter{},
	)

	entries := []tap.MetricEntry{
		entry("otelcol_spans", tap.MetricTypeSum, nil, 100),
	}
	cfg := &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines: map[string]config.Pipeline{
			"metrics": {Signal: config.SignalMetrics, Processors: []string{"batch"}},
		},
	}

	findings := engine.EvaluateWithCatalog(cfg, entries, nil)
	// Should only get findings from catalog rules — plain rules are skipped
	// (they're already evaluated by Evaluate/EvaluateWithMetrics)
	hasCatalog := false
	hasStatic := false
	for _, f := range findings {
		if f.RuleID == "catalog-internal-metrics-not-filtered" {
			hasCatalog = true
		}
		if f.RuleID == "missing-memory-limiter" {
			hasStatic = true
		}
	}
	if !hasCatalog {
		t.Error("expected catalog rule finding")
	}
	if hasStatic {
		t.Error("plain rules should not be evaluated by EvaluateWithCatalog")
	}
}

func TestEvaluateWithCatalog_CatalogRuleFallsBackToEvaluate(t *testing.T) {
	// CatalogRule.Evaluate should return nil for all catalog rules
	catalogRules := []Rule{
		&InternalMetricsNotFiltered{},
		&HighAttributeCount{},
		&PointCountOutlier{},
		&FilterKeepsEverything{},
		&FilterDropsEverything{},
		&NoFilterHighVolume{},
		&ManyHistograms{},
		&ShortScrapeInterval{},
	}

	for _, r := range catalogRules {
		findings := r.Evaluate(emptyCfg())
		if findings != nil {
			t.Errorf("rule %q Evaluate should return nil, got %v", r.ID(), findings)
		}
	}
}
