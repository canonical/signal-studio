package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/canonical/signal-studio/internal/tap"
)

type tapHandler struct {
	mgr             *tap.Manager
	defaultGRPCAddr string
	defaultHTTPAddr string
}

type tapStartRequest struct {
	GRPCAddr string `json:"grpcAddr"`
	HTTPAddr string `json:"httpAddr"`
}

func (h *tapHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	var req tapStartRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
			return
		}
	}

	grpcAddr := req.GRPCAddr
	if grpcAddr == "" {
		grpcAddr = h.defaultGRPCAddr
	}
	httpAddr := req.HTTPAddr
	if httpAddr == "" {
		httpAddr = h.defaultHTTPAddr
	}

	err := h.mgr.Start(tap.TapConfig{
		GRPCAddr: grpcAddr,
		HTTPAddr: httpAddr,
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
	if status == tap.TapStatusDisabled {
		resp["disabled"] = true
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
	metricCatalog := h.mgr.Catalog()
	metricEntries := metricCatalog.Entries()
	spanEntries := h.mgr.SpanCatalog().Entries()
	logEntries := h.mgr.LogCatalog().Entries()

	writeJSON(w, http.StatusOK, map[string]any{
		"metrics":     metricEntries,
		"spans":       spanEntries,
		"logs":        logEntries,
		"count":       len(metricEntries),
		"spanCount":   len(spanEntries),
		"logCount":    len(logEntries),
		"rateChanged": metricCatalog.RateChanged(),
	})
}

func (h *tapHandler) handleReset(w http.ResponseWriter, r *http.Request) {
	h.mgr.Catalog().Clear()
	h.mgr.SpanCatalog().Clear()
	h.mgr.LogCatalog().Clear()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
