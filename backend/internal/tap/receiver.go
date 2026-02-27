package tap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

// ReceiverConfig holds the addresses for the OTLP receiver.
type ReceiverConfig struct {
	GRPCAddr string
	HTTPAddr string
}

// metricHandler implements pmetricotlp.GRPCServer.
type metricHandler struct {
	pmetricotlp.UnimplementedGRPCServer
	catalog *Catalog
}

func (h *metricHandler) Export(_ context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	extractAndRecord(req.Metrics(), h.catalog)
	return pmetricotlp.NewExportResponse(), nil
}

// logHandler implements plogotlp.GRPCServer.
type logHandler struct {
	plogotlp.UnimplementedGRPCServer
	catalog *LogCatalog
}

func (h *logHandler) Export(_ context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	extractAndRecordLogs(req.Logs(), h.catalog)
	return plogotlp.NewExportResponse(), nil
}

// traceHandler implements ptraceotlp.GRPCServer.
type traceHandler struct {
	ptraceotlp.UnimplementedGRPCServer
	catalog *SpanCatalog
}

func (h *traceHandler) Export(_ context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	extractAndRecordSpans(req.Traces(), h.catalog)
	return ptraceotlp.NewExportResponse(), nil
}

// Receiver is an OTLP gRPC + HTTP receiver that extracts metric, log, and trace
// metadata and records it into the corresponding catalogs.
type Receiver struct {
	catalog     *Catalog
	spanCatalog *SpanCatalog
	logCatalog  *LogCatalog
	grpcServer  *grpc.Server
	httpServer  *http.Server
	grpcLis     net.Listener
	httpLis     net.Listener
}

// NewReceiver creates a new Receiver that records into the given catalogs.
func NewReceiver(cfg ReceiverConfig, catalog *Catalog, spanCatalog *SpanCatalog, logCatalog *LogCatalog) (*Receiver, error) {
	grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return nil, fmt.Errorf("grpc listen: %w", err)
	}

	httpLis, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		grpcLis.Close()
		return nil, fmt.Errorf("http listen: %w", err)
	}

	r := &Receiver{
		catalog:     catalog,
		spanCatalog: spanCatalog,
		logCatalog:  logCatalog,
		grpcLis:     grpcLis,
		httpLis:     httpLis,
	}

	// gRPC server — register signal-specific handlers
	r.grpcServer = grpc.NewServer()
	pmetricotlp.RegisterGRPCServer(r.grpcServer, &metricHandler{catalog: catalog})
	plogotlp.RegisterGRPCServer(r.grpcServer, &logHandler{catalog: logCatalog})
	ptraceotlp.RegisterGRPCServer(r.grpcServer, &traceHandler{catalog: spanCatalog})

	// HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/metrics", r.handleHTTPMetrics)
	mux.HandleFunc("POST /v1/logs", r.handleHTTPLogs)
	mux.HandleFunc("POST /v1/traces", r.handleHTTPTraces)
	r.httpServer = &http.Server{Handler: mux}

	return r, nil
}

// Start begins serving gRPC and HTTP in background goroutines.
func (r *Receiver) Start() {
	go r.grpcServer.Serve(r.grpcLis)
	go r.httpServer.Serve(r.httpLis)
}

// Stop gracefully stops both servers.
func (r *Receiver) Stop() {
	r.grpcServer.GracefulStop()
	r.httpServer.Close()
}

// GRPCAddr returns the actual address the gRPC server is listening on.
func (r *Receiver) GRPCAddr() string {
	return r.grpcLis.Addr().String()
}

// HTTPAddr returns the actual address the HTTP server is listening on.
func (r *Receiver) HTTPAddr() string {
	return r.httpLis.Addr().String()
}

// --- HTTP handlers ---

func (r *Receiver) handleHTTPMetrics(w http.ResponseWriter, req *http.Request) {
	body, err := readLimitedBody(req)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var metrics pmetric.Metrics
	ct := req.Header.Get("Content-Type")

	switch ct {
	case "application/x-protobuf", "application/protobuf":
		unmarshaler := &pmetric.ProtoUnmarshaler{}
		metrics, err = unmarshaler.UnmarshalMetrics(body)
	default:
		unmarshaler := &pmetric.JSONUnmarshaler{}
		metrics, err = unmarshaler.UnmarshalMetrics(body)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal: %v", err), http.StatusBadRequest)
		return
	}

	extractAndRecord(metrics, r.catalog)
	writeOTLPOK(w)
}

func (r *Receiver) handleHTTPLogs(w http.ResponseWriter, req *http.Request) {
	body, err := readLimitedBody(req)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var logs plog.Logs
	ct := req.Header.Get("Content-Type")

	switch ct {
	case "application/x-protobuf", "application/protobuf":
		unmarshaler := &plog.ProtoUnmarshaler{}
		logs, err = unmarshaler.UnmarshalLogs(body)
	default:
		unmarshaler := &plog.JSONUnmarshaler{}
		logs, err = unmarshaler.UnmarshalLogs(body)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal: %v", err), http.StatusBadRequest)
		return
	}

	extractAndRecordLogs(logs, r.logCatalog)
	writeOTLPOK(w)
}

func (r *Receiver) handleHTTPTraces(w http.ResponseWriter, req *http.Request) {
	body, err := readLimitedBody(req)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var traces ptrace.Traces
	ct := req.Header.Get("Content-Type")

	switch ct {
	case "application/x-protobuf", "application/protobuf":
		unmarshaler := &ptrace.ProtoUnmarshaler{}
		traces, err = unmarshaler.UnmarshalTraces(body)
	default:
		unmarshaler := &ptrace.JSONUnmarshaler{}
		traces, err = unmarshaler.UnmarshalTraces(body)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal: %v", err), http.StatusBadRequest)
		return
	}

	extractAndRecordSpans(traces, r.spanCatalog)
	writeOTLPOK(w)
}

// --- Helpers ---

const maxBodySize = 10 * 1024 * 1024 // 10MB

func readLimitedBody(req *http.Request) ([]byte, error) {
	return io.ReadAll(io.LimitReader(req.Body, maxBodySize))
}

func writeOTLPOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{})
}

// --- Metric extraction ---

func extractAndRecord(metrics pmetric.Metrics, catalog *Catalog) {
	var totalPoints int64
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		resourceKVs := pcommonMapKVs(rm.Resource().Attributes())
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeKVs := pcommonMapKVs(sm.Scope().Attributes())
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				name := m.Name()
				typ, attrKeys, pointCount := extractMetricMeta(m)
				catalog.Record(name, typ, attrKeys, int64(pointCount))
				totalPoints += int64(pointCount)

				catalog.RecordAttributes(name, AttributeLevelResource, resourceKVs)
				catalog.RecordAttributes(name, AttributeLevelScope, scopeKVs)

				dpAttrSets := extractDatapointAttrs(m)
				for _, dpKVs := range dpAttrSets {
					catalog.RecordAttributes(name, AttributeLevelDatapoint, dpKVs)
				}
			}
		}
	}
	if totalPoints > 0 {
		catalog.RecordBatch(totalPoints)
	}
}

// --- Log extraction ---

func extractAndRecordLogs(logs plog.Logs, catalog *LogCatalog) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		resourceKVs := pcommonMapKVs(rl.Resource().Attributes())
		serviceName := resourceServiceName(rl.Resource().Attributes())
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scopeName := sl.Scope().Name()
			if scopeName == "" {
				scopeName = "unscoped"
			}
			scopeKVs := pcommonMapKVs(sl.Scope().Attributes())
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				severity := SeverityRangeFromNumber(int32(lr.SeverityNumber()))
				eventName := logRecordEventName(lr)
				logKVs := pcommonMapKVs(lr.Attributes())

				catalog.Record(serviceName, scopeName, eventName, severity, 1)
				catalog.RecordAttributes(serviceName, scopeName, eventName, AttributeLevelResource, resourceKVs)
				catalog.RecordAttributes(serviceName, scopeName, eventName, AttributeLevelScope, scopeKVs)
				catalog.RecordAttributes(serviceName, scopeName, eventName, AttributeLevelDatapoint, logKVs)
			}
		}
	}
}

// logRecordEventName extracts the event name from a log record.
// It checks the EventName field first (proto field 12), then falls back
// to the "event.name" attribute for older SDKs.
func logRecordEventName(lr plog.LogRecord) string {
	if name := lr.EventName(); name != "" {
		return name
	}
	if v, ok := lr.Attributes().Get("event.name"); ok {
		return v.AsString()
	}
	return ""
}

// --- Trace extraction ---

func extractAndRecordSpans(traces ptrace.Traces, catalog *SpanCatalog) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		resourceKVs := pcommonMapKVs(rs.Resource().Attributes())
		serviceName := resourceServiceName(rs.Resource().Attributes())
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			scopeKVs := pcommonMapKVs(ss.Scope().Attributes())
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				spanName := span.Name()
				kind := mapSpanKind(span.Kind())
				status := mapSpanStatus(span.Status().Code())
				spanKVs := pcommonMapKVs(span.Attributes())

				catalog.Record(serviceName, spanName, kind, status, 1)
				catalog.RecordAttributes(serviceName, spanName, AttributeLevelResource, resourceKVs)
				catalog.RecordAttributes(serviceName, spanName, AttributeLevelScope, scopeKVs)
				catalog.RecordAttributes(serviceName, spanName, AttributeLevelDatapoint, spanKVs)
			}
		}
	}
}

func resourceServiceName(attrs pcommon.Map) string {
	v, ok := attrs.Get("service.name")
	if ok {
		return v.AsString()
	}
	return "unknown"
}

func mapSpanKind(k ptrace.SpanKind) SpanKind {
	switch k {
	case ptrace.SpanKindClient:
		return SpanKindClient
	case ptrace.SpanKindServer:
		return SpanKindServer
	case ptrace.SpanKindInternal:
		return SpanKindInternal
	case ptrace.SpanKindProducer:
		return SpanKindProducer
	case ptrace.SpanKindConsumer:
		return SpanKindConsumer
	default:
		return SpanKindUnset
	}
}

func mapSpanStatus(c ptrace.StatusCode) SpanStatusCode {
	switch c {
	case ptrace.StatusCodeOk:
		return SpanStatusOk
	case ptrace.StatusCodeError:
		return SpanStatusError
	default:
		return SpanStatusUnset
	}
}

// --- Metric helpers ---

func extractMetricMeta(m pmetric.Metric) (MetricType, []string, int) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dp := m.Gauge().DataPoints()
		return MetricTypeGauge, attrKeysFromNumberDP(dp), dp.Len()
	case pmetric.MetricTypeSum:
		dp := m.Sum().DataPoints()
		return MetricTypeSum, attrKeysFromNumberDP(dp), dp.Len()
	case pmetric.MetricTypeHistogram:
		dp := m.Histogram().DataPoints()
		return MetricTypeHistogram, attrKeysFromHistogramDP(dp), dp.Len()
	case pmetric.MetricTypeSummary:
		dp := m.Summary().DataPoints()
		return MetricTypeSummary, attrKeysFromSummaryDP(dp), dp.Len()
	case pmetric.MetricTypeExponentialHistogram:
		dp := m.ExponentialHistogram().DataPoints()
		return MetricTypeExponentialHistogram, attrKeysFromExpHistogramDP(dp), dp.Len()
	default:
		return MetricTypeGauge, nil, 0
	}
}

func attrKeysFromNumberDP(dp pmetric.NumberDataPointSlice) []string {
	if dp.Len() == 0 {
		return nil
	}
	return pcommonMapKeys(dp.At(0).Attributes())
}

func attrKeysFromHistogramDP(dp pmetric.HistogramDataPointSlice) []string {
	if dp.Len() == 0 {
		return nil
	}
	return pcommonMapKeys(dp.At(0).Attributes())
}

func attrKeysFromSummaryDP(dp pmetric.SummaryDataPointSlice) []string {
	if dp.Len() == 0 {
		return nil
	}
	return pcommonMapKeys(dp.At(0).Attributes())
}

func attrKeysFromExpHistogramDP(dp pmetric.ExponentialHistogramDataPointSlice) []string {
	if dp.Len() == 0 {
		return nil
	}
	return pcommonMapKeys(dp.At(0).Attributes())
}

func pcommonMapKeys(attrs pcommon.Map) []string {
	keys := make([]string, 0, attrs.Len())
	attrs.Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)
	return keys
}

func pcommonMapKVs(attrs pcommon.Map) []AttributeKV {
	if attrs.Len() == 0 {
		return nil
	}
	kvs := make([]AttributeKV, 0, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		kvs = append(kvs, AttributeKV{Key: k, Value: v.AsString()})
		return true
	})
	return kvs
}

func extractDatapointAttrs(m pmetric.Metric) [][]AttributeKV {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return numberDPAttrs(m.Gauge().DataPoints())
	case pmetric.MetricTypeSum:
		return numberDPAttrs(m.Sum().DataPoints())
	case pmetric.MetricTypeHistogram:
		dp := m.Histogram().DataPoints()
		result := make([][]AttributeKV, 0, dp.Len())
		for i := 0; i < dp.Len(); i++ {
			result = append(result, pcommonMapKVs(dp.At(i).Attributes()))
		}
		return result
	case pmetric.MetricTypeSummary:
		dp := m.Summary().DataPoints()
		result := make([][]AttributeKV, 0, dp.Len())
		for i := 0; i < dp.Len(); i++ {
			result = append(result, pcommonMapKVs(dp.At(i).Attributes()))
		}
		return result
	case pmetric.MetricTypeExponentialHistogram:
		dp := m.ExponentialHistogram().DataPoints()
		result := make([][]AttributeKV, 0, dp.Len())
		for i := 0; i < dp.Len(); i++ {
			result = append(result, pcommonMapKVs(dp.At(i).Attributes()))
		}
		return result
	default:
		return nil
	}
}

func numberDPAttrs(dp pmetric.NumberDataPointSlice) [][]AttributeKV {
	result := make([][]AttributeKV, 0, dp.Len())
	for i := 0; i < dp.Len(); i++ {
		result = append(result, pcommonMapKVs(dp.At(i).Attributes()))
	}
	return result
}
