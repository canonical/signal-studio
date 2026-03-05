package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/tap"
)

func TestHealthReturns200(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestAnalyzeValidYAML(t *testing.T) {
	router := newTestRouter(t)

	yaml := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
processors:
  batch: {}
exporters:
  debug:
    verbosity: detailed
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`
	req := httptest.NewRequest("POST", "/api/config/analyze", strings.NewReader(yaml))
	req.Header.Set("Content-Type", "text/yaml")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("analyze status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp analyzeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Config == nil {
		t.Fatal("config should not be nil")
	}
	if len(resp.Config.Pipelines) == 0 {
		t.Error("expected at least one pipeline")
	}
}

func TestAnalyzeInvalidYAML(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest("POST", "/api/config/analyze", strings.NewReader("not: valid: yaml: ["))
	req.Header.Set("Content-Type", "text/yaml")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAnalyzeEmptyBody(t *testing.T) {
	router := newTestRouter(t)

	// Empty YAML is valid (produces an empty config with no pipelines).
	req := httptest.NewRequest("POST", "/api/config/analyze", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/yaml")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp analyzeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Config.Pipelines) != 0 {
		t.Errorf("expected no pipelines, got %d", len(resp.Config.Pipelines))
	}
}

func TestAnalyzeWithTapCatalogData(t *testing.T) {
	mgr := metrics.NewManager(10 * time.Second)
	tapMgr := tap.NewManager(false)

	// Start tap to populate catalog.
	if err := tapMgr.Start(tap.TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"}); err != nil {
		t.Fatalf("tap start: %v", err)
	}
	defer tapMgr.Stop()

	// Add a metric to the catalog so catalog code paths are exercised.
	tapMgr.Catalog().Record("http_requests_total", "sum", nil, 1)

	router := NewRouter(mgr, tapMgr, nil)

	yaml := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
processors:
  filter/drop:
    metrics:
      include:
        match_type: strict
        metric_names:
          - http_requests_total
exporters:
  debug: {}
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [filter/drop]
      exporters: [debug]
`
	req := httptest.NewRequest("POST", "/api/config/analyze", strings.NewReader(yaml))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestPipelineSignalType(t *testing.T) {
	cfg := &config.CollectorConfig{
		Pipelines: map[string]config.Pipeline{
			"traces": {Signal: "traces"},
		},
	}
	if got := pipelineSignalType(cfg, "traces"); got != "traces" {
		t.Errorf("got %q, want traces", got)
	}
	if got := pipelineSignalType(cfg, "nonexistent"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestConvertEntriesToMetricInfos(t *testing.T) {
	entries := []tap.MetricEntry{
		{
			Name: "my_metric",
			Attributes: []tap.AttributeMeta{
				{Key: "host", Level: "resource", SampleValues: []string{"a", "b"}, Capped: false},
			},
		},
		{Name: "no_attrs"},
	}
	infos := convertEntriesToMetricInfos(entries)
	if len(infos) != 2 {
		t.Fatalf("len = %d, want 2", len(infos))
	}
	if infos[0].Name != "my_metric" {
		t.Errorf("name = %q", infos[0].Name)
	}
	if len(infos[0].Attributes) != 1 {
		t.Errorf("attrs len = %d", len(infos[0].Attributes))
	}
	if infos[0].Attributes[0].Key != "host" {
		t.Errorf("attr key = %q", infos[0].Attributes[0].Key)
	}
	if len(infos[1].Attributes) != 0 {
		t.Errorf("no_attrs should have 0 attrs")
	}
}

func TestExtractNamesFromFilterRules(t *testing.T) {
	fc := filter.FilterConfig{
		Rules: []filter.FilterRule{
			{Pattern: "metric_a"},
			{Pattern: "metric_b"},
			{Pattern: "metric_a"}, // duplicate
			{Pattern: ""},
		},
	}
	names := extractNamesFromFilterRules(fc)
	if len(names) != 2 {
		t.Errorf("len = %d, want 2", len(names))
	}
}

func TestAnalyzeWithMetricsConnected(t *testing.T) {
	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`# HELP up gauge
# TYPE up gauge
up 1
`))
	}))
	defer promServer.Close()

	mgr := metrics.NewManager(10 * time.Second)
	if err := mgr.Connect(metrics.ScrapeConfig{URL: promServer.URL}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Disconnect()

	tapMgr := tap.NewManager(false)
	router := NewRouter(mgr, tapMgr, nil)

	yaml := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`
	req := httptest.NewRequest("POST", "/api/config/analyze", strings.NewReader(yaml))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}
