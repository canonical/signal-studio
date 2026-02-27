package tap

import (
	"sort"
	"sync"
	"time"
)

// AttributeLevel identifies where an attribute was observed.
type AttributeLevel string

const (
	AttributeLevelResource  AttributeLevel = "resource"
	AttributeLevelScope     AttributeLevel = "scope"
	AttributeLevelDatapoint AttributeLevel = "datapoint"
)

// MaxSampleValues is the maximum number of distinct sample values tracked per attribute key.
const MaxSampleValues = 25

// AttributeMeta describes a single attribute key observed on a metric.
type AttributeMeta struct {
	Key          string         `json:"key"`
	Level        AttributeLevel `json:"level"`
	SampleValues []string       `json:"sampleValues"`
	UniqueCount  int64          `json:"uniqueCount"`
	Capped       bool           `json:"capped"`
}

// AttributeKV is a key-value pair extracted from OTLP attributes.
type AttributeKV struct {
	Key   string
	Value string
}

// attrTracker tracks bounded sample values for a single attribute key at a specific level.
type attrTracker struct {
	seen    map[string]struct{}
	samples []string
	uniqueN int64
	capped  bool
}

func newAttrTracker() *attrTracker {
	return &attrTracker{
		seen: make(map[string]struct{}),
	}
}

func (t *attrTracker) record(value string) {
	if t.capped {
		return
	}
	if _, exists := t.seen[value]; exists {
		return
	}
	t.seen[value] = struct{}{}
	t.samples = append(t.samples, value)
	t.uniqueN = int64(len(t.samples))
	if len(t.samples) >= MaxSampleValues {
		t.capped = true
		t.seen = nil // free memory
	}
}

func (t *attrTracker) toMeta(key string, level AttributeLevel) AttributeMeta {
	samples := make([]string, len(t.samples))
	copy(samples, t.samples)
	return AttributeMeta{
		Key:          key,
		Level:        level,
		SampleValues: samples,
		UniqueCount:  t.uniqueN,
		Capped:       t.capped,
	}
}

// MetricType represents an OpenTelemetry metric type.
type MetricType string

const (
	MetricTypeGauge                MetricType = "gauge"
	MetricTypeSum                  MetricType = "sum"
	MetricTypeHistogram            MetricType = "histogram"
	MetricTypeSummary              MetricType = "summary"
	MetricTypeExponentialHistogram MetricType = "exponential_histogram"
)

// MetricEntry holds metadata about a single metric name.
type MetricEntry struct {
	Name          string          `json:"name"`
	Type          MetricType      `json:"type"`
	AttributeKeys []string       `json:"attributeKeys"`
	Attributes    []AttributeMeta `json:"attributes,omitempty"`
	PointCount    int64           `json:"pointCount"`
	ScrapeCount   int64           `json:"scrapeCount"`
	LastSeenAt    time.Time       `json:"lastSeenAt"`
	FirstSeenAt   time.Time       `json:"firstSeenAt"`
}

// metricEntryInternal is the internal representation that includes mutable tracker state.
type metricEntryInternal struct {
	MetricEntry
	attrTrackers map[string]*attrTracker // keyed by "level:key"
}

// rateWindow tracks total points and batches received during a time window.
type rateWindow struct {
	points    int64
	batches   int64
	startedAt time.Time
}

// Catalog is an in-memory metric name catalog with TTL expiry.
type Catalog struct {
	mu      sync.RWMutex
	entries map[string]*metricEntryInternal
	ttl     time.Duration

	// Rate change detection: two consecutive windows of point ingestion.
	prevWindow *rateWindow
	currWindow *rateWindow
	windowDur  time.Duration
}

const defaultWindowDur = 2 * time.Minute

// NewCatalog creates a new Catalog with the given TTL.
func NewCatalog(ttl time.Duration) *Catalog {
	return &Catalog{
		entries:   make(map[string]*metricEntryInternal),
		ttl:       ttl,
		windowDur: defaultWindowDur,
	}
}

// Record adds or updates a metric entry. Attribute keys are merged (union)
// and point counts are accumulated on existing entries.
func (c *Catalog) Record(name string, typ MetricType, attrKeys []string, pointCount int64) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[name]
	if !exists {
		keys := make([]string, len(attrKeys))
		copy(keys, attrKeys)
		sort.Strings(keys)
		c.entries[name] = &metricEntryInternal{
			MetricEntry: MetricEntry{
				Name:          name,
				Type:          typ,
				AttributeKeys: keys,
				PointCount:    pointCount,
				ScrapeCount:   1,
				LastSeenAt:    now,
				FirstSeenAt:   now,
			},
			attrTrackers: make(map[string]*attrTracker),
		}
	} else {
		entry.PointCount += pointCount
		entry.ScrapeCount++
		entry.LastSeenAt = now
		entry.AttributeKeys = mergeKeys(entry.AttributeKeys, attrKeys)
	}
}

// RecordAttributes merges attribute key-value pairs into the bounded trackers
// for the given metric name and attribute level.
func (c *Catalog) RecordAttributes(name string, level AttributeLevel, attrs []AttributeKV) {
	if len(attrs) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[name]
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

// RecordBatch records that an OTLP export batch with the given total point
// count was received. This drives the rate-change detection by comparing
// average points-per-batch across consecutive time windows.
func (c *Catalog) RecordBatch(totalPoints int64) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currWindow == nil {
		c.currWindow = &rateWindow{points: totalPoints, batches: 1, startedAt: now}
		return
	}

	// If the current window has elapsed, rotate.
	if now.Sub(c.currWindow.startedAt) >= c.windowDur {
		c.prevWindow = c.currWindow
		c.currWindow = &rateWindow{points: totalPoints, batches: 1, startedAt: now}
		return
	}

	c.currWindow.points += totalPoints
	c.currWindow.batches++
}

// Entries returns a copy of all entries sorted by name.
// Attribute trackers are converted to AttributeMeta slices sorted by level then key.
func (c *Catalog) Entries() []MetricEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]MetricEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entry := e.MetricEntry
		entry.Attributes = buildAttributeMetas(e.attrTrackers)
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// buildAttributeMetas converts the internal tracker map into a sorted slice.
func buildAttributeMetas(trackers map[string]*attrTracker) []AttributeMeta {
	if len(trackers) == 0 {
		return nil
	}
	metas := make([]AttributeMeta, 0, len(trackers))
	for compositeKey, tracker := range trackers {
		level, key := splitTrackerKey(compositeKey)
		metas = append(metas, tracker.toMeta(key, level))
	}
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].Level != metas[j].Level {
			return levelOrder(metas[i].Level) < levelOrder(metas[j].Level)
		}
		return metas[i].Key < metas[j].Key
	})
	return metas
}

func splitTrackerKey(compositeKey string) (AttributeLevel, string) {
	for i, ch := range compositeKey {
		if ch == ':' {
			return AttributeLevel(compositeKey[:i]), compositeKey[i+1:]
		}
	}
	return AttributeLevelDatapoint, compositeKey
}

func levelOrder(l AttributeLevel) int {
	switch l {
	case AttributeLevelResource:
		return 0
	case AttributeLevelScope:
		return 1
	case AttributeLevelDatapoint:
		return 2
	default:
		return 3
	}
}

// Names returns all metric names sorted alphabetically.
func (c *Catalog) Names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.entries))
	for name := range c.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RateChanged returns true if the average points-per-batch has changed
// significantly between the previous and current observation windows (>50% change).
// This detects changes in scrape configuration or the set of collected metrics
// without being affected by timing jitter between window boundaries.
func (c *Catalog) RateChanged() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.prevWindow == nil || c.currWindow == nil {
		return false
	}

	// Need at least 2 batches in each window for a meaningful comparison.
	if c.prevWindow.batches < 2 || c.currWindow.batches < 2 {
		return false
	}

	prevAvg := float64(c.prevWindow.points) / float64(c.prevWindow.batches)
	currAvg := float64(c.currWindow.points) / float64(c.currWindow.batches)

	if prevAvg == 0 {
		return currAvg > 0
	}

	change := (currAvg - prevAvg) / prevAvg
	if change < 0 {
		change = -change
	}
	return change > 0.5
}

// Clear removes all entries and resets rate tracking.
func (c *Catalog) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*metricEntryInternal)
	c.prevWindow = nil
	c.currWindow = nil
}

// Len returns the number of entries.
func (c *Catalog) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Prune removes entries that haven't been seen since the TTL expired.
func (c *Catalog) Prune() {
	cutoff := time.Now().Add(-c.ttl)
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, entry := range c.entries {
		if entry.LastSeenAt.Before(cutoff) {
			delete(c.entries, name)
		}
	}
}

// mergeKeys returns the sorted union of two sorted string slices.
func mergeKeys(existing, incoming []string) []string {
	set := make(map[string]struct{}, len(existing)+len(incoming))
	for _, k := range existing {
		set[k] = struct{}{}
	}
	for _, k := range incoming {
		set[k] = struct{}{}
	}
	merged := make([]string, 0, len(set))
	for k := range set {
		merged = append(merged, k)
	}
	sort.Strings(merged)
	return merged
}
