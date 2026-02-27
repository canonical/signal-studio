package tap

import (
	"fmt"
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

func TestCatalog_RecordAttributes_Basic(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("my.metric", MetricTypeGauge, nil, 1)

	c.RecordAttributes("my.metric", AttributeLevelResource, []AttributeKV{
		{Key: "service.name", Value: "frontend"},
		{Key: "deployment.environment", Value: "prod"},
	})

	entries := c.Entries()
	if len(entries[0].Attributes) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(entries[0].Attributes))
	}
	// Sorted by level then key
	a0 := entries[0].Attributes[0]
	if a0.Key != "deployment.environment" || a0.Level != AttributeLevelResource {
		t.Errorf("unexpected first attr: %+v", a0)
	}
	if len(a0.SampleValues) != 1 || a0.SampleValues[0] != "prod" {
		t.Errorf("unexpected sample values: %v", a0.SampleValues)
	}
	if a0.UniqueCount != 1 {
		t.Errorf("expected unique count 1, got %d", a0.UniqueCount)
	}
}

func TestCatalog_RecordAttributes_CappingAt25(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("my.metric", MetricTypeGauge, nil, 1)

	for i := 0; i < 30; i++ {
		c.RecordAttributes("my.metric", AttributeLevelDatapoint, []AttributeKV{
			{Key: "id", Value: fmt.Sprintf("val-%d", i)},
		})
	}

	entries := c.Entries()
	attr := entries[0].Attributes[0]
	if len(attr.SampleValues) != MaxSampleValues {
		t.Errorf("expected %d samples, got %d", MaxSampleValues, len(attr.SampleValues))
	}
	if !attr.Capped {
		t.Error("expected capped to be true")
	}
	// UniqueCount freezes at MaxSampleValues once capped (we can't track further).
	if attr.UniqueCount != MaxSampleValues {
		t.Errorf("expected unique count %d, got %d", MaxSampleValues, attr.UniqueCount)
	}
}

func TestCatalog_RecordAttributes_MultipleLevels(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("my.metric", MetricTypeGauge, nil, 1)

	c.RecordAttributes("my.metric", AttributeLevelResource, []AttributeKV{
		{Key: "service.name", Value: "api"},
	})
	c.RecordAttributes("my.metric", AttributeLevelScope, []AttributeKV{
		{Key: "otel.scope.name", Value: "mylib"},
	})
	c.RecordAttributes("my.metric", AttributeLevelDatapoint, []AttributeKV{
		{Key: "method", Value: "GET"},
	})

	entries := c.Entries()
	if len(entries[0].Attributes) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(entries[0].Attributes))
	}
	// Should be sorted: resource, scope, datapoint
	if entries[0].Attributes[0].Level != AttributeLevelResource {
		t.Errorf("expected resource first, got %s", entries[0].Attributes[0].Level)
	}
	if entries[0].Attributes[1].Level != AttributeLevelScope {
		t.Errorf("expected scope second, got %s", entries[0].Attributes[1].Level)
	}
	if entries[0].Attributes[2].Level != AttributeLevelDatapoint {
		t.Errorf("expected datapoint third, got %s", entries[0].Attributes[2].Level)
	}
}

func TestCatalog_RecordAttributes_Deduplication(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("my.metric", MetricTypeGauge, nil, 1)

	// Record same value multiple times
	for i := 0; i < 5; i++ {
		c.RecordAttributes("my.metric", AttributeLevelResource, []AttributeKV{
			{Key: "service.name", Value: "api"},
		})
	}

	entries := c.Entries()
	attr := entries[0].Attributes[0]
	if len(attr.SampleValues) != 1 {
		t.Errorf("expected 1 sample (deduplicated), got %d", len(attr.SampleValues))
	}
	if attr.UniqueCount != 1 {
		t.Errorf("expected unique count 1 (deduplicated), got %d", attr.UniqueCount)
	}
	if attr.Capped {
		t.Error("expected capped to be false")
	}
}

func TestCatalog_RecordAttributes_NoopForUnknownMetric(t *testing.T) {
	c := NewCatalog(5 * time.Minute)

	// No metric recorded yet — should be a no-op
	c.RecordAttributes("nonexistent", AttributeLevelResource, []AttributeKV{
		{Key: "k", Value: "v"},
	})

	if c.Len() != 0 {
		t.Error("expected 0 entries after recording attrs for nonexistent metric")
	}
}

func TestCatalog_RecordAttributes_EmptySlice(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("my.metric", MetricTypeGauge, nil, 1)

	// Empty attrs should be a no-op
	c.RecordAttributes("my.metric", AttributeLevelResource, nil)
	c.RecordAttributes("my.metric", AttributeLevelResource, []AttributeKV{})

	entries := c.Entries()
	if len(entries[0].Attributes) != 0 {
		t.Errorf("expected 0 attributes for empty record, got %d", len(entries[0].Attributes))
	}
}

func TestCatalog_RecordAttributes_ClearResetsTrackers(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("my.metric", MetricTypeGauge, nil, 1)
	c.RecordAttributes("my.metric", AttributeLevelResource, []AttributeKV{
		{Key: "service.name", Value: "api"},
	})

	c.Clear()

	if c.Len() != 0 {
		t.Error("expected 0 entries after clear")
	}

	// Re-record and verify clean state
	c.Record("my.metric", MetricTypeGauge, nil, 1)
	entries := c.Entries()
	if len(entries[0].Attributes) != 0 {
		t.Errorf("expected 0 attributes after clear and re-record, got %d", len(entries[0].Attributes))
	}
}

func TestCatalog_RecordAttributes_ConcurrentAccess(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("my.metric", MetricTypeGauge, nil, 1)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.RecordAttributes("my.metric", AttributeLevelDatapoint, []AttributeKV{
				{Key: "id", Value: fmt.Sprintf("v%d", n)},
			})
		}(i)
	}
	wg.Wait()

	entries := c.Entries()
	if len(entries[0].Attributes) != 1 {
		t.Fatalf("expected 1 attribute key, got %d", len(entries[0].Attributes))
	}
	attr := entries[0].Attributes[0]
	if attr.Key != "id" {
		t.Errorf("expected key 'id', got %q", attr.Key)
	}
	// Should have tracked at most MaxSampleValues samples
	if len(attr.SampleValues) > MaxSampleValues {
		t.Errorf("expected at most %d samples, got %d", MaxSampleValues, len(attr.SampleValues))
	}
}

func TestCatalog_RecordAttributes_UniqueCountStableAcrossScrapes(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("system.disk.operations", MetricTypeSum, nil, 186)

	// Simulate a fixed set of 10 attribute combos repeated across many scrapes.
	values := []string{"v0", "v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9"}
	for scrape := 0; scrape < 50; scrape++ {
		for _, v := range values {
			c.RecordAttributes("system.disk.operations", AttributeLevelDatapoint, []AttributeKV{
				{Key: "device", Value: v},
			})
		}
	}

	entries := c.Entries()
	attr := entries[0].Attributes[0]
	if attr.UniqueCount != 10 {
		t.Errorf("expected unique count 10 (stable across scrapes), got %d", attr.UniqueCount)
	}
	if attr.Capped {
		t.Error("expected capped=false with only 10 unique values")
	}
}

func TestCatalog_RecordAttributes_UniqueCountStableWhenCapped(t *testing.T) {
	c := NewCatalog(5 * time.Minute)
	c.Record("high.card.metric", MetricTypeSum, nil, 200)

	// First: push past the cap with 30 unique values.
	for i := 0; i < 30; i++ {
		c.RecordAttributes("high.card.metric", AttributeLevelDatapoint, []AttributeKV{
			{Key: "id", Value: fmt.Sprintf("unique-%d", i)},
		})
	}

	// Then: simulate 100 more scrapes re-sending the same 200 data points.
	// Before the fix, uniqueN would grow by 200 per scrape.
	for scrape := 0; scrape < 100; scrape++ {
		for i := 0; i < 200; i++ {
			c.RecordAttributes("high.card.metric", AttributeLevelDatapoint, []AttributeKV{
				{Key: "id", Value: fmt.Sprintf("dp-%d", i)},
			})
		}
	}

	entries := c.Entries()
	attr := entries[0].Attributes[0]
	if !attr.Capped {
		t.Fatal("expected capped=true")
	}
	// UniqueCount must be exactly MaxSampleValues, not growing with each scrape.
	if attr.UniqueCount != MaxSampleValues {
		t.Errorf("expected unique count %d (frozen at cap), got %d", MaxSampleValues, attr.UniqueCount)
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
