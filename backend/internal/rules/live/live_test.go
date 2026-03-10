package live

import (
	"testing"
	"time"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
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
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("severity = %q, want warning", findings[0].Severity)
	}
	if findings[0].Confidence != rules.ConfidenceMedium {
		t.Errorf("confidence = %q, want medium", findings[0].Confidence)
	}
	if findings[0].Implication == "" {
		t.Error("expected non-empty implication")
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

func TestHighDropRateSkipsTracesWithSampler(t *testing.T) {
	t0 := time.Now()
	// High trace drop rate, but a sampler is configured — should NOT fire for traces.
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 1000},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 2000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 30000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 3000},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 40000},
			{Name: metrics.MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 4000},
		}),
	)

	cfg := &config.CollectorConfig{
		Pipelines: map[string]config.Pipeline{
			"traces": {Signal: config.SignalTraces, Processors: []string{"memory_limiter", "tail_sampling", "batch"}},
		},
	}
	rule := &HighDropRate{}
	findings := rule.EvaluateWithMetrics(cfg, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings when sampler is present, got %d: %v", len(findings), findings)
	}
}

func TestHighDropRateStillFiresForLogsWithSampler(t *testing.T) {
	t0 := time.Now()
	// High log drop rate with a traces sampler — should still fire for logs.
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 10000},
			{Name: metrics.MetricExporterSentLogRecords, Labels: map[string]string{"exporter": "otlp"}, Value: 1000},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 20000},
			{Name: metrics.MetricExporterSentLogRecords, Labels: map[string]string{"exporter": "otlp"}, Value: 2000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 30000},
			{Name: metrics.MetricExporterSentLogRecords, Labels: map[string]string{"exporter": "otlp"}, Value: 3000},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "filelog"}, Value: 40000},
			{Name: metrics.MetricExporterSentLogRecords, Labels: map[string]string{"exporter": "otlp"}, Value: 4000},
		}),
	)

	cfg := &config.CollectorConfig{
		Pipelines: map[string]config.Pipeline{
			"traces": {Signal: config.SignalTraces, Processors: []string{"tail_sampling", "batch"}},
			"logs":   {Signal: config.SignalLogs, Processors: []string{"batch"}},
		},
	}
	rule := &HighDropRate{}
	findings := rule.EvaluateWithMetrics(cfg, store)
	if len(findings) == 0 {
		t.Fatal("expected findings for high log drop rate even with traces sampler")
	}
	if findings[0].Title != "High drop rate on logs pipeline" {
		t.Errorf("expected logs finding, got %q", findings[0].Title)
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

func TestReceiverExporterMismatchSkipsTracesWithSampler(t *testing.T) {
	t0 := time.Now()
	// Accepted >> sent for traces, but probabilistic_sampler is configured.
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

	cfg := &config.CollectorConfig{
		Pipelines: map[string]config.Pipeline{
			"traces": {Signal: config.SignalTraces, Processors: []string{"probabilistic_sampler", "batch"}},
		},
	}
	rule := &ReceiverExporterMismatch{}
	findings := rule.EvaluateWithMetrics(cfg, store)
	if len(findings) != 0 {
		t.Errorf("expected no findings when sampler is present, got %d: %v", len(findings), findings)
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

// --- ExporterSustainedFailures ---

func TestExporterSustainedFailures_Fires(t *testing.T) {
	t0 := time.Now()
	// 5 snapshots with increasing failure count for 4 intervals — sustained.
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 100},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 200},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 300},
		}),
		makeSnapshot(t0, 40, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 400},
		}),
	)

	rule := &ExporterSustainedFailures{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestExporterSustainedFailures_NoFire_TransientFailure(t *testing.T) {
	t0 := time.Now()
	// Failure in only 1 interval, not sustained.
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 50},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 50},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 50},
		}),
	)

	rule := &ExporterSustainedFailures{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for transient failure, got %d", len(findings))
	}
}

func TestExporterSustainedFailures_NoFire_InsufficientData(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp"}, Value: 100},
		}),
	)

	rule := &ExporterSustainedFailures{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with insufficient data, got %d", len(findings))
	}
}

func TestExporterSustainedFailures_StaticReturnsNil(t *testing.T) {
	rule := &ExporterSustainedFailures{}
	if findings := rule.Evaluate(&config.CollectorConfig{}); findings != nil {
		t.Error("Evaluate() should return nil")
	}
}

// --- ReceiverBackpressure ---

func TestReceiverBackpressure_Fires(t *testing.T) {
	t0 := time.Now()
	// Baseline of ~1000/s for first intervals, then drops to ~200/s.
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 22000},
		}),
		makeSnapshot(t0, 40, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 24000},
		}),
	)

	rule := &ReceiverBackpressure{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestReceiverBackpressure_NoFire_SteadyTraffic(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 30000},
		}),
		makeSnapshot(t0, 40, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 40000},
		}),
	)

	rule := &ReceiverBackpressure{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for steady traffic, got %d", len(findings))
	}
}

func TestReceiverBackpressure_NoFire_LowTraffic(t *testing.T) {
	t0 := time.Now()
	// Even a big percentage drop doesn't fire if baseline is < 10/s.
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 50},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 100},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 100},
		}),
		makeSnapshot(t0, 40, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 100},
		}),
	)

	rule := &ReceiverBackpressure{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for low traffic baseline, got %d", len(findings))
	}
}

func TestReceiverBackpressure_StaticReturnsNil(t *testing.T) {
	rule := &ReceiverBackpressure{}
	if findings := rule.Evaluate(&config.CollectorConfig{}); findings != nil {
		t.Error("Evaluate() should return nil")
	}
}

// --- ZeroThroughput ---

func TestZeroThroughput_Fires(t *testing.T) {
	t0 := time.Now()
	// 5 snapshots, all with zero values — 4 intervals of zero traffic.
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedMetricPoints, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedMetricPoints, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedMetricPoints, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedMetricPoints, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 40, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedLogRecords, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
			{Name: metrics.MetricReceiverAcceptedMetricPoints, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
	)

	rule := &ZeroThroughput{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestZeroThroughput_NoFire_HasTraffic(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 100},
		}),
		makeSnapshot(t0, 20, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 200},
		}),
		makeSnapshot(t0, 30, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 300},
		}),
	)

	rule := &ZeroThroughput{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when traffic exists, got %d", len(findings))
	}
}

func TestZeroThroughput_NoFire_InsufficientData(t *testing.T) {
	t0 := time.Now()
	store := storeWithSnapshots(
		makeSnapshot(t0, 0, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
		makeSnapshot(t0, 10, []metrics.MetricSample{
			{Name: metrics.MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 0},
		}),
	)

	rule := &ZeroThroughput{}
	findings := rule.EvaluateWithMetrics(&config.CollectorConfig{}, store)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with insufficient data, got %d", len(findings))
	}
}

func TestZeroThroughput_StaticReturnsNil(t *testing.T) {
	rule := &ZeroThroughput{}
	if findings := rule.Evaluate(&config.CollectorConfig{}); findings != nil {
		t.Error("Evaluate() should return nil")
	}
}
