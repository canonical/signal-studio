package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAlertCoverageInlineRules(t *testing.T) {
	router := newTestRouter(t)

	body := `{
		"rules": "groups:\n  - name: test\n    rules:\n      - alert: HighErrors\n        expr: rate(errors_total[5m]) > 0.1\n",
		"configYaml": "receivers:\n  otlp:\n    protocols:\n      grpc:\n        endpoint: 0.0.0.0:4317\nexporters:\n  debug: {}\nservice:\n  pipelines:\n    metrics:\n      receivers: [otlp]\n      exporters: [debug]\n"
	}`
	req := httptest.NewRequest("POST", "/api/alert-coverage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["results"]; !ok {
		t.Error("response should contain results")
	}
}

func TestAlertCoverageMissingRules(t *testing.T) {
	router := newTestRouter(t)

	body := `{"configYaml": "receivers: {}"}`
	req := httptest.NewRequest("POST", "/api/alert-coverage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAlertCoverageInvalidJSON(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest("POST", "/api/alert-coverage", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAlertCoverageWithConfigAndFilter(t *testing.T) {
	router := newTestRouter(t)

	body := `{
		"rules": "groups:\n  - name: test\n    rules:\n      - alert: HighErrors\n        expr: rate(errors_total[5m]) > 0.1\n",
		"configYaml": "receivers:\n  otlp:\n    protocols:\n      grpc:\n        endpoint: 0.0.0.0:4317\nprocessors:\n  filter/drop:\n    metrics:\n      include:\n        match_type: strict\n        metric_names:\n          - errors_total\nexporters:\n  debug: {}\nservice:\n  pipelines:\n    metrics:\n      receivers: [otlp]\n      processors: [filter/drop]\n      exporters: [debug]\n"
	}`
	req := httptest.NewRequest("POST", "/api/alert-coverage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAlertCoverageInvalidRulesYAML(t *testing.T) {
	router := newTestRouter(t)

	body := `{"rules": "not: valid: yaml: ["}`
	req := httptest.NewRequest("POST", "/api/alert-coverage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
