package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/tap"
)

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	mgr := metrics.NewManager(10 * time.Second)
	tapMgr := tap.NewManager(false)
	return NewRouter(mgr, tapMgr, nil)
}

func TestCORSHeaders(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("CORS origin = %q, want http://localhost:5173", got)
	}
}

func TestOptionsPreflightReturns204(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestIsAllowedOriginEmpty(t *testing.T) {
	if !isAllowedOrigin("http://example.com", "") {
		t.Error("empty allowed should permit all origins")
	}
}

func TestIsAllowedOriginWildcard(t *testing.T) {
	if !isAllowedOrigin("http://example.com", "*") {
		t.Error("wildcard should permit all origins")
	}
}

func TestIsAllowedOriginSpecific(t *testing.T) {
	if !isAllowedOrigin("http://a.com", "http://a.com,http://b.com") {
		t.Error("specific match should be allowed")
	}
	if isAllowedOrigin("http://c.com", "http://a.com,http://b.com") {
		t.Error("non-matching origin should be rejected")
	}
}
