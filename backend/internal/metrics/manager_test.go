package metrics

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerConnectDisconnect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(samplePrometheusOutput))
	}))
	defer srv.Close()

	mgr := NewManager(100 * time.Millisecond)

	// Initially disconnected
	status, _ := mgr.Status()
	if status != StatusDisconnected {
		t.Errorf("initial status = %q, want disconnected", status)
	}

	// Connect
	err := mgr.Connect(ScrapeConfig{URL: srv.URL})
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}

	status, _ = mgr.Status()
	if status != StatusConnected {
		t.Errorf("status after connect = %q, want connected", status)
	}

	// Snapshot should have data from the test scrape
	snap := mgr.Snapshot()
	if snap.Status != "connected" {
		t.Errorf("snapshot status = %q, want connected", snap.Status)
	}

	// Wait for at least one background scrape
	time.Sleep(200 * time.Millisecond)

	if mgr.Store().Len() < 2 {
		t.Errorf("store len = %d, expected >= 2 after background scrape", mgr.Store().Len())
	}

	// Disconnect
	mgr.Disconnect()
	status, _ = mgr.Status()
	if status != StatusDisconnected {
		t.Errorf("status after disconnect = %q, want disconnected", status)
	}

	if mgr.Store().Len() != 0 {
		t.Errorf("store len = %d after disconnect, want 0", mgr.Store().Len())
	}
}

func TestManagerConnectFailure(t *testing.T) {
	// Server that returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mgr := NewManager(10 * time.Second)
	err := mgr.Connect(ScrapeConfig{URL: srv.URL})
	if err == nil {
		t.Fatal("expected error connecting to failing endpoint")
	}

	status, lastErr := mgr.Status()
	if status != StatusError {
		t.Errorf("status = %q, want error", status)
	}
	if lastErr == "" {
		t.Error("expected lastErr to be set")
	}
}

func TestManagerReconnect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(samplePrometheusOutput))
	}))
	defer srv.Close()

	mgr := NewManager(1 * time.Second)

	// Connect first time
	err := mgr.Connect(ScrapeConfig{URL: srv.URL})
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}

	// Connect again (should stop previous scraper)
	err = mgr.Connect(ScrapeConfig{URL: srv.URL})
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}

	status, _ := mgr.Status()
	if status != StatusConnected {
		t.Errorf("status = %q, want connected", status)
	}

	mgr.Disconnect()
}

func TestManagerScrapeLoopHandlesError(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First call (test scrape) succeeds
			w.Write([]byte(samplePrometheusOutput))
		} else {
			// Subsequent calls fail
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	mgr := NewManager(100 * time.Millisecond)
	err := mgr.Connect(ScrapeConfig{URL: srv.URL})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Wait for a failed background scrape
	time.Sleep(250 * time.Millisecond)

	status, lastErr := mgr.Status()
	if status != StatusError {
		t.Errorf("status = %q, want error after failed scrape", status)
	}
	if lastErr == "" {
		t.Error("expected lastErr after failed scrape")
	}

	mgr.Disconnect()
}

func TestResetStore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(samplePrometheusOutput))
	}))
	defer srv.Close()

	mgr := NewManager(1 * time.Second)

	err := mgr.Connect(ScrapeConfig{URL: srv.URL})
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}

	// Verify we have data after connect
	if mgr.Store().Len() == 0 {
		t.Fatal("expected store to have data after connect")
	}

	// Reset the store
	mgr.ResetStore()

	// Store should be empty
	if mgr.Store().Len() != 0 {
		t.Errorf("store len = %d after ResetStore, want 0", mgr.Store().Len())
	}

	// Connection status should still be connected
	status, _ := mgr.Status()
	if status != StatusConnected {
		t.Errorf("status = %q after ResetStore, want connected", status)
	}

	mgr.Disconnect()
}

func TestManagerSnapshotDisconnected(t *testing.T) {
	mgr := NewManager(10 * time.Second)
	snap := mgr.Snapshot()
	if snap.Status != "disconnected" {
		t.Errorf("snapshot status = %q, want disconnected", snap.Status)
	}
}
