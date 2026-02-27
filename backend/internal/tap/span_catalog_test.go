package tap

import (
	"testing"
	"time"
)

func TestSpanCatalog_RecordAndRetrieve(t *testing.T) {
	c := NewSpanCatalog(5 * time.Minute)

	c.Record("my-service", "GET /users", SpanKindServer, SpanStatusOk, 10)
	c.Record("my-service", "POST /orders", SpanKindServer, SpanStatusUnset, 5)

	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}

	entries := c.Entries()
	// Sorted by service name then span name
	if entries[0].SpanName != "GET /users" {
		t.Errorf("expected GET /users first, got %s", entries[0].SpanName)
	}
	if entries[0].ServiceName != "my-service" {
		t.Errorf("expected my-service, got %s", entries[0].ServiceName)
	}
	if entries[0].SpanCount != 10 {
		t.Errorf("expected 10 spans, got %d", entries[0].SpanCount)
	}
	if entries[0].SpanKind != SpanKindServer {
		t.Errorf("expected server kind, got %s", entries[0].SpanKind)
	}
}

func TestSpanCatalog_Accumulation(t *testing.T) {
	c := NewSpanCatalog(5 * time.Minute)

	c.Record("svc", "op", SpanKindClient, SpanStatusOk, 3)
	c.Record("svc", "op", SpanKindClient, SpanStatusOk, 7)

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SpanCount != 10 {
		t.Errorf("expected 10 accumulated spans, got %d", entries[0].SpanCount)
	}
	if entries[0].ScrapeCount != 2 {
		t.Errorf("expected 2 scrapes, got %d", entries[0].ScrapeCount)
	}
}

func TestSpanCatalog_ErrorStatusSticky(t *testing.T) {
	c := NewSpanCatalog(5 * time.Minute)

	c.Record("svc", "op", SpanKindServer, SpanStatusOk, 1)
	c.Record("svc", "op", SpanKindServer, SpanStatusError, 1)
	c.Record("svc", "op", SpanKindServer, SpanStatusOk, 1)

	entries := c.Entries()
	if entries[0].StatusCode != SpanStatusError {
		t.Errorf("expected error status to stick, got %s", entries[0].StatusCode)
	}
}

func TestSpanCatalog_Attributes(t *testing.T) {
	c := NewSpanCatalog(5 * time.Minute)

	c.Record("svc", "op", SpanKindServer, SpanStatusOk, 1)
	c.RecordAttributes("svc", "op", AttributeLevelResource, []AttributeKV{
		{Key: "service.name", Value: "svc"},
	})
	c.RecordAttributes("svc", "op", AttributeLevelDatapoint, []AttributeKV{
		{Key: "http.method", Value: "GET"},
		{Key: "http.status_code", Value: "200"},
	})

	entries := c.Entries()
	if len(entries[0].Attributes) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(entries[0].Attributes))
	}
}

func TestSpanCatalog_ClearAndPrune(t *testing.T) {
	c := NewSpanCatalog(5 * time.Minute)
	c.Record("svc", "op", SpanKindServer, SpanStatusOk, 1)

	c.Clear()
	if c.Len() != 0 {
		t.Errorf("expected 0 after clear, got %d", c.Len())
	}

	// Prune test
	c2 := NewSpanCatalog(0) // zero TTL
	c2.Record("svc", "op", SpanKindServer, SpanStatusOk, 1)
	time.Sleep(10 * time.Millisecond)
	c2.Prune()
	if c2.Len() != 0 {
		t.Errorf("expected 0 after prune, got %d", c2.Len())
	}
}

func TestSpanCatalog_MultipleServices(t *testing.T) {
	c := NewSpanCatalog(5 * time.Minute)

	c.Record("auth-service", "login", SpanKindServer, SpanStatusOk, 1)
	c.Record("api-gateway", "route", SpanKindServer, SpanStatusOk, 1)
	c.Record("auth-service", "validate", SpanKindInternal, SpanStatusOk, 1)

	entries := c.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Sorted by service name then span name
	if entries[0].ServiceName != "api-gateway" {
		t.Errorf("expected api-gateway first, got %s", entries[0].ServiceName)
	}
}
