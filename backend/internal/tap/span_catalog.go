package tap

import (
	"sort"
	"sync"
	"time"
)

// SpanKind represents the kind of a span.
type SpanKind string

const (
	SpanKindClient   SpanKind = "client"
	SpanKindServer   SpanKind = "server"
	SpanKindInternal SpanKind = "internal"
	SpanKindProducer SpanKind = "producer"
	SpanKindConsumer SpanKind = "consumer"
	SpanKindUnset    SpanKind = "unset"
)

// SpanStatusCode represents the status of a span.
type SpanStatusCode string

const (
	SpanStatusUnset SpanStatusCode = "unset"
	SpanStatusOk    SpanStatusCode = "ok"
	SpanStatusError SpanStatusCode = "error"
)

// SpanEntry holds metadata about a discovered trace operation.
type SpanEntry struct {
	ServiceName string          `json:"serviceName"`
	SpanName    string          `json:"spanName"`
	SpanKind    SpanKind        `json:"spanKind"`
	StatusCode  SpanStatusCode  `json:"statusCode"`
	Attributes  []AttributeMeta `json:"attributes,omitempty"`
	SpanCount   int64           `json:"spanCount"`
	ScrapeCount int64           `json:"scrapeCount"`
	LastSeenAt  time.Time       `json:"lastSeenAt"`
	FirstSeenAt time.Time       `json:"firstSeenAt"`
}

// spanEntryInternal is the internal representation with mutable tracker state.
type spanEntryInternal struct {
	SpanEntry
	attrTrackers map[string]*attrTracker
}

// SpanCatalog is an in-memory span catalog with TTL expiry.
type SpanCatalog struct {
	mu      sync.RWMutex
	entries map[string]*spanEntryInternal // keyed by "serviceName\x00spanName"
	ttl     time.Duration
}

// NewSpanCatalog creates a new SpanCatalog with the given TTL.
func NewSpanCatalog(ttl time.Duration) *SpanCatalog {
	return &SpanCatalog{
		entries: make(map[string]*spanEntryInternal),
		ttl:     ttl,
	}
}

func spanKey(serviceName, spanName string) string {
	return serviceName + "\x00" + spanName
}

// Record adds or updates a span entry.
func (c *SpanCatalog) Record(serviceName, spanName string, kind SpanKind, status SpanStatusCode, count int64) {
	now := time.Now()
	key := spanKey(serviceName, spanName)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.entries[key] = &spanEntryInternal{
			SpanEntry: SpanEntry{
				ServiceName: serviceName,
				SpanName:    spanName,
				SpanKind:    kind,
				StatusCode:  status,
				SpanCount:   count,
				ScrapeCount: 1,
				LastSeenAt:  now,
				FirstSeenAt: now,
			},
			attrTrackers: make(map[string]*attrTracker),
		}
	} else {
		entry.SpanCount += count
		entry.ScrapeCount++
		entry.LastSeenAt = now
		// Update status to error if any span reports error
		if status == SpanStatusError {
			entry.StatusCode = SpanStatusError
		}
	}
}

// RecordAttributes merges attribute key-value pairs into the bounded trackers.
func (c *SpanCatalog) RecordAttributes(serviceName, spanName string, level AttributeLevel, attrs []AttributeKV) {
	if len(attrs) == 0 {
		return
	}
	key := spanKey(serviceName, spanName)

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

// Entries returns a copy of all entries sorted by service name then span name.
func (c *SpanCatalog) Entries() []SpanEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]SpanEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entry := e.SpanEntry
		entry.Attributes = buildAttributeMetas(e.attrTrackers)
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ServiceName != result[j].ServiceName {
			return result[i].ServiceName < result[j].ServiceName
		}
		return result[i].SpanName < result[j].SpanName
	})
	return result
}

// Len returns the number of entries.
func (c *SpanCatalog) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clear removes all entries.
func (c *SpanCatalog) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*spanEntryInternal)
}

// Prune removes entries that haven't been seen since the TTL expired.
func (c *SpanCatalog) Prune() {
	cutoff := time.Now().Add(-c.ttl)
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if entry.LastSeenAt.Before(cutoff) {
			delete(c.entries, key)
		}
	}
}
