package tap

import (
	"sort"
	"sync"
	"time"
)

// SeverityRange groups OTLP severity numbers into coarse buckets.
type SeverityRange string

const (
	SeverityTrace SeverityRange = "trace"
	SeverityDebug SeverityRange = "debug"
	SeverityInfo  SeverityRange = "info"
	SeverityWarn  SeverityRange = "warn"
	SeverityError SeverityRange = "error"
	SeverityFatal SeverityRange = "fatal"
	SeverityUnset SeverityRange = "unset"
)

// SeverityRangeFromNumber maps an OTLP severity number (1-24) to a SeverityRange.
func SeverityRangeFromNumber(n int32) SeverityRange {
	switch {
	case n == 0:
		return SeverityUnset
	case n <= 4:
		return SeverityTrace
	case n <= 8:
		return SeverityDebug
	case n <= 12:
		return SeverityInfo
	case n <= 16:
		return SeverityWarn
	case n <= 20:
		return SeverityError
	default:
		return SeverityFatal
	}
}

// LogKind classifies a log catalog entry based on how the composite key resolved.
type LogKind string

const (
	LogKindEvent LogKind = "event" // event.name was present
	LogKindLog   LogKind = "log"   // plain log record
)

// SeverityCount holds a severity bucket and its record count.
type SeverityCount struct {
	Severity SeverityRange `json:"severity"`
	Count    int64         `json:"count"`
}

// LogEntry holds metadata about a discovered log source.
// Keyed by (ServiceName, ScopeName, EventName) per ADR-0014.
type LogEntry struct {
	ServiceName    string          `json:"serviceName"`
	ScopeName      string          `json:"scopeName"`
	EventName      string          `json:"eventName,omitempty"`
	LogKind        LogKind         `json:"logKind"`
	SeverityCounts []SeverityCount `json:"severityCounts"`
	Attributes     []AttributeMeta `json:"attributes,omitempty"`
	RecordCount    int64           `json:"recordCount"`
	ScrapeCount    int64           `json:"scrapeCount"`
	LastSeenAt     time.Time       `json:"lastSeenAt"`
	FirstSeenAt    time.Time       `json:"firstSeenAt"`
}

// logEntryInternal is the internal representation with mutable tracker state.
type logEntryInternal struct {
	serviceName    string
	scopeName      string
	eventName      string
	logKind        LogKind
	severityCounts map[SeverityRange]int64
	recordCount    int64
	scrapeCount    int64
	lastSeenAt     time.Time
	firstSeenAt    time.Time
	attrTrackers   map[string]*attrTracker
}

// LogCatalog is an in-memory log catalog with TTL expiry.
// Keyed by (serviceName, scopeName, eventName) per ADR-0014.
type LogCatalog struct {
	mu      sync.RWMutex
	entries map[string]*logEntryInternal
	ttl     time.Duration
}

// NewLogCatalog creates a new LogCatalog with the given TTL.
func NewLogCatalog(ttl time.Duration) *LogCatalog {
	return &LogCatalog{
		entries: make(map[string]*logEntryInternal),
		ttl:     ttl,
	}
}

func logKey(serviceName, scopeName, eventName string) string {
	return serviceName + "\x00" + scopeName + "\x00" + eventName
}

// Record adds or updates a log entry with the given severity.
func (c *LogCatalog) Record(serviceName, scopeName, eventName string, severity SeverityRange, count int64) {
	now := time.Now()
	key := logKey(serviceName, scopeName, eventName)

	kind := LogKindLog
	if eventName != "" {
		kind = LogKindEvent
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.entries[key] = &logEntryInternal{
			serviceName:    serviceName,
			scopeName:      scopeName,
			eventName:      eventName,
			logKind:        kind,
			severityCounts: map[SeverityRange]int64{severity: count},
			recordCount:    count,
			scrapeCount:    1,
			lastSeenAt:     now,
			firstSeenAt:    now,
			attrTrackers:   make(map[string]*attrTracker),
		}
	} else {
		entry.severityCounts[severity] += count
		entry.recordCount += count
		entry.scrapeCount++
		entry.lastSeenAt = now
	}
}

// RecordAttributes merges attribute key-value pairs into the bounded trackers.
func (c *LogCatalog) RecordAttributes(serviceName, scopeName, eventName string, level AttributeLevel, attrs []AttributeKV) {
	if len(attrs) == 0 {
		return
	}
	key := logKey(serviceName, scopeName, eventName)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return
	}
	if entry.attrTrackers == nil {
		entry.attrTrackers = make(map[string]*attrTracker)
	}
	for _, kv := range attrs {
		tKey := string(level) + ":" + kv.Key
		tracker, ok := entry.attrTrackers[tKey]
		if !ok {
			tracker = newAttrTracker()
			entry.attrTrackers[tKey] = tracker
		}
		tracker.record(kv.Value)
	}
}

// Entries returns a copy of all entries sorted by service name then scope name.
func (c *LogCatalog) Entries() []LogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]LogEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entry := LogEntry{
			ServiceName: e.serviceName,
			ScopeName:   e.scopeName,
			EventName:   e.eventName,
			LogKind:     e.logKind,
			RecordCount: e.recordCount,
			ScrapeCount: e.scrapeCount,
			LastSeenAt:  e.lastSeenAt,
			FirstSeenAt: e.firstSeenAt,
			Attributes:  buildAttributeMetas(e.attrTrackers),
		}
		entry.SeverityCounts = buildSeverityCounts(e.severityCounts)
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ServiceName != result[j].ServiceName {
			return result[i].ServiceName < result[j].ServiceName
		}
		if result[i].ScopeName != result[j].ScopeName {
			return result[i].ScopeName < result[j].ScopeName
		}
		return result[i].EventName < result[j].EventName
	})
	return result
}

// buildSeverityCounts converts the internal map to a sorted slice.
func buildSeverityCounts(m map[SeverityRange]int64) []SeverityCount {
	if len(m) == 0 {
		return nil
	}
	counts := make([]SeverityCount, 0, len(m))
	for sev, cnt := range m {
		counts = append(counts, SeverityCount{Severity: sev, Count: cnt})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].Severity < counts[j].Severity
	})
	return counts
}

// Len returns the number of entries.
func (c *LogCatalog) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clear removes all entries.
func (c *LogCatalog) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*logEntryInternal)
}

// Prune removes entries that haven't been seen since the TTL expired.
func (c *LogCatalog) Prune() {
	cutoff := time.Now().Add(-c.ttl)
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if entry.lastSeenAt.Before(cutoff) {
			delete(c.entries, key)
		}
	}
}
