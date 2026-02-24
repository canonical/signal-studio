package api

import (
	"encoding/json"
	"net/http"

	"github.com/simskij/otel-signal-lens/internal/metrics"
)

type metricsHandler struct {
	mgr *metrics.Manager
}

type connectRequest struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

func (h *metricsHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url is required"})
		return
	}

	err := h.mgr.Connect(metrics.ScrapeConfig{
		URL:   req.URL,
		Token: req.Token,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

func (h *metricsHandler) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	h.mgr.Disconnect()
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

func (h *metricsHandler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	snap := h.mgr.Snapshot()
	writeJSON(w, http.StatusOK, snap)
}

func (h *metricsHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, lastErr := h.mgr.Status()
	resp := map[string]string{"status": string(status)}
	if lastErr != "" {
		resp["error"] = lastErr
	}
	writeJSON(w, http.StatusOK, resp)
}
