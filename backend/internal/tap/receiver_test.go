package tap

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func buildTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	// Gauge metric with attributes
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName("system.cpu.utilization")
	m1.SetEmptyGauge()
	dp1 := m1.Gauge().DataPoints().AppendEmpty()
	dp1.SetDoubleValue(0.75)
	dp1.Attributes().PutStr("cpu", "0")
	dp1.Attributes().PutStr("state", "user")

	// Sum metric
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName("http.server.request_count")
	m2.SetEmptySum()
	dp2 := m2.Sum().DataPoints().AppendEmpty()
	dp2.SetIntValue(42)
	dp2a := m2.Sum().DataPoints().AppendEmpty()
	dp2a.SetIntValue(10)

	// Histogram metric
	m3 := sm.Metrics().AppendEmpty()
	m3.SetName("http.server.duration")
	m3.SetEmptyHistogram()
	dp3 := m3.Histogram().DataPoints().AppendEmpty()
	dp3.SetCount(100)
	dp3.Attributes().PutStr("method", "GET")

	return md
}

func TestExtractAndRecord(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	md := buildTestMetrics()

	extractAndRecord(md, catalog)

	if catalog.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", catalog.Len())
	}

	entries := catalog.Entries()

	// Entries are sorted by name
	// 1. http.server.duration (histogram)
	// 2. http.server.request_count (sum)
	// 3. system.cpu.utilization (gauge)

	if entries[0].Name != "http.server.duration" {
		t.Errorf("expected http.server.duration, got %s", entries[0].Name)
	}
	if entries[0].Type != MetricTypeHistogram {
		t.Errorf("expected histogram, got %s", entries[0].Type)
	}
	if entries[0].PointCount != 1 {
		t.Errorf("expected 1 data point, got %d", entries[0].PointCount)
	}
	if len(entries[0].AttributeKeys) != 1 || entries[0].AttributeKeys[0] != "method" {
		t.Errorf("expected [method], got %v", entries[0].AttributeKeys)
	}

	if entries[1].Name != "http.server.request_count" {
		t.Errorf("expected http.server.request_count, got %s", entries[1].Name)
	}
	if entries[1].Type != MetricTypeSum {
		t.Errorf("expected sum, got %s", entries[1].Type)
	}
	if entries[1].PointCount != 2 {
		t.Errorf("expected 2 data points, got %d", entries[1].PointCount)
	}

	if entries[2].Name != "system.cpu.utilization" {
		t.Errorf("expected system.cpu.utilization, got %s", entries[2].Name)
	}
	if entries[2].Type != MetricTypeGauge {
		t.Errorf("expected gauge, got %s", entries[2].Type)
	}
	if len(entries[2].AttributeKeys) != 2 {
		t.Fatalf("expected 2 attribute keys, got %d", len(entries[2].AttributeKeys))
	}
	if entries[2].AttributeKeys[0] != "cpu" || entries[2].AttributeKeys[1] != "state" {
		t.Errorf("expected [cpu, state], got %v", entries[2].AttributeKeys)
	}
}

func TestExtractAndRecord_EmptyMetrics(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	md := pmetric.NewMetrics()

	extractAndRecord(md, catalog)

	if catalog.Len() != 0 {
		t.Errorf("expected 0 entries for empty metrics, got %d", catalog.Len())
	}
}

func TestExtractAndRecord_NoDataPoints(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("empty.gauge")
	m.SetEmptyGauge()

	extractAndRecord(md, catalog)

	if catalog.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", catalog.Len())
	}
	entries := catalog.Entries()
	if entries[0].PointCount != 0 {
		t.Errorf("expected 0 points, got %d", entries[0].PointCount)
	}
	if len(entries[0].AttributeKeys) != 0 {
		t.Errorf("expected 0 attr keys for empty data points, got %d", len(entries[0].AttributeKeys))
	}
}

func TestExtractMetricMeta_AllTypes(t *testing.T) {
	tests := []struct {
		name string
		want MetricType
		setup func(m pmetric.Metric)
	}{
		{"gauge", MetricTypeGauge, func(m pmetric.Metric) {
			m.SetEmptyGauge().DataPoints().AppendEmpty()
		}},
		{"sum", MetricTypeSum, func(m pmetric.Metric) {
			m.SetEmptySum().DataPoints().AppendEmpty()
		}},
		{"histogram", MetricTypeHistogram, func(m pmetric.Metric) {
			m.SetEmptyHistogram().DataPoints().AppendEmpty()
		}},
		{"summary", MetricTypeSummary, func(m pmetric.Metric) {
			m.SetEmptySummary().DataPoints().AppendEmpty()
		}},
		{"exp_histogram", MetricTypeExponentialHistogram, func(m pmetric.Metric) {
			m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := pmetric.NewMetrics()
			m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			m.SetName("test." + tt.name)
			tt.setup(m)

			typ, _, count := extractMetricMeta(m)
			if typ != tt.want {
				t.Errorf("expected type %s, got %s", tt.want, typ)
			}
			if count != 1 {
				t.Errorf("expected 1 data point, got %d", count)
			}
		})
	}
}

func TestPcommonMapKeys(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("z", "1")
	m.PutStr("a", "2")
	m.PutStr("m", "3")

	keys := pcommonMapKeys(m)
	expected := []string{"a", "m", "z"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range expected {
		if keys[i] != k {
			t.Errorf("expected %q at index %d, got %q", k, i, keys[i])
		}
	}
}

func TestExtractAndRecord_ResourceAttributes(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "my-service")
	rm.Resource().Attributes().PutStr("deployment.environment", "production")
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test.gauge")
	m.SetEmptyGauge()
	m.Gauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	extractAndRecord(md, catalog)

	entries := catalog.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Check that resource-level attributes are recorded
	var resourceAttrs []AttributeMeta
	for _, attr := range entries[0].Attributes {
		if attr.Level == AttributeLevelResource {
			resourceAttrs = append(resourceAttrs, attr)
		}
	}
	if len(resourceAttrs) != 2 {
		t.Fatalf("expected 2 resource attributes, got %d", len(resourceAttrs))
	}

	// Attributes are sorted by key within the resource level
	foundDeployment := false
	foundService := false
	for _, attr := range resourceAttrs {
		switch attr.Key {
		case "deployment.environment":
			foundDeployment = true
			if len(attr.SampleValues) != 1 || attr.SampleValues[0] != "production" {
				t.Errorf("expected deployment.environment sample value 'production', got %v", attr.SampleValues)
			}
		case "service.name":
			foundService = true
			if len(attr.SampleValues) != 1 || attr.SampleValues[0] != "my-service" {
				t.Errorf("expected service.name sample value 'my-service', got %v", attr.SampleValues)
			}
		default:
			t.Errorf("unexpected resource attribute key: %s", attr.Key)
		}
	}
	if !foundDeployment {
		t.Error("missing resource attribute: deployment.environment")
	}
	if !foundService {
		t.Error("missing resource attribute: service.name")
	}
}

func TestExtractAndRecord_ScopeAttributes(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().Attributes().PutStr("otel.profiling.version", "0.1.0")
	sm.Scope().SetName("my-scope")
	m := sm.Metrics().AppendEmpty()
	m.SetName("test.sum")
	m.SetEmptySum()
	m.Sum().DataPoints().AppendEmpty().SetIntValue(42)

	extractAndRecord(md, catalog)

	entries := catalog.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	var scopeAttrs []AttributeMeta
	for _, attr := range entries[0].Attributes {
		if attr.Level == AttributeLevelScope {
			scopeAttrs = append(scopeAttrs, attr)
		}
	}
	if len(scopeAttrs) != 1 {
		t.Fatalf("expected 1 scope attribute, got %d", len(scopeAttrs))
	}
	if scopeAttrs[0].Key != "otel.profiling.version" {
		t.Errorf("expected scope attribute key 'otel.profiling.version', got %q", scopeAttrs[0].Key)
	}
	if len(scopeAttrs[0].SampleValues) != 1 || scopeAttrs[0].SampleValues[0] != "0.1.0" {
		t.Errorf("expected scope attribute sample value '0.1.0', got %v", scopeAttrs[0].SampleValues)
	}
}

func TestExtractAndRecord_DatapointAttributes(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("http.server.duration")
	m.SetEmptyHistogram()

	dp1 := m.Histogram().DataPoints().AppendEmpty()
	dp1.SetCount(100)
	dp1.Attributes().PutStr("http.method", "GET")
	dp1.Attributes().PutStr("http.status_code", "200")

	dp2 := m.Histogram().DataPoints().AppendEmpty()
	dp2.SetCount(50)
	dp2.Attributes().PutStr("http.method", "POST")
	dp2.Attributes().PutStr("http.status_code", "201")

	extractAndRecord(md, catalog)

	entries := catalog.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	var dpAttrs []AttributeMeta
	for _, attr := range entries[0].Attributes {
		if attr.Level == AttributeLevelDatapoint {
			dpAttrs = append(dpAttrs, attr)
		}
	}
	if len(dpAttrs) != 2 {
		t.Fatalf("expected 2 datapoint attributes, got %d", len(dpAttrs))
	}

	// Sorted by key: http.method, http.status_code
	foundMethod := false
	foundStatus := false
	for _, attr := range dpAttrs {
		switch attr.Key {
		case "http.method":
			foundMethod = true
			if len(attr.SampleValues) != 2 {
				t.Errorf("expected 2 sample values for http.method, got %d", len(attr.SampleValues))
			}
		case "http.status_code":
			foundStatus = true
			if len(attr.SampleValues) != 2 {
				t.Errorf("expected 2 sample values for http.status_code, got %d", len(attr.SampleValues))
			}
		default:
			t.Errorf("unexpected datapoint attribute key: %s", attr.Key)
		}
	}
	if !foundMethod {
		t.Error("missing datapoint attribute: http.method")
	}
	if !foundStatus {
		t.Error("missing datapoint attribute: http.status_code")
	}
}

func TestExtractDatapointAttrs_AllDataPoints(t *testing.T) {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test.gauge")
	m.SetEmptyGauge()

	// Create 20 data points
	for i := 0; i < 20; i++ {
		dp := m.Gauge().DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i))
		dp.Attributes().PutStr("index", fmt.Sprintf("%d", i))
	}

	result := extractDatapointAttrs(m)
	if len(result) != 20 {
		t.Fatalf("expected 20 data point attr sets, got %d", len(result))
	}

	for i, kvs := range result {
		if len(kvs) != 1 {
			t.Errorf("expected 1 KV at index %d, got %d", i, len(kvs))
			continue
		}
		if kvs[0].Key != "index" {
			t.Errorf("expected key 'index' at index %d, got %q", i, kvs[0].Key)
		}
		expected := fmt.Sprintf("%d", i)
		if kvs[0].Value != expected {
			t.Errorf("expected value %q at index %d, got %q", expected, i, kvs[0].Value)
		}
	}
}

func TestPcommonMapKVs(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("name", "test-service")
	m.PutInt("port", 8080)
	m.PutBool("debug", true)

	kvs := pcommonMapKVs(m)
	if len(kvs) != 3 {
		t.Fatalf("expected 3 KVs, got %d", len(kvs))
	}

	// Build a map for easy lookup since pcommon.Map iteration order is not guaranteed
	kvMap := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		kvMap[kv.Key] = kv.Value
	}

	if v, ok := kvMap["name"]; !ok || v != "test-service" {
		t.Errorf("expected name=test-service, got %q (found=%v)", v, ok)
	}
	if v, ok := kvMap["port"]; !ok || v != "8080" {
		t.Errorf("expected port=8080, got %q (found=%v)", v, ok)
	}
	if v, ok := kvMap["debug"]; !ok || v != "true" {
		t.Errorf("expected debug=true, got %q (found=%v)", v, ok)
	}
}

func TestPcommonMapKVs_Empty(t *testing.T) {
	m := pcommon.NewMap()
	kvs := pcommonMapKVs(m)
	if kvs != nil {
		t.Errorf("expected nil for empty map, got %v", kvs)
	}
}

func TestExtractDatapointAttrs_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m pmetric.Metric)
	}{
		{"gauge", func(m pmetric.Metric) {
			m.SetEmptyGauge()
			dp := m.Gauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(1.0)
			dp.Attributes().PutStr("key", "gauge-val")
		}},
		{"sum", func(m pmetric.Metric) {
			m.SetEmptySum()
			dp := m.Sum().DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			dp.Attributes().PutStr("key", "sum-val")
		}},
		{"histogram", func(m pmetric.Metric) {
			m.SetEmptyHistogram()
			dp := m.Histogram().DataPoints().AppendEmpty()
			dp.SetCount(1)
			dp.Attributes().PutStr("key", "hist-val")
		}},
		{"summary", func(m pmetric.Metric) {
			m.SetEmptySummary()
			dp := m.Summary().DataPoints().AppendEmpty()
			dp.SetCount(1)
			dp.Attributes().PutStr("key", "summary-val")
		}},
		{"exp_histogram", func(m pmetric.Metric) {
			m.SetEmptyExponentialHistogram()
			dp := m.ExponentialHistogram().DataPoints().AppendEmpty()
			dp.SetCount(1)
			dp.Attributes().PutStr("key", "exphist-val")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := pmetric.NewMetrics()
			m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			m.SetName("test." + tt.name)
			tt.setup(m)

			result := extractDatapointAttrs(m)
			if len(result) != 1 {
				t.Fatalf("expected 1 data point attr set, got %d", len(result))
			}
			if len(result[0]) != 1 {
				t.Fatalf("expected 1 KV, got %d", len(result[0]))
			}
			if result[0][0].Key != "key" {
				t.Errorf("expected key 'key', got %q", result[0][0].Key)
			}
			expectedVal := tt.name
			// Map test name to expected value
			switch tt.name {
			case "gauge":
				expectedVal = "gauge-val"
			case "sum":
				expectedVal = "sum-val"
			case "histogram":
				expectedVal = "hist-val"
			case "summary":
				expectedVal = "summary-val"
			case "exp_histogram":
				expectedVal = "exphist-val"
			}
			if result[0][0].Value != expectedVal {
				t.Errorf("expected value %q, got %q", expectedVal, result[0][0].Value)
			}
		})
	}
}

func TestReceiver_GRPCIntegration(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	recv, err := NewReceiver(ReceiverConfig{GRPCAddr: ":0", HTTPAddr: ":0"}, catalog, NewSpanCatalog(5*time.Minute), NewLogCatalog(5*time.Minute))
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	recv.Start()
	defer recv.Stop()

	// Connect a gRPC client
	conn, err := grpc.NewClient(
		recv.GRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pmetricotlp.NewGRPCClient(conn)

	// Build and send metrics
	md := buildTestMetrics()
	req := pmetricotlp.NewExportRequestFromMetrics(md)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Export(ctx, req)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if catalog.Len() != 3 {
		t.Errorf("expected 3 catalog entries after gRPC export, got %d", catalog.Len())
	}
}

func buildTestTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "my-service")

	ss := rs.ScopeSpans().AppendEmpty()

	span1 := ss.Spans().AppendEmpty()
	span1.SetName("GET /users")
	span1.SetKind(ptrace.SpanKindServer)
	span1.Status().SetCode(ptrace.StatusCodeOk)
	span1.Attributes().PutStr("http.method", "GET")

	span2 := ss.Spans().AppendEmpty()
	span2.SetName("SELECT users")
	span2.SetKind(ptrace.SpanKindClient)
	span2.Status().SetCode(ptrace.StatusCodeUnset)

	return td
}

func TestExtractAndRecordSpans(t *testing.T) {
	catalog := NewSpanCatalog(5 * time.Minute)
	td := buildTestTraces()

	extractAndRecordSpans(td, catalog)

	if catalog.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", catalog.Len())
	}

	entries := catalog.Entries()
	// Sorted by service name then span name
	if entries[0].SpanName != "GET /users" {
		t.Errorf("expected GET /users, got %s", entries[0].SpanName)
	}
	if entries[0].ServiceName != "my-service" {
		t.Errorf("expected my-service, got %s", entries[0].ServiceName)
	}
	if entries[0].SpanKind != SpanKindServer {
		t.Errorf("expected server, got %s", entries[0].SpanKind)
	}
	if entries[0].StatusCode != SpanStatusOk {
		t.Errorf("expected ok, got %s", entries[0].StatusCode)
	}

	if entries[1].SpanName != "SELECT users" {
		t.Errorf("expected SELECT users, got %s", entries[1].SpanName)
	}
	if entries[1].SpanKind != SpanKindClient {
		t.Errorf("expected client, got %s", entries[1].SpanKind)
	}
}

func TestExtractAndRecordSpans_Attributes(t *testing.T) {
	catalog := NewSpanCatalog(5 * time.Minute)
	td := buildTestTraces()

	extractAndRecordSpans(td, catalog)

	entries := catalog.Entries()
	span := entries[0] // GET /users

	var resourceAttrs, dpAttrs int
	for _, a := range span.Attributes {
		switch a.Level {
		case AttributeLevelResource:
			resourceAttrs++
		case AttributeLevelDatapoint:
			dpAttrs++
		}
	}
	if resourceAttrs != 1 {
		t.Errorf("expected 1 resource attribute (service.name), got %d", resourceAttrs)
	}
	if dpAttrs != 1 {
		t.Errorf("expected 1 datapoint attribute (http.method), got %d", dpAttrs)
	}
}

func buildTestLogs() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "my-service")

	sl := rl.ScopeLogs().AppendEmpty()

	lr1 := sl.LogRecords().AppendEmpty()
	lr1.SetSeverityNumber(plog.SeverityNumberInfo)
	lr1.SetSeverityText("INFO")
	lr1.Attributes().PutStr("log.source", "stdout")

	lr2 := sl.LogRecords().AppendEmpty()
	lr2.SetSeverityNumber(plog.SeverityNumberError)
	lr2.SetSeverityText("ERROR")

	lr3 := sl.LogRecords().AppendEmpty()
	lr3.SetSeverityNumber(plog.SeverityNumberInfo)
	lr3.SetSeverityText("INFO")

	return ld
}

func TestExtractAndRecordLogs(t *testing.T) {
	catalog := NewLogCatalog(5 * time.Minute)
	ld := buildTestLogs()

	extractAndRecordLogs(ld, catalog)

	// All logs share (my-service, unscoped, "") key → 1 entry
	if catalog.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", catalog.Len())
	}

	entries := catalog.Entries()
	if entries[0].ServiceName != "my-service" {
		t.Errorf("expected my-service, got %s", entries[0].ServiceName)
	}
	if entries[0].ScopeName != "unscoped" {
		t.Errorf("expected unscoped, got %s", entries[0].ScopeName)
	}
	if entries[0].RecordCount != 3 {
		t.Errorf("expected 3 total records, got %d", entries[0].RecordCount)
	}

	// Check severity distribution
	sevMap := make(map[SeverityRange]int64)
	for _, sc := range entries[0].SeverityCounts {
		sevMap[sc.Severity] = sc.Count
	}
	if sevMap[SeverityInfo] != 2 {
		t.Errorf("expected 2 info records, got %d", sevMap[SeverityInfo])
	}
	if sevMap[SeverityError] != 1 {
		t.Errorf("expected 1 error record, got %d", sevMap[SeverityError])
	}
}

func TestExtractAndRecordLogs_Attributes(t *testing.T) {
	catalog := NewLogCatalog(5 * time.Minute)
	ld := buildTestLogs()

	extractAndRecordLogs(ld, catalog)

	entries := catalog.Entries()
	// Single entry should have log.source datapoint attribute from lr1
	var dpAttrs int
	for _, a := range entries[0].Attributes {
		if a.Level == AttributeLevelDatapoint {
			dpAttrs++
		}
	}
	if dpAttrs != 1 {
		t.Errorf("expected 1 datapoint attribute (log.source), got %d", dpAttrs)
	}
}

func TestExtractAndRecordLogs_ScopeAndEvent(t *testing.T) {
	catalog := NewLogCatalog(5 * time.Minute)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "auth-svc")

	// Scoped logs
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("com.example.AuthController")
	lr := sl.LogRecords().AppendEmpty()
	lr.SetSeverityNumber(plog.SeverityNumberInfo)

	// Event logs
	sl2 := rl.ScopeLogs().AppendEmpty()
	sl2.Scope().SetName("otel.events")
	lr2 := sl2.LogRecords().AppendEmpty()
	lr2.SetSeverityNumber(plog.SeverityNumberInfo)
	lr2.Attributes().PutStr("event.name", "user.login")

	extractAndRecordLogs(ld, catalog)

	if catalog.Len() != 2 {
		t.Fatalf("expected 2 entries (scoped log + event), got %d", catalog.Len())
	}

	entries := catalog.Entries()
	// Sorted: (auth-svc, com.example.AuthController, "") then (auth-svc, otel.events, "user.login")
	if entries[0].ScopeName != "com.example.AuthController" {
		t.Errorf("expected com.example.AuthController, got %s", entries[0].ScopeName)
	}
	if entries[0].LogKind != LogKindLog {
		t.Errorf("expected log kind, got %s", entries[0].LogKind)
	}
	if entries[1].EventName != "user.login" {
		t.Errorf("expected user.login event, got %s", entries[1].EventName)
	}
	if entries[1].LogKind != LogKindEvent {
		t.Errorf("expected event kind, got %s", entries[1].LogKind)
	}
}

func TestLogRecordEventName(t *testing.T) {
	// No event name
	lr := plog.NewLogRecord()
	if got := logRecordEventName(lr); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// event.name attribute fallback
	lr2 := plog.NewLogRecord()
	lr2.Attributes().PutStr("event.name", "http.request")
	if got := logRecordEventName(lr2); got != "http.request" {
		t.Errorf("expected http.request, got %q", got)
	}
}

func TestReceiver_GRPCTraceIntegration(t *testing.T) {
	spanCatalog := NewSpanCatalog(5 * time.Minute)
	recv, err := NewReceiver(ReceiverConfig{GRPCAddr: ":0", HTTPAddr: ":0"},
		NewCatalog(5*time.Minute), spanCatalog, NewLogCatalog(5*time.Minute))
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	recv.Start()
	defer recv.Stop()

	conn, err := grpc.NewClient(
		recv.GRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := ptraceotlp.NewGRPCClient(conn)
	td := buildTestTraces()
	req := ptraceotlp.NewExportRequestFromTraces(td)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Export(ctx, req)
	if err != nil {
		t.Fatalf("trace export failed: %v", err)
	}

	if spanCatalog.Len() != 2 {
		t.Errorf("expected 2 span catalog entries after gRPC export, got %d", spanCatalog.Len())
	}
}

func TestReceiver_GRPCLogIntegration(t *testing.T) {
	logCatalog := NewLogCatalog(5 * time.Minute)
	recv, err := NewReceiver(ReceiverConfig{GRPCAddr: ":0", HTTPAddr: ":0"},
		NewCatalog(5*time.Minute), NewSpanCatalog(5*time.Minute), logCatalog)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	recv.Start()
	defer recv.Stop()

	conn, err := grpc.NewClient(
		recv.GRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := plogotlp.NewGRPCClient(conn)
	ld := buildTestLogs()
	req := plogotlp.NewExportRequestFromLogs(ld)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Export(ctx, req)
	if err != nil {
		t.Fatalf("log export failed: %v", err)
	}

	if logCatalog.Len() != 1 {
		t.Errorf("expected 1 log catalog entry after gRPC export, got %d", logCatalog.Len())
	}
}

func TestResourceServiceName(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("service.name", "my-svc")

	if got := resourceServiceName(attrs); got != "my-svc" {
		t.Errorf("expected my-svc, got %s", got)
	}

	empty := pcommon.NewMap()
	if got := resourceServiceName(empty); got != "unknown" {
		t.Errorf("expected unknown, got %s", got)
	}
}

func TestMapSpanKind(t *testing.T) {
	tests := []struct {
		in   ptrace.SpanKind
		want SpanKind
	}{
		{ptrace.SpanKindClient, SpanKindClient},
		{ptrace.SpanKindServer, SpanKindServer},
		{ptrace.SpanKindInternal, SpanKindInternal},
		{ptrace.SpanKindProducer, SpanKindProducer},
		{ptrace.SpanKindConsumer, SpanKindConsumer},
		{ptrace.SpanKindUnspecified, SpanKindUnset},
	}
	for _, tt := range tests {
		if got := mapSpanKind(tt.in); got != tt.want {
			t.Errorf("mapSpanKind(%v) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestMapSpanStatus(t *testing.T) {
	tests := []struct {
		in   ptrace.StatusCode
		want SpanStatusCode
	}{
		{ptrace.StatusCodeOk, SpanStatusOk},
		{ptrace.StatusCodeError, SpanStatusError},
		{ptrace.StatusCodeUnset, SpanStatusUnset},
	}
	for _, tt := range tests {
		if got := mapSpanStatus(tt.in); got != tt.want {
			t.Errorf("mapSpanStatus(%v) = %s, want %s", tt.in, got, tt.want)
		}
	}
}
