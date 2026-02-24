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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

// ReceiverConfig holds the addresses for the OTLP receiver.
type ReceiverConfig struct {
	GRPCAddr string
	HTTPAddr string
}

// Receiver is an OTLP gRPC + HTTP receiver that extracts metric metadata
// and records it into a Catalog.
type Receiver struct {
	pmetricotlp.UnimplementedGRPCServer
	catalog    *Catalog
	grpcServer *grpc.Server
	httpServer *http.Server
	grpcLis    net.Listener
	httpLis    net.Listener
}

// NewReceiver creates a new Receiver that records into the given catalog.
func NewReceiver(cfg ReceiverConfig, catalog *Catalog) (*Receiver, error) {
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
		catalog: catalog,
		grpcLis: grpcLis,
		httpLis: httpLis,
	}

	// gRPC server
	r.grpcServer = grpc.NewServer()
	pmetricotlp.RegisterGRPCServer(r.grpcServer, r)

	// HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/metrics", r.handleHTTP)
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

// Export implements pmetricotlp.GRPCServer for gRPC OTLP export.
func (r *Receiver) Export(_ context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	extractAndRecord(req.Metrics(), r.catalog)
	return pmetricotlp.NewExportResponse(), nil
}

// handleHTTP handles HTTP OTLP metric exports.
// Accepts application/x-protobuf and application/json.
func (r *Receiver) handleHTTP(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(io.LimitReader(req.Body, 10*1024*1024)) // 10MB limit
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
		// Default to JSON
		unmarshaler := &pmetric.JSONUnmarshaler{}
		metrics, err = unmarshaler.UnmarshalMetrics(body)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal: %v", err), http.StatusBadRequest)
		return
	}

	extractAndRecord(metrics, r.catalog)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{})
}

// extractAndRecord iterates over OTLP metrics and records metadata into the catalog.
func extractAndRecord(metrics pmetric.Metrics, catalog *Catalog) {
	var totalPoints int64
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				name := m.Name()
				typ, attrKeys, pointCount := extractMetricMeta(m)
				catalog.Record(name, typ, attrKeys, int64(pointCount))
				totalPoints += int64(pointCount)
			}
		}
	}
	if totalPoints > 0 {
		catalog.RecordBatch(totalPoints)
	}
}

// extractMetricMeta extracts the metric type, attribute keys (from the first data point),
// and total data point count.
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

// pcommonMapKeys extracts sorted attribute key names from a pcommon.Map.
func pcommonMapKeys(attrs pcommon.Map) []string {
	keys := make([]string, 0, attrs.Len())
	attrs.Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)
	return keys
}
