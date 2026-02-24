package metrics

import (
	"context"
	"log"
	"sync"
	"time"
)

// ConnectionStatus represents the state of the metrics connection.
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusError        ConnectionStatus = "error"
)

// Manager coordinates the metrics scraping lifecycle.
type Manager struct {
	mu       sync.RWMutex
	status   ConnectionStatus
	lastErr  string
	store    *Store
	scraper  *Scraper
	cancel   context.CancelFunc
	interval time.Duration
}

// NewManager creates a new metrics manager.
func NewManager(interval time.Duration) *Manager {
	return &Manager{
		status:   StatusDisconnected,
		store:    NewStore(),
		interval: interval,
	}
}

// Connect starts scraping the given endpoint.
// It performs a test scrape first and returns an error if it fails.
func (m *Manager) Connect(cfg ScrapeConfig) error {
	m.mu.Lock()
	// If already connected, stop the existing scraper first
	if m.cancel != nil {
		m.cancel()
	}
	m.status = StatusConnecting
	m.lastErr = ""
	m.store.Clear()
	m.scraper = NewScraper(cfg)
	m.mu.Unlock()

	// Test scrape
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap, err := m.scraper.Scrape(ctx)
	if err != nil {
		m.mu.Lock()
		m.status = StatusError
		m.lastErr = err.Error()
		m.mu.Unlock()
		return err
	}

	m.store.Push(snap)

	// Start background scraping
	bgCtx, bgCancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancel = bgCancel
	m.status = StatusConnected
	m.mu.Unlock()

	go m.scrapeLoop(bgCtx)
	return nil
}

// Disconnect stops scraping and clears stored data.
func (m *Manager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.scraper = nil
	m.status = StatusDisconnected
	m.lastErr = ""
	m.store.Clear()
}

// Status returns the current connection status and last error (if any).
func (m *Manager) Status() (ConnectionStatus, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status, m.lastErr
}

// Snapshot returns the computed snapshot for the frontend.
func (m *Manager) Snapshot() *ComputedSnapshot {
	status, _ := m.Status()
	return ComputeSnapshot(m.store, string(status))
}

// Store returns the underlying store (for live rules).
func (m *Manager) Store() *Store {
	return m.store
}

func (m *Manager) scrapeLoop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, err := m.scraper.Scrape(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return // context cancelled, exit gracefully
				}
				log.Printf("metrics scrape error: %v", err)
				m.mu.Lock()
				m.status = StatusError
				m.lastErr = err.Error()
				m.mu.Unlock()
				continue
			}
			m.store.Push(snap)
			m.mu.Lock()
			m.status = StatusConnected
			m.lastErr = ""
			m.mu.Unlock()
		}
	}
}
