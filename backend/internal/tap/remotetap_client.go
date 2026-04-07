package tap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// RemoteTapStatus represents the state of the remotetap client connection.
type RemoteTapStatus string

const (
	RemoteTapStatusIdle       RemoteTapStatus = "idle"
	RemoteTapStatusConnecting RemoteTapStatus = "connecting"
	RemoteTapStatusConnected  RemoteTapStatus = "connected"
	RemoteTapStatusError      RemoteTapStatus = "error"
)

// remoteTapClient connects to a remotetapprocessor WebSocket endpoint and feeds
// received telemetry into the shared catalogs.
type remoteTapClient struct {
	mu          sync.RWMutex
	status      RemoteTapStatus
	lastErr     string
	addr        string
	catalog     *Catalog
	spanCatalog *SpanCatalog
	logCatalog  *LogCatalog
	cancel      context.CancelFunc
	doneCh      chan struct{}
}

func newRemoteTapClient(catalog *Catalog, spanCatalog *SpanCatalog, logCatalog *LogCatalog) *remoteTapClient {
	return &remoteTapClient{
		status:      RemoteTapStatusIdle,
		catalog:     catalog,
		spanCatalog: spanCatalog,
		logCatalog:  logCatalog,
	}
}

// Connect dials the given address and starts streaming telemetry in the
// background. If already connected the existing connection is closed first.
func (c *remoteTapClient) Connect(addr string) error {
	c.Disconnect()

	wsURL := normalizeWebSocketURL(addr)

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	c.mu.Lock()
	c.status = RemoteTapStatusConnecting
	c.lastErr = ""
	c.addr = addr
	c.cancel = cancel
	c.doneCh = doneCh
	c.mu.Unlock()

	go c.run(ctx, wsURL, doneCh)

	return nil
}

// Disconnect closes the current connection and blocks until the background
// goroutine has exited.
func (c *remoteTapClient) Disconnect() {
	c.mu.Lock()
	cancel := c.cancel
	doneCh := c.doneCh
	c.cancel = nil
	c.doneCh = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if doneCh != nil {
		<-doneCh
	}

	c.mu.Lock()
	c.status = RemoteTapStatusIdle
	c.lastErr = ""
	c.addr = ""
	c.mu.Unlock()
}

// Status returns the current connection state, last error, and endpoint address.
func (c *remoteTapClient) Status() (RemoteTapStatus, string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status, c.lastErr, c.addr
}

func (c *remoteTapClient) run(ctx context.Context, wsURL string, doneCh chan struct{}) {
	defer close(doneCh)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		c.mu.Lock()
		c.status = RemoteTapStatusError
		c.lastErr = fmt.Sprintf("dial: %v", err)
		c.mu.Unlock()
		return
	}
	defer conn.Close()

	// Close the WebSocket connection when context is cancelled so ReadMessage
	// unblocks and the loop exits cleanly.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	c.mu.Lock()
	c.status = RemoteTapStatusConnected
	c.mu.Unlock()

	// Run catalog pruning for the duration of this connection so entries expire
	// even when the passive OTLP tap is not running.
	go c.pruneLoop(ctx)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-ctx.Done():
				// Normal shutdown via Disconnect(); leave status as-is.
			default:
				c.mu.Lock()
				c.status = RemoteTapStatusError
				c.lastErr = fmt.Sprintf("read: %v", err)
				c.mu.Unlock()
			}
			return
		}
		c.process(msg)
	}
}

func (c *remoteTapClient) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.catalog.Prune()
			c.spanCatalog.Prune()
			c.logCatalog.Prune()
		}
	}
}

// signalPeek is used to identify the signal type from the top-level JSON key.
type signalPeek struct {
	ResourceMetrics json.RawMessage `json:"resourceMetrics"`
	ResourceSpans   json.RawMessage `json:"resourceSpans"`
	ResourceLogs    json.RawMessage `json:"resourceLogs"`
}

func (c *remoteTapClient) process(msg []byte) {
	var peek signalPeek
	if err := json.Unmarshal(msg, &peek); err != nil {
		return
	}
	switch {
	case len(peek.ResourceMetrics) > 0:
		metrics, err := (&pmetric.JSONUnmarshaler{}).UnmarshalMetrics(msg)
		if err == nil {
			extractAndRecord(metrics, c.catalog)
		}
	case len(peek.ResourceSpans) > 0:
		traces, err := (&ptrace.JSONUnmarshaler{}).UnmarshalTraces(msg)
		if err == nil {
			extractAndRecordSpans(traces, c.spanCatalog)
		}
	case len(peek.ResourceLogs) > 0:
		logs, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs(msg)
		if err == nil {
			extractAndRecordLogs(logs, c.logCatalog)
		}
	}
}

// normalizeWebSocketURL ensures the address has a ws:// or wss:// scheme.
func normalizeWebSocketURL(addr string) string {
	if strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://") {
		return addr
	}
	return "ws://" + addr
}
