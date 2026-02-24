package metrics

import (
	"testing"
	"time"
)

func TestStorePushAndLatest(t *testing.T) {
	store := NewStore()

	if store.Latest() != nil {
		t.Error("expected nil for empty store")
	}

	snap1 := &Snapshot{CollectedAt: time.Now()}
	store.Push(snap1)

	if got := store.Latest(); got != snap1 {
		t.Error("Latest() should return the pushed snapshot")
	}
	if store.Len() != 1 {
		t.Errorf("Len() = %d, want 1", store.Len())
	}
}

func TestStorePrevious(t *testing.T) {
	store := NewStore()

	if store.Previous() != nil {
		t.Error("expected nil for empty store")
	}

	snap1 := &Snapshot{CollectedAt: time.Now()}
	store.Push(snap1)
	if store.Previous() != nil {
		t.Error("expected nil with only one snapshot")
	}

	snap2 := &Snapshot{CollectedAt: time.Now().Add(10 * time.Second)}
	store.Push(snap2)
	if got := store.Previous(); got != snap1 {
		t.Error("Previous() should return the first snapshot")
	}
}

func TestStoreEviction(t *testing.T) {
	store := NewStore() // default window = 6

	for i := 0; i < 10; i++ {
		store.Push(&Snapshot{
			CollectedAt: time.Now().Add(time.Duration(i) * 10 * time.Second),
		})
	}

	if store.Len() != 6 {
		t.Errorf("Len() = %d, want 6 (window size)", store.Len())
	}

	window := store.Window()
	if len(window) != 6 {
		t.Errorf("Window() len = %d, want 6", len(window))
	}
}

func TestStoreClear(t *testing.T) {
	store := NewStore()
	store.Push(&Snapshot{CollectedAt: time.Now()})
	store.Push(&Snapshot{CollectedAt: time.Now()})

	store.Clear()

	if store.Len() != 0 {
		t.Errorf("Len() = %d after Clear, want 0", store.Len())
	}
	if store.Latest() != nil {
		t.Error("Latest() should be nil after Clear")
	}
}

func TestRatePerSecond(t *testing.T) {
	t0 := time.Now()
	t1 := t0.Add(10 * time.Second)

	prev := &Snapshot{
		CollectedAt: t0,
		Samples: []MetricSample{
			{Name: "otelcol_receiver_accepted_spans", Labels: map[string]string{"receiver": "otlp"}, Value: 1000},
		},
	}
	curr := &Snapshot{
		CollectedAt: t1,
		Samples: []MetricSample{
			{Name: "otelcol_receiver_accepted_spans", Labels: map[string]string{"receiver": "otlp"}, Value: 2000},
		},
	}

	rate := RatePerSecond(prev, curr, "otelcol_receiver_accepted_spans", map[string]string{"receiver": "otlp"})
	expected := 100.0 // (2000-1000)/10
	if rate != expected {
		t.Errorf("rate = %f, want %f", rate, expected)
	}
}

func TestRatePerSecondCounterReset(t *testing.T) {
	t0 := time.Now()
	t1 := t0.Add(10 * time.Second)

	prev := &Snapshot{
		CollectedAt: t0,
		Samples: []MetricSample{
			{Name: "counter", Labels: map[string]string{}, Value: 5000},
		},
	}
	curr := &Snapshot{
		CollectedAt: t1,
		Samples: []MetricSample{
			{Name: "counter", Labels: map[string]string{}, Value: 100}, // reset
		},
	}

	rate := RatePerSecond(prev, curr, "counter", map[string]string{})
	if rate != 0 {
		t.Errorf("rate = %f on counter reset, want 0", rate)
	}
}

func TestRatePerSecondNilSnapshots(t *testing.T) {
	snap := &Snapshot{
		CollectedAt: time.Now(),
		Samples: []MetricSample{
			{Name: "counter", Labels: map[string]string{}, Value: 100},
		},
	}

	if rate := RatePerSecond(nil, snap, "counter", map[string]string{}); rate != 0 {
		t.Errorf("rate with nil prev = %f, want 0", rate)
	}
	if rate := RatePerSecond(snap, nil, "counter", map[string]string{}); rate != 0 {
		t.Errorf("rate with nil curr = %f, want 0", rate)
	}
}

func TestGaugeValue(t *testing.T) {
	snap := &Snapshot{
		Samples: []MetricSample{
			{Name: "otelcol_exporter_queue_size", Labels: map[string]string{"exporter": "otlp/backend"}, Value: 145},
		},
	}

	val, ok := GaugeValue(snap, "otelcol_exporter_queue_size", map[string]string{"exporter": "otlp/backend"})
	if !ok || val != 145 {
		t.Errorf("gauge = %f (ok=%v), want 145", val, ok)
	}

	_, ok = GaugeValue(snap, "nonexistent", map[string]string{})
	if ok {
		t.Error("expected ok=false for nonexistent metric")
	}

	_, ok = GaugeValue(nil, "otelcol_exporter_queue_size", map[string]string{})
	if ok {
		t.Error("expected ok=false for nil snapshot")
	}
}

func TestComputeSnapshot(t *testing.T) {
	t0 := time.Now()
	t1 := t0.Add(10 * time.Second)

	store := NewStore()
	store.Push(&Snapshot{
		CollectedAt: t0,
		Samples: []MetricSample{
			{Name: MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 10000},
			{Name: MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 9000},
			{Name: MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 10},
			{Name: MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 100},
			{Name: MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 1000},
		},
	})
	store.Push(&Snapshot{
		CollectedAt: t1,
		Samples: []MetricSample{
			{Name: MetricReceiverAcceptedSpans, Labels: map[string]string{"receiver": "otlp"}, Value: 20000},
			{Name: MetricExporterSentSpans, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 18000},
			{Name: MetricExporterSendFailedSpans, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 30},
			{Name: MetricExporterQueueSize, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 200},
			{Name: MetricExporterQueueCapacity, Labels: map[string]string{"exporter": "otlp/backend"}, Value: 1000},
		},
	})

	cs := ComputeSnapshot(store, "connected")

	if cs.Status != "connected" {
		t.Errorf("status = %q, want connected", cs.Status)
	}

	// Receiver: (20000-10000)/10 = 1000 spans/s
	recv, ok := cs.Receivers["otlp"]
	if !ok {
		t.Fatal("missing receiver 'otlp'")
	}
	if recv.AcceptedSpansRate != 1000 {
		t.Errorf("receiver accepted rate = %f, want 1000", recv.AcceptedSpansRate)
	}

	// Exporter: (18000-9000)/10 = 900 spans/s
	exp, ok := cs.Exporters["otlp/backend"]
	if !ok {
		t.Fatal("missing exporter 'otlp/backend'")
	}
	if exp.SentSpansRate != 900 {
		t.Errorf("exporter sent rate = %f, want 900", exp.SentSpansRate)
	}
	if exp.QueueSize != 200 {
		t.Errorf("queue size = %f, want 200", exp.QueueSize)
	}
	if exp.QueueUtilizationPct != 20 {
		t.Errorf("queue utilization = %f, want 20", exp.QueueUtilizationPct)
	}

	// Signal: accepted=1000, sent=900, drop=10%
	traces, ok := cs.Signals["traces"]
	if !ok {
		t.Fatal("missing signal 'traces'")
	}
	if traces.ReceiverAcceptedRate != 1000 {
		t.Errorf("traces accepted rate = %f, want 1000", traces.ReceiverAcceptedRate)
	}
	if traces.ExporterSentRate != 900 {
		t.Errorf("traces sent rate = %f, want 900", traces.ExporterSentRate)
	}
	if traces.DropRatePct != 10 {
		t.Errorf("traces drop rate = %f, want 10", traces.DropRatePct)
	}
}

func TestComputeSnapshotEmpty(t *testing.T) {
	store := NewStore()
	cs := ComputeSnapshot(store, "disconnected")

	if cs.Status != "disconnected" {
		t.Errorf("status = %q, want disconnected", cs.Status)
	}
	if len(cs.Signals) != 0 && cs.Signals["traces"].ReceiverAcceptedRate != 0 {
		t.Error("expected zero rates for empty store")
	}
}

func TestMatchLabels(t *testing.T) {
	sample := map[string]string{"receiver": "otlp", "transport": "grpc"}

	if !matchLabels(sample, map[string]string{"receiver": "otlp"}) {
		t.Error("should match subset")
	}
	if !matchLabels(sample, map[string]string{}) {
		t.Error("empty required should match anything")
	}
	if matchLabels(sample, map[string]string{"receiver": "jaeger"}) {
		t.Error("should not match different value")
	}
	if matchLabels(sample, map[string]string{"exporter": "otlp"}) {
		t.Error("should not match missing key")
	}
}
