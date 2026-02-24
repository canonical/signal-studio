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
)

const defaultTTL = 5 * time.Minute

// TapConfig holds the configuration for a tap session.
type TapConfig struct {
	GRPCAddr string
	HTTPAddr string
}

// Manager coordinates the tap lifecycle.
type Manager struct {
	mu        sync.RWMutex
	status    TapStatus
	lastErr   string
	catalog   *Catalog
	receiver  *Receiver
	stopCh    chan struct{}
	startedAt time.Time
	grpcAddr  string
	httpAddr  string
}

// NewManager creates a new tap Manager in idle state.
func NewManager() *Manager {
	return &Manager{
		status:  TapStatusIdle,
		catalog: NewCatalog(defaultTTL),
	}
}

// Start begins a tap session. If already listening, the existing session is replaced.
func (m *Manager) Start(cfg TapConfig) error {
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
	recv, err := NewReceiver(rcvCfg, m.catalog)
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
		}
	}
}
