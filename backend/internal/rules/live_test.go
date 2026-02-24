package rules

import (
	"testing"
	"time"

	"github.com/simskij/otel-signal-lens/internal/config"
	"github.com/simskij/otel-signal-lens/internal/metrics"
)

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

func TestHighDropRateFires(t *testing.T) {
	t0 := time.Now()
	// 3 intervals where accepted >> sent (>10% drop)
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 8000},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 15000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 30000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 22000},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 40000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 29000},
		}),
	)

	rule := &HighDropRate{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) == 0 {
		t.Fatal("expected findings for high drop rate")
	}
	if findings[0].RuleID != "live-high-drop-rate" {
		t.Errorf("ruleId = %q, want live-high-drop-rate", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("severity = %q, want warning", findings[0].Severity)
	}
}

func TestHighDropRateNoFire(t *testing.T) {
	t0 := time.Now()
	// All data sent successfully (0% drop)
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 10000},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 20000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 30000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 30000},
		}),
	)

	rule := &HighDropRate{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestHighDropRateInsufficientData(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
		}),
	)

	rule := &HighDropRate{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings with < 3 snapshots, got %d", len(findings))
	}
}

func TestHighDropRateStaticReturnsNil(t *testing.T) {
	rule := &HighDropRate{}
	findings := rule.Evaluate(&config.CollectorConfig{})
	if findings != nil {
		t.Errorf("Evaluate() should return nil, got %v", findings)
	}
}

func TestLogVolumeDominanceFires(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 40000},
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 1000},
		}),
	)

	rule := &LogVolumeDominance{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) == 0 {
		t.Fatal("expected finding for log dominance")
	}
	if findings[0].RuleID != "live-log-volume-dominance" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
}

func TestLogVolumeDominanceNoFire(t *testing.T) {
	t0 := time.Now()
	// Logs at 2x traces — below 3x threshold
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 20000},
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
		}),
	)

	rule := &LogVolumeDominance{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestLogVolumeDominanceNoTraces(t *testing.T) {
	t0 := time.Now()
	// No traces at all — should not fire
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 40000},
		}),
	)

	rule := &LogVolumeDominance{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings when there are no traces, got %d", len(findings))
	}
}

func TestQueueNearCapacityFires(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 850},
			{Name: metrics.MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 1000},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 900},
			{Name: metrics.MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 1000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 920},
			{Name: metrics.MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 1000},
		}),
	)

	rule := &QueueNearCapacity{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) == 0 {
		t.Fatal("expected finding for queue near capacity")
	}
	if findings[0].RuleID != "live-queue-near-capacity" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
}

func TestQueueNearCapacityNoFire(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp"}, Value: 100},
			{Name: metrics.MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp"}, Value: 1000},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp"}, Value: 150},
			{Name: metrics.MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp"}, Value: 1000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp"}, Value: 120},
			{Name: metrics.MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp"}, Value: 1000},
		}),
	)

	rule := &QueueNearCapacity{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestReceiverExporterMismatchFires(t *testing.T) {
	t0 := time.Now()
	// Accepted consistently > 2x sent across 4+ intervals
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 3000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 6000},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 30000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 9000},
		}),
		makeSnapshot(t0, 40, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 40000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 12000},
		}),
	)

	rule := &ReceiverExporterMismatch{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) == 0 {
		t.Fatal("expected finding for receiver-exporter mismatch")
	}
	if findings[0].RuleID != "live-receiver-exporter-mismatch" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
}

func TestReceiverExporterMismatchNoFire(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 9500},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 19000},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 30000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 28500},
		}),
		makeSnapshot(t0, 40, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 40000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 38000},
		}),
	)

	rule := &ReceiverExporterMismatch{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

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
		if f.RuleID[:5] != "live-" {
			staticCount++
		}
	}
	if staticCount == 0 {
		t.Error("expected static findings even without metrics")
	}
	for _, f := range findings {
		if f.RuleID[:5] == "live-" {
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
