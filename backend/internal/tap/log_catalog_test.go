package tap

import (
	"testing"
	"time"
)

func TestLogCatalog_RecordAndRetrieve(t *testing.T) {
	c := NewLogCatalog(5 * time.Minute)

	c.Record("my-service", "com.example.App", "", SeverityInfo, 5)
	c.Record("my-service", "com.example.App", "", SeverityError, 2)

	if c.Len() != 1 {
		t.Fatalf("expected 1 entry (same key), got %d", c.Len())
	}

	entries := c.Entries()
	if entries[0].ServiceName != "my-service" {
		t.Errorf("expected my-service, got %s", entries[0].ServiceName)
	}
	if entries[0].ScopeName != "com.example.App" {
		t.Errorf("expected com.example.App, got %s", entries[0].ScopeName)
	}
	if entries[0].LogKind != LogKindLog {
		t.Errorf("expected log kind, got %s", entries[0].LogKind)
	}
	if entries[0].RecordCount != 7 {
		t.Errorf("expected 7 total records, got %d", entries[0].RecordCount)
	}

	// Check severity distribution
	sevMap := make(map[SeverityRange]int64)
	for _, sc := range entries[0].SeverityCounts {
		sevMap[sc.Severity] = sc.Count
	}
	if sevMap[SeverityInfo] != 5 {
		t.Errorf("expected 5 info records, got %d", sevMap[SeverityInfo])
	}
	if sevMap[SeverityError] != 2 {
		t.Errorf("expected 2 error records, got %d", sevMap[SeverityError])
	}
}

func TestLogCatalog_Accumulation(t *testing.T) {
	c := NewLogCatalog(5 * time.Minute)

	c.Record("svc", "logger", "", SeverityInfo, 3)
	c.Record("svc", "logger", "", SeverityInfo, 7)

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].RecordCount != 10 {
		t.Errorf("expected 10 accumulated records, got %d", entries[0].RecordCount)
	}
	if entries[0].ScrapeCount != 2 {
		t.Errorf("expected 2 scrapes, got %d", entries[0].ScrapeCount)
	}
}

func TestLogCatalog_EventName(t *testing.T) {
	c := NewLogCatalog(5 * time.Minute)

	c.Record("svc", "otel.instrumentation", "user.login", SeverityInfo, 3)
	c.Record("svc", "otel.instrumentation", "", SeverityInfo, 5)

	if c.Len() != 2 {
		t.Fatalf("expected 2 entries (event vs log), got %d", c.Len())
	}

	entries := c.Entries()
	// Both have same service+scope, sorted by event name ("" < "user.login")
	if entries[0].EventName != "" {
		t.Errorf("expected plain log first, got event %q", entries[0].EventName)
	}
	if entries[0].LogKind != LogKindLog {
		t.Errorf("expected log kind, got %s", entries[0].LogKind)
	}
	if entries[1].EventName != "user.login" {
		t.Errorf("expected user.login event, got %q", entries[1].EventName)
	}
	if entries[1].LogKind != LogKindEvent {
		t.Errorf("expected event kind, got %s", entries[1].LogKind)
	}
}

func TestLogCatalog_SeverityDistribution(t *testing.T) {
	c := NewLogCatalog(5 * time.Minute)

	c.Record("svc", "logger", "", SeverityInfo, 100)
	c.Record("svc", "logger", "", SeverityWarn, 20)
	c.Record("svc", "logger", "", SeverityError, 5)
	c.Record("svc", "logger", "", SeverityInfo, 50)

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	sevMap := make(map[SeverityRange]int64)
	for _, sc := range entries[0].SeverityCounts {
		sevMap[sc.Severity] = sc.Count
	}
	if sevMap[SeverityInfo] != 150 {
		t.Errorf("expected 150 info, got %d", sevMap[SeverityInfo])
	}
	if sevMap[SeverityWarn] != 20 {
		t.Errorf("expected 20 warn, got %d", sevMap[SeverityWarn])
	}
	if sevMap[SeverityError] != 5 {
		t.Errorf("expected 5 error, got %d", sevMap[SeverityError])
	}
	if entries[0].RecordCount != 175 {
		t.Errorf("expected 175 total, got %d", entries[0].RecordCount)
	}
}

func TestLogCatalog_Attributes(t *testing.T) {
	c := NewLogCatalog(5 * time.Minute)

	c.Record("svc", "logger", "", SeverityWarn, 1)
	c.RecordAttributes("svc", "logger", "", AttributeLevelResource, []AttributeKV{
		{Key: "service.name", Value: "svc"},
	})
	c.RecordAttributes("svc", "logger", "", AttributeLevelDatapoint, []AttributeKV{
		{Key: "log.source", Value: "stdout"},
	})

	entries := c.Entries()
	if len(entries[0].Attributes) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(entries[0].Attributes))
	}
}

func TestLogCatalog_ClearAndPrune(t *testing.T) {
	c := NewLogCatalog(5 * time.Minute)
	c.Record("svc", "logger", "", SeverityInfo, 1)

	c.Clear()
	if c.Len() != 0 {
		t.Errorf("expected 0 after clear, got %d", c.Len())
	}

	c2 := NewLogCatalog(0)
	c2.Record("svc", "logger", "", SeverityInfo, 1)
	time.Sleep(10 * time.Millisecond)
	c2.Prune()
	if c2.Len() != 0 {
		t.Errorf("expected 0 after prune, got %d", c2.Len())
	}
}

func TestLogCatalog_UnscopedFallback(t *testing.T) {
	c := NewLogCatalog(5 * time.Minute)

	// When scope is "unscoped", it should still work as a valid key
	c.Record("svc", "unscoped", "", SeverityInfo, 1)

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ScopeName != "unscoped" {
		t.Errorf("expected unscoped, got %s", entries[0].ScopeName)
	}
	if entries[0].LogKind != LogKindLog {
		t.Errorf("expected log kind, got %s", entries[0].LogKind)
	}
}

func TestSeverityRangeFromNumber(t *testing.T) {
	tests := []struct {
		num  int32
		want SeverityRange
	}{
		{0, SeverityUnset},
		{1, SeverityTrace},
		{4, SeverityTrace},
		{5, SeverityDebug},
		{8, SeverityDebug},
		{9, SeverityInfo},
		{12, SeverityInfo},
		{13, SeverityWarn},
		{16, SeverityWarn},
		{17, SeverityError},
		{20, SeverityError},
		{21, SeverityFatal},
		{24, SeverityFatal},
	}

	for _, tt := range tests {
		got := SeverityRangeFromNumber(tt.num)
		if got != tt.want {
			t.Errorf("SeverityRangeFromNumber(%d) = %s, want %s", tt.num, got, tt.want)
		}
	}
}
