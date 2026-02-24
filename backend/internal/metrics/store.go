package metrics

import (
	"sync"
	"time"
)

const defaultWindowSize = 6

// Store holds a sliding window of metric snapshots.
type Store struct {
	mu         sync.RWMutex
	snapshots  []*Snapshot
	windowSize int
}

// NewStore creates a store with the default window size.
func NewStore() *Store {
	return &Store{windowSize: defaultWindowSize}
}

// Push adds a snapshot and evicts the oldest if the window is full.
func (s *Store) Push(snap *Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots = append(s.snapshots, snap)
	if len(s.snapshots) > s.windowSize {
		s.snapshots = s.snapshots[len(s.snapshots)-s.windowSize:]
	}
}

// Latest returns the most recent snapshot, or nil if empty.
func (s *Store) Latest() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.snapshots) == 0 {
		return nil
	}
	return s.snapshots[len(s.snapshots)-1]
}

// Previous returns the second-most-recent snapshot, or nil.
func (s *Store) Previous() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.snapshots) < 2 {
		return nil
	}
	return s.snapshots[len(s.snapshots)-2]
}

// Len returns the number of stored snapshots.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.snapshots)
}

// Window returns all snapshots in the window (oldest first).
func (s *Store) Window() []*Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*Snapshot, len(s.snapshots))
	copy(out, s.snapshots)
	return out
}

// Clear removes all snapshots.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = nil
}

// lookupValue finds a metric value in a snapshot by name and label match.
func lookupValue(snap *Snapshot, name string, labels map[string]string) (float64, bool) {
	for _, sample := range snap.Samples {
		if sample.Name != name {
			continue
		}
		if matchLabels(sample.Labels, labels) {
			return sample.Value, true
		}
	}
	return 0, false
}

// matchLabels returns true if the sample labels contain all the required labels.
func matchLabels(sampleLabels, required map[string]string) bool {
	for k, v := range required {
		if sampleLabels[k] != v {
			return false
		}
	}
	return true
}

// RatePerSecond computes the per-second rate for a counter metric between two snapshots.
// Returns 0 if the counter has reset or insufficient data.
func RatePerSecond(prev, curr *Snapshot, name string, labels map[string]string) float64 {
	if prev == nil || curr == nil {
		return 0
	}

	prevVal, prevOk := lookupValue(prev, name, labels)
	currVal, currOk := lookupValue(curr, name, labels)
	if !prevOk || !currOk {
		return 0
	}

	// Counter reset — skip this interval
	if currVal < prevVal {
		return 0
	}

	elapsed := curr.CollectedAt.Sub(prev.CollectedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}

	return (currVal - prevVal) / elapsed
}

// GaugeValue returns the current gauge value from the latest snapshot.
func GaugeValue(snap *Snapshot, name string, labels map[string]string) (float64, bool) {
	if snap == nil {
		return 0, false
	}
	return lookupValue(snap, name, labels)
}

// ComputedSnapshot holds pre-computed rates and derived metrics for the frontend.
type ComputedSnapshot struct {
	Status      string                       `json:"status"`
	CollectedAt time.Time                    `json:"collectedAt"`
	Signals     map[string]*SignalMetrics    `json:"signals"`
	Exporters   map[string]*ExporterMetrics `json:"exporters"`
	Receivers   map[string]*ReceiverMetrics `json:"receivers"`
}

// SignalMetrics holds aggregate throughput for a signal type.
type SignalMetrics struct {
	ReceiverAcceptedRate float64 `json:"receiverAcceptedRate"`
	ExporterSentRate     float64 `json:"exporterSentRate"`
	ExporterFailedRate   float64 `json:"exporterFailedRate"`
	DropRatePct          float64 `json:"dropRatePct"`
}

// ExporterMetrics holds per-exporter throughput and queue data.
type ExporterMetrics struct {
	QueueSize           float64 `json:"queueSize"`
	QueueCapacity       float64 `json:"queueCapacity"`
	QueueUtilizationPct float64 `json:"queueUtilizationPct"`
	SentSpansRate       float64 `json:"sentSpansRate"`
	SentMetricPtsRate   float64 `json:"sentMetricPointsRate"`
	SentLogRecsRate     float64 `json:"sentLogRecordsRate"`
	FailedSpansRate     float64 `json:"failedSpansRate"`
}

// ReceiverMetrics holds per-receiver throughput.
type ReceiverMetrics struct {
	AcceptedSpansRate      float64 `json:"acceptedSpansRate"`
	AcceptedMetricPtsRate  float64 `json:"acceptedMetricPointsRate"`
	AcceptedLogRecsRate    float64 `json:"acceptedLogRecordsRate"`
}

// Metric name constants (normalized, without _total suffix).
const (
	MetricReceiverAcceptedSpans        = "otelcol_receiver_accepted_spans"
	MetricReceiverAcceptedMetricPoints = "otelcol_receiver_accepted_metric_points"
	MetricReceiverAcceptedLogRecords   = "otelcol_receiver_accepted_log_records"
	MetricExporterSentSpans            = "otelcol_exporter_sent_spans"
	MetricExporterSentMetricPoints     = "otelcol_exporter_sent_metric_points"
	MetricExporterSentLogRecords       = "otelcol_exporter_sent_log_records"
	MetricExporterSendFailedSpans      = "otelcol_exporter_send_failed_spans"
	MetricExporterSendFailedMetricPts  = "otelcol_exporter_send_failed_metric_points"
	MetricExporterSendFailedLogRecs    = "otelcol_exporter_send_failed_log_records"
	MetricExporterQueueSize            = "otelcol_exporter_queue_size"
	MetricExporterQueueCapacity        = "otelcol_exporter_queue_capacity"
)

// ComputeSnapshot calculates rates and derived metrics from the store.
func ComputeSnapshot(store *Store, status string) *ComputedSnapshot {
	curr := store.Latest()
	prev := store.Previous()

	cs := &ComputedSnapshot{
		Status:    status,
		Signals:   make(map[string]*SignalMetrics),
		Exporters: make(map[string]*ExporterMetrics),
		Receivers: make(map[string]*ReceiverMetrics),
	}

	if curr == nil {
		return cs
	}
	cs.CollectedAt = curr.CollectedAt

	// Collect unique receivers and exporters from the latest snapshot
	receivers := uniqueLabelValues(curr, "receiver")
	exporters := uniqueLabelValues(curr, "exporter")

	// Per-receiver rates
	for _, recv := range receivers {
		lbls := map[string]string{"receiver": recv}
		cs.Receivers[recv] = &ReceiverMetrics{
			AcceptedSpansRate:     RatePerSecond(prev, curr, MetricReceiverAcceptedSpans, lbls),
			AcceptedMetricPtsRate: RatePerSecond(prev, curr, MetricReceiverAcceptedMetricPoints, lbls),
			AcceptedLogRecsRate:   RatePerSecond(prev, curr, MetricReceiverAcceptedLogRecords, lbls),
		}
	}

	// Per-exporter rates and queue gauges
	for _, exp := range exporters {
		lbls := map[string]string{"exporter": exp}
		qSize, _ := GaugeValue(curr, MetricExporterQueueSize, lbls)
		qCap, _ := GaugeValue(curr, MetricExporterQueueCapacity, lbls)
		utilPct := 0.0
		if qCap > 0 {
			utilPct = (qSize / qCap) * 100
		}
		cs.Exporters[exp] = &ExporterMetrics{
			QueueSize:           qSize,
			QueueCapacity:       qCap,
			QueueUtilizationPct: utilPct,
			SentSpansRate:       RatePerSecond(prev, curr, MetricExporterSentSpans, lbls),
			SentMetricPtsRate:   RatePerSecond(prev, curr, MetricExporterSentMetricPoints, lbls),
			SentLogRecsRate:     RatePerSecond(prev, curr, MetricExporterSentLogRecords, lbls),
			FailedSpansRate:     RatePerSecond(prev, curr, MetricExporterSendFailedSpans, lbls),
		}
	}

	// Aggregate signal-level metrics
	cs.Signals["traces"] = aggregateSignalMetrics(prev, curr,
		MetricReceiverAcceptedSpans, MetricExporterSentSpans, MetricExporterSendFailedSpans,
		receivers, exporters)
	cs.Signals["metrics"] = aggregateSignalMetrics(prev, curr,
		MetricReceiverAcceptedMetricPoints, MetricExporterSentMetricPoints, MetricExporterSendFailedMetricPts,
		receivers, exporters)
	cs.Signals["logs"] = aggregateSignalMetrics(prev, curr,
		MetricReceiverAcceptedLogRecords, MetricExporterSentLogRecords, MetricExporterSendFailedLogRecs,
		receivers, exporters)

	return cs
}

func aggregateSignalMetrics(prev, curr *Snapshot, acceptedMetric, sentMetric, failedMetric string, receivers, exporters []string) *SignalMetrics {
	sm := &SignalMetrics{}

	for _, recv := range receivers {
		lbls := map[string]string{"receiver": recv}
		sm.ReceiverAcceptedRate += RatePerSecond(prev, curr, acceptedMetric, lbls)
	}
	for _, exp := range exporters {
		lbls := map[string]string{"exporter": exp}
		sm.ExporterSentRate += RatePerSecond(prev, curr, sentMetric, lbls)
		sm.ExporterFailedRate += RatePerSecond(prev, curr, failedMetric, lbls)
	}

	if sm.ReceiverAcceptedRate > 0 {
		dropped := sm.ReceiverAcceptedRate - sm.ExporterSentRate
		if dropped < 0 {
			dropped = 0
		}
		sm.DropRatePct = (dropped / sm.ReceiverAcceptedRate) * 100
	}

	return sm
}

func uniqueLabelValues(snap *Snapshot, labelName string) []string {
	seen := map[string]struct{}{}
	var values []string
	for _, s := range snap.Samples {
		if v, ok := s.Labels[labelName]; ok {
			if _, exists := seen[v]; !exists {
				seen[v] = struct{}{}
				values = append(values, v)
			}
		}
	}
	return values
}
