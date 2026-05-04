package tap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// wsServer starts a test WebSocket server that sends a single message then
// keeps the connection open until the client disconnects.
func wsServer(t *testing.T, msg []byte) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, msg)
		// Wait for client to close.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	return ts
}

func wsAddr(ts *httptest.Server) string {
	// Convert http://127.0.0.1:PORT → 127.0.0.1:PORT so normalizeWebSocketURL prepends ws://
	return strings.TrimPrefix(ts.URL, "http://")
}

func metricsJSON() []byte {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test.metric")
	m.SetEmptyGauge().DataPoints().AppendEmpty()
	b, _ := (&pmetric.JSONMarshaler{}).MarshalMetrics(md)
	return b
}

func tracesJSON() []byte {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	ss := rs.ScopeSpans().AppendEmpty()
	s := ss.Spans().AppendEmpty()
	s.SetName("op")
	b, _ := (&ptrace.JSONMarshaler{}).MarshalTraces(td)
	return b
}

func TestRemoteTapClientConnectsAndReceivesMetrics(t *testing.T) {
	ts := wsServer(t, metricsJSON())
	defer ts.Close()

	catalog := NewCatalog(defaultTTL)
	spanCatalog := NewSpanCatalog(defaultTTL)
	logCatalog := NewLogCatalog(defaultTTL)
	c := newRemoteTapClient(catalog, spanCatalog, logCatalog)

	if err := c.Connect(wsAddr(ts)); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Poll until catalog is populated (up to 2 seconds).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(catalog.Entries()) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	c.Disconnect()

	entries := catalog.Entries()
	if len(entries) == 0 {
		t.Fatal("catalog is empty after receiving metrics message")
	}
	if entries[0].Name != "test.metric" {
		t.Errorf("metric name = %q, want %q", entries[0].Name, "test.metric")
	}
}

func TestRemoteTapClientConnectsAndReceivesTraces(t *testing.T) {
	ts := wsServer(t, tracesJSON())
	defer ts.Close()

	catalog := NewCatalog(defaultTTL)
	spanCatalog := NewSpanCatalog(defaultTTL)
	logCatalog := NewLogCatalog(defaultTTL)
	c := newRemoteTapClient(catalog, spanCatalog, logCatalog)

	if err := c.Connect(wsAddr(ts)); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(spanCatalog.Entries()) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	c.Disconnect()

	entries := spanCatalog.Entries()
	if len(entries) == 0 {
		t.Fatal("span catalog is empty after receiving traces message")
	}
	if entries[0].SpanName != "op" {
		t.Errorf("span name = %q, want %q", entries[0].SpanName, "op")
	}
}

func TestRemoteTapClientStatusTransitions(t *testing.T) {
	ts := wsServer(t, metricsJSON())
	defer ts.Close()

	catalog := NewCatalog(defaultTTL)
	c := newRemoteTapClient(catalog, NewSpanCatalog(defaultTTL), NewLogCatalog(defaultTTL))

	if s, _, _ := c.Status(); s != RemoteTapStatusIdle {
		t.Errorf("initial status = %v, want idle", s)
	}

	_ = c.Connect(wsAddr(ts))

	// Poll until connected.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s, _, _ := c.Status(); s == RemoteTapStatusConnected {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if s, _, _ := c.Status(); s != RemoteTapStatusConnected {
		t.Errorf("status after connect = %v, want connected", s)
	}

	c.Disconnect()

	if s, _, _ := c.Status(); s != RemoteTapStatusIdle {
		t.Errorf("status after disconnect = %v, want idle", s)
	}
}

func TestRemoteTapClientInvalidAddress(t *testing.T) {
	catalog := NewCatalog(defaultTTL)
	c := newRemoteTapClient(catalog, NewSpanCatalog(defaultTTL), NewLogCatalog(defaultTTL))

	_ = c.Connect("127.0.0.1:1") // nothing listening on port 1

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s, _, _ := c.Status(); s == RemoteTapStatusError {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	s, errMsg, _ := c.Status()
	if s != RemoteTapStatusError {
		t.Errorf("status = %v, want error", s)
	}
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}

	c.Disconnect() // should not panic or deadlock
}

func TestNormalizeWebSocketURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"localhost:12001", "ws://localhost:12001"},
		{"ws://localhost:12001", "ws://localhost:12001"},
		{"wss://secure:443", "wss://secure:443"},
	}
	for _, tc := range cases {
		got := normalizeWebSocketURL(tc.input)
		if got != tc.want {
			t.Errorf("normalizeWebSocketURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRemoteTapClientIgnoresUnknownSignal(t *testing.T) {
	ts := wsServer(t, []byte(`{"unknownKey":[]}`))
	defer ts.Close()

	catalog := NewCatalog(defaultTTL)
	c := newRemoteTapClient(catalog, NewSpanCatalog(defaultTTL), NewLogCatalog(defaultTTL))
	_ = c.Connect(wsAddr(ts))

	// Give it time to process the message; catalog should remain empty.
	time.Sleep(200 * time.Millisecond)
	c.Disconnect()

	if len(catalog.Entries()) > 0 {
		t.Error("catalog should be empty for unknown signal type")
	}
}

