package tap

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
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

func TestReceiver_GRPCIntegration(t *testing.T) {
	catalog := NewCatalog(5 * time.Minute)
	recv, err := NewReceiver(ReceiverConfig{GRPCAddr: ":0", HTTPAddr: ":0"}, catalog)
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
