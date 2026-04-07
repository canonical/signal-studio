package tap

import (
	"fmt"
	"sync"
	"time"
)

// TapStatus represents the state of the tap.
type TapStatus string

const (
	TapStatusIdle      TapStatus = "idle"
	TapStatusListening TapStatus = "listening"
	TapStatusError     TapStatus = "error"
	TapStatusDisabled  TapStatus = "disabled"
)

const defaultTTL = 5 * time.Minute

// TapConfig holds the configuration for a tap session.
type TapConfig struct {
	GRPCAddr string
	HTTPAddr string
}

// Manager coordinates the tap lifecycle.
type Manager struct {
	mu          sync.RWMutex
	disabled    bool
	status      TapStatus
	lastErr     string
	catalog     *Catalog
	spanCatalog *SpanCatalog
	logCatalog  *LogCatalog
	receiver    *Receiver
	stopCh      chan struct{}
	startedAt   time.Time
	grpcAddr    string
	httpAddr    string
	remoteTap   *remoteTapClient
}

// NewManager creates a new tap Manager.
// If disabled is true the manager rejects Start calls and reports "disabled" status.
func NewManager(disabled bool) *Manager {
	status := TapStatusIdle
	if disabled {
		status = TapStatusDisabled
	}
	catalog := NewCatalog(defaultTTL)
	spanCatalog := NewSpanCatalog(defaultTTL)
	logCatalog := NewLogCatalog(defaultTTL)
	return &Manager{
		disabled:    disabled,
		status:      status,
		catalog:     catalog,
		spanCatalog: spanCatalog,
		logCatalog:  logCatalog,
		remoteTap:   newRemoteTapClient(catalog, spanCatalog, logCatalog),
	}
}

// Start begins a tap session. If already listening, the existing session is replaced.
// Returns an error when the manager was created in disabled mode.
func (m *Manager) Start(cfg TapConfig) error {
	if m.disabled {
		return fmt.Errorf("tap is disabled")
	}
	m.mu.Lock()
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	if m.receiver != nil {
		m.receiver.Stop()
		m.receiver = nil
	}
	m.mu.Unlock()

	rcvCfg := ReceiverConfig{
		GRPCAddr: cfg.GRPCAddr,
		HTTPAddr: cfg.HTTPAddr,
	}
	recv, err := NewReceiver(rcvCfg, m.catalog, m.spanCatalog, m.logCatalog)
	if err != nil {
		m.mu.Lock()
		m.status = TapStatusError
		m.lastErr = err.Error()
		m.mu.Unlock()
		return fmt.Errorf("start receiver: %w", err)
	}

	recv.Start()

	stopCh := make(chan struct{})

	m.mu.Lock()
	m.receiver = recv
	m.status = TapStatusListening
	m.lastErr = ""
	m.stopCh = stopCh
	m.startedAt = time.Now()
	m.grpcAddr = recv.GRPCAddr()
	m.httpAddr = recv.HTTPAddr()
	m.mu.Unlock()

	go m.pruneLoop(stopCh)

	return nil
}

// Stop ends the current tap session.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	if m.receiver != nil {
		m.receiver.Stop()
		m.receiver = nil
	}
	m.status = TapStatusIdle
	m.lastErr = ""
}

// Status returns the current tap status, last error, and start time.
func (m *Manager) Status() (TapStatus, string, time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status, m.lastErr, m.startedAt
}

// Addrs returns the actual gRPC and HTTP addresses the receiver is listening on.
func (m *Manager) Addrs() (grpc string, http string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.grpcAddr, m.httpAddr
}

// Catalog returns the metric catalog.
func (m *Manager) Catalog() *Catalog {
	return m.catalog
}

// SpanCatalog returns the span catalog.
func (m *Manager) SpanCatalog() *SpanCatalog {
	return m.spanCatalog
}

// LogCatalog returns the log catalog.
func (m *Manager) LogCatalog() *LogCatalog {
	return m.logCatalog
}

// ConnectRemoteTap starts an outbound WebSocket connection to a remotetapprocessor
// endpoint and feeds received telemetry into the shared catalogs.
func (m *Manager) ConnectRemoteTap(addr string) error {
	if m.disabled {
		return fmt.Errorf("tap is disabled")
	}
	return m.remoteTap.Connect(addr)
}

// DisconnectRemoteTap closes the remotetap WebSocket connection.
func (m *Manager) DisconnectRemoteTap() {
	m.remoteTap.Disconnect()
}

// RemoteTapStatus returns the current remotetap connection state, last error, and address.
func (m *Manager) RemoteTapStatus() (RemoteTapStatus, string, string) {
	return m.remoteTap.Status()
}

// pruneLoop periodically removes expired catalog entries.
func (m *Manager) pruneLoop(stopCh chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			m.catalog.Prune()
			m.spanCatalog.Prune()
			m.logCatalog.Prune()
		}
	}
}
