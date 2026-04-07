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
	"github.com/gorilla/websocket"
)

func tapRouter(t *testing.T, tapMgr *tap.Manager) http.Handler {
	t.Helper()
	mgr := metrics.NewManager(10 * time.Second)
	return NewRouter(mgr, tapMgr, nil)
}

func TestTapStatusIdle(t *testing.T) {
	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	req := httptest.NewRequest("GET", "/api/tap/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "idle" {
		t.Errorf("status = %v, want idle", body["status"])
	}
}

func TestTapStartStop(t *testing.T) {
	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	// Start with ephemeral ports.
	body := `{"grpcAddr":":0","httpAddr":":0"}`
	req := httptest.NewRequest("POST", "/api/tap/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("start status = %d; body: %s", w.Code, w.Body.String())
	}

	// Status should be listening.
	req = httptest.NewRequest("GET", "/api/tap/status", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var statusBody map[string]any
	json.NewDecoder(w.Body).Decode(&statusBody)
	if statusBody["status"] != "listening" {
		t.Errorf("status = %v, want listening", statusBody["status"])
	}

	// Stop.
	req = httptest.NewRequest("POST", "/api/tap/stop", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("stop status = %d", w.Code)
	}
}

func TestTapCatalogEmpty(t *testing.T) {
	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	req := httptest.NewRequest("GET", "/api/tap/catalog", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("catalog status = %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if count, ok := body["count"].(float64); !ok || count != 0 {
		t.Errorf("count = %v, want 0", body["count"])
	}
}

func TestTapReset(t *testing.T) {
	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	req := httptest.NewRequest("POST", "/api/tap/reset", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("reset status = %d", w.Code)
	}
}

func TestTapStartInvalidBody(t *testing.T) {
	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	req := httptest.NewRequest("POST", "/api/tap/start", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTapStartWhenDisabled(t *testing.T) {
	tapMgr := tap.NewManager(true) // disabled
	router := tapRouter(t, tapMgr)

	body := `{"grpcAddr":":0","httpAddr":":0"}`
	req := httptest.NewRequest("POST", "/api/tap/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("start-when-disabled status = %d, want 500", w.Code)
	}
}

func TestTapStatusDisabled(t *testing.T) {
	tapMgr := tap.NewManager(true) // disabled
	router := tapRouter(t, tapMgr)

	req := httptest.NewRequest("GET", "/api/tap/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["disabled"] != true {
		t.Errorf("disabled = %v, want true", body["disabled"])
	}
}

func TestTapStatusIncludesRemoteTap(t *testing.T) {
	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	req := httptest.NewRequest("GET", "/api/tap/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)

	rt, ok := body["remotetap"].(map[string]any)
	if !ok {
		t.Fatalf("remotetap field missing or wrong type: %v", body["remotetap"])
	}
	if rt["status"] != "idle" {
		t.Errorf("remotetap.status = %v, want idle", rt["status"])
	}
}

func TestRemoteTapHandlerConnectMissingAddr(t *testing.T) {
	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	req := httptest.NewRequest("POST", "/api/tap/remotetap/connect", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestRemoteTapHandlerConnectAndDisconnect(t *testing.T) {
	// Start a minimal WebSocket server.
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer ts.Close()

	addr := strings.TrimPrefix(ts.URL, "http://")

	tapMgr := tap.NewManager(false)
	router := tapRouter(t, tapMgr)

	body, _ := json.Marshal(map[string]string{"addr": addr})
	req := httptest.NewRequest("POST", "/api/tap/remotetap/connect", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("connect status = %d; body: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/tap/remotetap/disconnect", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("disconnect status = %d", w.Code)
	}
}
