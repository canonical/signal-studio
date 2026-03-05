package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/tap"
)

func metricsRouter(t *testing.T, mgr *metrics.Manager) http.Handler {
	t.Helper()
	tapMgr := tap.NewManager(false)
	return NewRouter(mgr, tapMgr, nil)
}

func TestMetricsStatusDisconnected(t *testing.T) {
	mgr := metrics.NewManager(10 * time.Second)
	router := metricsRouter(t, mgr)

	req := httptest.NewRequest("GET", "/api/metrics/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "disconnected" {
		t.Errorf("status = %q, want disconnected", body["status"])
	}
}

func TestMetricsConnect(t *testing.T) {
	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`# HELP up gauge
# TYPE up gauge
up 1
`))
	}))
	defer promServer.Close()

	mgr := metrics.NewManager(10 * time.Second)
	router := metricsRouter(t, mgr)

	body := `{"url":"` + promServer.URL + `"}`
	req := httptest.NewRequest("POST", "/api/metrics/connect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("connect status = %d; body: %s", w.Code, w.Body.String())
	}

	// Disconnect to clean up.
	req = httptest.NewRequest("POST", "/api/metrics/disconnect", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("disconnect status = %d", w.Code)
	}
}

func TestMetricsConnectBadURL(t *testing.T) {
	mgr := metrics.NewManager(10 * time.Second)
	router := metricsRouter(t, mgr)

	body := `{"url":"http://127.0.0.1:1"}`
	req := httptest.NewRequest("POST", "/api/metrics/connect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestMetricsConnectInvalidBody(t *testing.T) {
	mgr := metrics.NewManager(10 * time.Second)
	router := metricsRouter(t, mgr)

	req := httptest.NewRequest("POST", "/api/metrics/connect", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestMetricsConnectEmptyURL(t *testing.T) {
	mgr := metrics.NewManager(10 * time.Second)
	router := metricsRouter(t, mgr)

	req := httptest.NewRequest("POST", "/api/metrics/connect", strings.NewReader(`{"url":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	mgr := metrics.NewManager(10 * time.Second)
	router := metricsRouter(t, mgr)

	req := httptest.NewRequest("GET", "/api/metrics/snapshot", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("snapshot status = %d", w.Code)
	}
}

func TestMetricsReset(t *testing.T) {
	mgr := metrics.NewManager(10 * time.Second)
	router := metricsRouter(t, mgr)

	req := httptest.NewRequest("POST", "/api/metrics/reset", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("reset status = %d", w.Code)
	}
}
