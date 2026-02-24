package tap

import (
	"sync"
	"testing"
	"time"
)

func TestCatalog_RecordAndRetrieve(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	c.Record("http.server.duration", MetricTypeHistogram, []string{"method", "status_code"}, 100)

	if c.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", c.Len())
	}

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Name != "http.server.duration" {
		t.Errorf("expected name http.server.duration, got %s", e.Name)
	}
	if e.Type != MetricTypeHistogram {
		t.Errorf("expected type histogram, got %s", e.Type)
	}
	if e.PointCount != 100 {
		t.Errorf("expected point count 100, got %d", e.PointCount)
	}
	if e.ScrapeCount != 1 {
		t.Errorf("expected scrape count 1, got %d", e.ScrapeCount)
	}
	if len(e.AttributeKeys) != 2 {
		t.Fatalf("expected 2 attribute keys, got %d", len(e.AttributeKeys))
	}
	if e.AttributeKeys[0] != "method" || e.AttributeKeys[1] != "status_code" {
		t.Errorf("unexpected attribute keys: %v", e.AttributeKeys)
	}
}

func TestCatalog_AttributeKeyMerging(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	c.Record("my.metric", MetricTypeGauge, []string{"a", "b"}, 10)
	c.Record("my.metric", MetricTypeGauge, []string{"b", "c"}, 20)

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	expected := []string{"a", "b", "c"}
	if len(e.AttributeKeys) != len(expected) {
		t.Fatalf("expected %d attribute keys, got %d: %v", len(expected), len(e.AttributeKeys), e.AttributeKeys)
	}
	for i, k := range expected {
		if e.AttributeKeys[i] != k {
			t.Errorf("expected key %q at index %d, got %q", k, i, e.AttributeKeys[i])
		}
	}
}

func TestCatalog_PointCountAccumulation(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	c.Record("cpu.usage", MetricTypeGauge, nil, 50)
	c.Record("cpu.usage", MetricTypeGauge, nil, 75)

	entries := c.Entries()
	if entries[0].PointCount != 125 {
		t.Errorf("expected point count 125, got %d", entries[0].PointCount)
	}
	if entries[0].ScrapeCount != 2 {
		t.Errorf("expected scrape count 2, got %d", entries[0].ScrapeCount)
	}
}

func TestCatalog_TTLExpiry(t *testing.T) {
	c := NewCatalog(50 * time.Millisecond)

	c.Record("old.metric", MetricTypeSum, nil, 10)
	time.Sleep(100 * time.Millisecond)
	c.Record("new.metric", MetricTypeGauge, nil, 5)

	c.Prune()

	if c.Len() != 1 {
		t.Fatalf("expected 1 entry after prune, got %d", c.Len())
	}
	names := c.Names()
	if names[0] != "new.metric" {
		t.Errorf("expected new.metric to survive prune, got %s", names[0])
	}
}

func TestCatalog_Prune(t *testing.T) {
	c := NewCatalog(50 * time.Millisecond)

	c.Record("m1", MetricTypeGauge, nil, 1)
	c.Record("m2", MetricTypeGauge, nil, 1)
	time.Sleep(100 * time.Millisecond)

	c.Prune()

	if c.Len() != 0 {
		t.Errorf("expected 0 entries after prune, got %d", c.Len())
	}
}

func TestCatalog_Clear(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	c.Record("m1", MetricTypeGauge, nil, 1)
	c.Record("m2", MetricTypeSum, nil, 2)

	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", c.Len())
	}
}

func TestCatalog_Names(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	c.Record("z.metric", MetricTypeGauge, nil, 1)
	c.Record("a.metric", MetricTypeSum, nil, 1)
	c.Record("m.metric", MetricTypeHistogram, nil, 1)

	names := c.Names()
	expected := []string{"a.metric", "m.metric", "z.metric"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d", len(expected), len(names))
	}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("expected %q at index %d, got %q", n, i, names[i])
		}
	}
}

func TestCatalog_ConcurrentAccess(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Record("concurrent.metric", MetricTypeGauge, []string{"key"}, 1)
		}(i)
	}

	wg.Wait()

	if c.Len() != 1 {
		t.Errorf("expected 1 entry, got %d", c.Len())
	}
	entries := c.Entries()
	if entries[0].PointCount != 100 {
		t.Errorf("expected point count 100, got %d", entries[0].PointCount)
	}
}

func TestCatalog_EmptyAttrKeys(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	c.Record("no.attrs", MetricTypeGauge, nil, 5)
	entries := c.Entries()
	if len(entries[0].AttributeKeys) != 0 {
		t.Errorf("expected 0 attribute keys, got %d", len(entries[0].AttributeKeys))
	}
}

func TestCatalog_RateChanged_InsufficientData(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	// No data at all.
	if c.RateChanged() {
		t.Error("expected RateChanged=false with no data")
	}

	// Single batch in one window only.
	c.RecordBatch(100)
	if c.RateChanged() {
		t.Error("expected RateChanged=false with only one window")
	}
}

func TestCatalog_RateChanged_NeedsTwoBatches(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.windowDur = 50 * time.Millisecond

	// First window: only 1 batch.
	c.RecordBatch(100)
	time.Sleep(55 * time.Millisecond)

	// Second window: only 1 batch.
	c.RecordBatch(500)

	// Should not trigger — need ≥2 batches per window.
	if c.RateChanged() {
		t.Error("expected RateChanged=false with <2 batches per window")
	}
}

func TestCatalog_RateChanged_StableRate(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.windowDur = 50 * time.Millisecond

	// First window: 3 batches of 100 pts each → avg 100 pts/batch.
	c.RecordBatch(100)
	c.RecordBatch(100)
	c.RecordBatch(100)
	time.Sleep(55 * time.Millisecond) // triggers rotation

	// Second window: 3 batches of 100 pts each → avg 100 pts/batch.
	c.RecordBatch(100)
	c.RecordBatch(100)
	c.RecordBatch(100)

	if c.RateChanged() {
		t.Error("expected RateChanged=false when points-per-batch is stable")
	}
}

func TestCatalog_RateChanged_DetectsChange(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.windowDur = 50 * time.Millisecond

	// First window: 3 batches of 100 pts → avg 100 pts/batch.
	c.RecordBatch(100)
	c.RecordBatch(100)
	c.RecordBatch(100)
	time.Sleep(55 * time.Millisecond) // triggers rotation

	// Second window: 3 batches of 500 pts → avg 500 pts/batch (5x change).
	c.RecordBatch(500)
	c.RecordBatch(500)
	c.RecordBatch(500)

	if !c.RateChanged() {
		t.Error("expected RateChanged=true when points-per-batch changed significantly")
	}
}

func TestCatalog_RateChanged_ToleratesMinorJitter(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.windowDur = 50 * time.Millisecond

	// First window: avg ~100 pts/batch.
	c.RecordBatch(95)
	c.RecordBatch(105)
	c.RecordBatch(100)
	time.Sleep(55 * time.Millisecond)

	// Second window: avg ~110 pts/batch (10% increase, below 50% threshold).
	c.RecordBatch(108)
	c.RecordBatch(115)
	c.RecordBatch(107)

	if c.RateChanged() {
		t.Error("expected RateChanged=false for minor batch size jitter")
	}
}

func TestCatalog_ClearResetsRateTracking(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.windowDur = 50 * time.Millisecond

	c.RecordBatch(100)
	c.RecordBatch(100)
	time.Sleep(55 * time.Millisecond)
	c.RecordBatch(500)
	c.RecordBatch(500)

	c.Clear()

	if c.RateChanged() {
		t.Error("expected RateChanged=false after Clear()")
	}
}

func TestCatalog_Timestamps(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	c.Record("ts.metric", MetricTypeGauge, nil, 1)
	entries := c.Entries()
	first := entries[0].FirstSeenAt

	time.Sleep(10 * time.Millisecond)
	c.Record("ts.metric", MetricTypeGauge, nil, 1)

	entries = c.Entries()
	if !entries[0].FirstSeenAt.Equal(first) {
		t.Error("FirstSeenAt should not change on subsequent records")
	}
	if !entries[0].LastSeenAt.After(first) {
		t.Error("LastSeenAt should advance on subsequent records")
	}
}
