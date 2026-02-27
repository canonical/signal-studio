package tap

import (
	"testing"
	"time"
)

func TestManager_StartStop(t *testing.T) {
	mgr := NewManager(false)

	status, _, _ := mgr.Status()
	if status != TapStatusIdle {
		t.Fatalf("expected idle, got %s", status)
	}

	err := mgr.Start(TapConfig{
		GRPCAddr: ":0",
		HTTPAddr: ":0",
	})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	status, _, startedAt := mgr.Status()
	if status != TapStatusListening {
		t.Errorf("expected listening, got %s", status)
	}
	if startedAt.IsZero() {
		t.Error("startedAt should not be zero")
	}

	mgr.Stop()

	status, _, _ = mgr.Status()
	if status != TapStatusIdle {
		t.Errorf("expected idle after stop, got %s", status)
	}
}

func TestManager_StaysListening(t *testing.T) {
	mgr := NewManager(false)

	err := mgr.Start(TapConfig{
		GRPCAddr: ":0",
		HTTPAddr: ":0",
	})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer mgr.Stop()

	// Should still be listening after a delay (no auto-stop)
	time.Sleep(200 * time.Millisecond)

	status, _, _ := mgr.Status()
	if status != TapStatusListening {
		t.Errorf("expected listening (no auto-stop), got %s", status)
	}
}

func TestManager_CatalogPersistsAcrossRestarts(t *testing.T) {
	mgr := NewManager(false)

	err := mgr.Start(TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	mgr.Catalog().Record("metric.a", MetricTypeGauge, nil, 10)
	mgr.Stop()

	// Restart
	err = mgr.Start(TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"})
	if err != nil {
		t.Fatalf("start 2 failed: %v", err)
	}

	mgr.Catalog().Record("metric.b", MetricTypeSum, nil, 5)
	mgr.Stop()

	if mgr.Catalog().Len() != 2 {
		t.Errorf("expected 2 entries persisted across restarts, got %d", mgr.Catalog().Len())
	}
}

func TestManager_StartWhileListening(t *testing.T) {
	mgr := NewManager(false)

	err := mgr.Start(TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"})
	if err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	// Start again while listening — should replace session
	err = mgr.Start(TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"})
	if err != nil {
		t.Fatalf("second start failed: %v", err)
	}

	status, _, _ := mgr.Status()
	if status != TapStatusListening {
		t.Errorf("expected listening after restart, got %s", status)
	}

	mgr.Stop()
}

func TestManager_StartFailure(t *testing.T) {
	mgr := NewManager(false)

	err := mgr.Start(TapConfig{
		GRPCAddr: "invalid-not-a-real-addr:99999999",
		HTTPAddr: ":0",
	})
	if err == nil {
		mgr.Stop()
		t.Fatal("expected error for invalid address")
	}

	status, lastErr, _ := mgr.Status()
	if status != TapStatusError {
		t.Errorf("expected error status, got %s", status)
	}
	if lastErr == "" {
		t.Error("expected non-empty lastErr")
	}
}

func TestManager_Addrs(t *testing.T) {
	mgr := NewManager(false)

	err := mgr.Start(TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer mgr.Stop()

	grpc, http := mgr.Addrs()
	if grpc == "" {
		t.Error("expected non-empty gRPC addr")
	}
	if http == "" {
		t.Error("expected non-empty HTTP addr")
	}
}

func TestManager_TimestampCorrectness(t *testing.T) {
	mgr := NewManager(false)
	before := time.Now()

	err := mgr.Start(TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer mgr.Stop()

	after := time.Now()

	_, _, startedAt := mgr.Status()

	if startedAt.Before(before) || startedAt.After(after) {
		t.Errorf("startedAt %v not between %v and %v", startedAt, before, after)
	}
}

func TestManager_Disabled(t *testing.T) {
	mgr := NewManager(true)

	status, _, _ := mgr.Status()
	if status != TapStatusDisabled {
		t.Fatalf("expected disabled, got %s", status)
	}

	err := mgr.Start(TapConfig{GRPCAddr: ":0", HTTPAddr: ":0"})
	if err == nil {
		mgr.Stop()
		t.Fatal("expected error when starting disabled manager")
	}

	// Status should still be disabled after failed start
	status, _, _ = mgr.Status()
	if status != TapStatusDisabled {
		t.Errorf("expected disabled after failed start, got %s", status)
	}
}
