package api

import (
	"net/http"
	"time"

	"github.com/canonical/signal-studio/internal/tap"
)

type tapHandler struct {
	mgr *tap.Manager
}

func (h *tapHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	// Start is only used as a fallback — normally the tap is started via
	// the TAP_ENABLED env var on server startup.
	err := h.mgr.Start(tap.TapConfig{
		GRPCAddr: ":4317",
		HTTPAddr: ":4318",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "listening"})
}

func (h *tapHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	h.mgr.Stop()
	writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
}

func (h *tapHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, lastErr, startedAt := h.mgr.Status()
	grpcAddr, httpAddr := h.mgr.Addrs()

	resp := map[string]any{
		"status":   string(status),
		"grpcAddr": grpcAddr,
		"httpAddr": httpAddr,
	}
	if lastErr != "" {
		resp["error"] = lastErr
	}
	if !startedAt.IsZero() {
		resp["startedAt"] = startedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *tapHandler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	catalog := h.mgr.Catalog()
	entries := catalog.Entries()
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":     entries,
		"count":       len(entries),
		"rateChanged": catalog.RateChanged(),
	})
}

func (h *tapHandler) handleReset(w http.ResponseWriter, r *http.Request) {
	h.mgr.Catalog().Clear()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
