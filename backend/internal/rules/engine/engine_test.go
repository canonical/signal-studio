package engine

import (
	"testing"
	"time"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/rules/catalog"
	"github.com/canonical/signal-studio/internal/rules/static"
	"github.com/canonical/signal-studio/internal/tap"
)

func mustParse(t *testing.T, yaml string) *config.CollectorConfig {
	t.Helper()
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse test config: %v", err)
	}
	return cfg
}

func emptyCfg() *config.CollectorConfig {
	return &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{},
		Pipelines:  map[string]config.Pipeline{},
	}
}

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

func makeSnapshot(t0 time.Time, offsetSec int, samples []metrics.MetricSample) *metrics.Snapshot {
	return &metrics.Snapshot{
		CollectedAt: t0.Add(time.Duration(offsetSec) * time.Second),
		Samples:     samples,
	}
}

func storeWithSnapshots(snaps ...*metrics.Snapshot) *metrics.Store {
	s := metrics.NewStore()
	for _, snap := range snaps {
		s.Push(snap)
	}
	return s
}

// --- Engine integration: DefaultEngine ---

func TestDefaultEngine(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
  prometheus:
exporters:
  debug:
  otlp/backend:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug, otlp/backend]
    logs:
      receivers: [otlp]
      exporters: [debug]
`)
	engine := NewDefaultEngine()
	findings := engine.Evaluate(cfg)

	// Expect findings from multiple rules
	if len(findings) == 0 {
		t.Fatal("expected findings from default engine")
	}

	// Should have: missing memory_limiter (x2), missing batch (x2),
	// no sampling (traces), unused prometheus, multiple exporters (traces),
	// no log severity filter
	ruleHits := make(map[string]int)
	for _, f := range findings {
		ruleHits[f.RuleID]++
	}

	expectedRules := []string{
		"missing-memory-limiter",
		"missing-batch",
		"no-trace-sampling",
		"unused-components",
		"multiple-exporters-no-routing",
		"no-log-severity-filter",
	}
	for _, r := range expectedRules {
		if ruleHits[r] == 0 {
			t.Errorf("expected at least one finding from rule %q", r)
		}
	}
}

// --- Engine integration: DefaultEngine with extended rules ---

func TestDefaultEngineExtendedRules(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
processors:
  batch:
  memory_limiter:
    check_interval: 1s
  filter:
    logs:
      log_record:
        - 'severity_number < 9'
exporters:
  debug:
  otlp/backend:
    endpoint: backend:4317
extensions:
  pprof:
    endpoint: localhost:1777
service:
  extensions: [pprof]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, memory_limiter]
      exporters: [debug, otlp/backend]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, filter, batch]
      exporters: [otlp/backend]
`)
	engine := NewDefaultEngine()
	findings := engine.Evaluate(cfg)

	ruleHits := make(map[string]int)
	for _, f := range findings {
		ruleHits[f.RuleID]++
	}

	// Expected new rule hits:
	// memory-limiter-not-first: traces (batch, memory_limiter -> idx 1)
	// batch-before-sampling: none (no sampler)
	// receiver-endpoint-wildcard: otlp (0.0.0.0)
	// debug-exporter-in-pipeline: traces (debug)
	// pprof-extension-enabled: pprof
	// memory-limiter-without-limits: memory_limiter (no limit_mib or limit_percentage)
	// exporter-no-sending-queue: otlp/backend
	// exporter-no-retry: otlp/backend
	// filter-error-mode-propagate: filter (default propagate)

	expectedNewRules := []string{
		"memory-limiter-not-first",
		"receiver-endpoint-wildcard",
		"debug-exporter-in-pipeline",
		"pprof-extension-enabled",
		"memory-limiter-without-limits",
		"exporter-no-sending-queue",
		"exporter-no-retry",
		"filter-error-mode-propagate",
		"no-health-check-trace-filter",
	}
	for _, r := range expectedNewRules {
		if ruleHits[r] == 0 {
			t.Errorf("expected at least one finding from new rule %q", r)
		}
	}
}

// --- Engine integration: EvaluateWithMetrics ---

func TestEvaluateWithMetricsIntegration(t *testing.T) {
	cfg := &config.CollectorConfig{
		Receivers:  map[string]config.ComponentConfig{"otlp": {Type: "otlp", Name: "otlp"}},
		Processors: map[string]config.ComponentConfig{},
		Exporters:  map[string]config.ComponentConfig{"otlp/backend": {Type: "otlp", Name: "otlp/backend"}},
		Pipelines: map[string]config.Pipeline{
			"traces": {Signal: "traces", Receivers: []string{"otlp"}, Exporters: []string{"otlp/backend"}},
		},
	}

	engine := NewDefaultEngine()

	// Without metrics — live rules produce nothing, static rules still work
	findings := engine.EvaluateWithMetrics(cfg, nil)
	staticCount := 0
	for _, f := range findings {
		if len(f.RuleID) >= 5 && f.RuleID[:5] != "live-" {
			staticCount++
		}
	}
	if staticCount == 0 {
		t.Error("expected static findings even without metrics")
	}
	for _, f := range findings {
		if len(f.RuleID) >= 5 && f.RuleID[:5] == "live-" {
			t.Errorf("unexpected live finding without metrics: %s", f.RuleID)
		}
	}

	// With metrics — live rules can fire
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 50000},
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 1000},
		}),
	)

	findings = engine.EvaluateWithMetrics(cfg, store)
	hasLive := false
	for _, f := range findings {
		if f.RuleID == "live-log-volume-dominance" {
			hasLive = true
		}
	}
	if !hasLive {
		t.Error("expected live-log-volume-dominance finding with metrics data")
	}
}

// --- Engine integration: EvaluateWithCatalog mixed rules ---

func TestEvaluateWithCatalog_MixedRules(t *testing.T) {
	// Engine with one catalog rule and one plain rule
	engine := NewEngine(
		&catalog.InternalMetricsNotFiltered{},
		&static.MissingMemoryLimiter{},
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

// --- Engine integration: CatalogRule.Evaluate returns nil ---

func TestEvaluateWithCatalog_CatalogRuleFallsBackToEvaluate(t *testing.T) {
	// CatalogRule.Evaluate should return nil for all catalog rules
	catalogRules := catalog.AllRules()

	for _, r := range catalogRules {
		if _, ok := r.(rules.CatalogRule); !ok {
			continue
		}
		findings := r.Evaluate(emptyCfg())
		if findings != nil {
			t.Errorf("rule %q Evaluate should return nil, got %v", r.ID(), findings)
		}
	}
}

// TestRuleMetadata verifies every rule has non-empty Description and DefaultSeverity.
func TestRuleMetadata(t *testing.T) {
	engine := NewDefaultEngine()
	seen := make(map[string]bool)
	for _, r := range engine.Rules() {
		if seen[r.ID()] {
			t.Errorf("duplicate rule ID %q", r.ID())
		}
		seen[r.ID()] = true

		if r.Description() == "" {
			t.Errorf("rule %q has empty Description()", r.ID())
		}
		sev := r.DefaultSeverity()
		if sev != rules.SeverityInfo && sev != rules.SeverityWarning && sev != rules.SeverityCritical {
			t.Errorf("rule %q has invalid DefaultSeverity: %q", r.ID(), sev)
		}
	}
}

// Ensure unused imports are used.
var _ = filter.OutcomeDropped
