package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/simskij/signal-studio/internal/config"
	"github.com/simskij/signal-studio/internal/filter"
	"github.com/simskij/signal-studio/internal/metrics"
	"github.com/simskij/signal-studio/internal/rules"
	"github.com/simskij/signal-studio/internal/tap"
)

type analyzeResponse struct {
	Config         *config.CollectorConfig `json:"config"`
	Findings       []rules.Finding         `json:"findings"`
	FilterAnalyses []filter.FilterAnalysis  `json:"filterAnalyses,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type analyzeHandler struct {
	mgr    *metrics.Manager
	tapMgr *tap.Manager
}

func (h *analyzeHandler) handleAnalyzeConfig(w http.ResponseWriter, r *http.Request) {
	maxSize := 256 // default KB
	if v := os.Getenv("MAX_YAML_SIZE_KB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxSize = n
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, int64(maxSize)*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "failed to read request body"})
		return
	}

	cfg, err := config.Parse(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	engine := rules.NewDefaultEngine()

	// Use live rules when metrics are connected
	var findings []rules.Finding
	status, _ := h.mgr.Status()
	if status == metrics.StatusConnected {
		findings = engine.EvaluateWithMetrics(cfg, h.mgr.Store())
	} else {
		findings = engine.Evaluate(cfg)
	}

	// Compute filter analyses first so they're available for catalog rules
	var filterAnalyses []filter.FilterAnalysis
	if h.tapMgr != nil && h.tapMgr.Catalog().Len() > 0 {
		fcs := filter.ExtractFilterConfigs(cfg)
		metricNames := h.tapMgr.Catalog().Names()
		for _, fc := range fcs {
			if len(fc.Rules) == 0 {
				continue
			}
			filterAnalyses = append(filterAnalyses, filter.AnalyzeFilter(fc, metricNames))
		}
	}

	// Evaluate catalog rules when tap catalog has data
	if h.tapMgr != nil && h.tapMgr.Catalog().Len() > 0 {
		catalogFindings := engine.EvaluateWithCatalog(cfg, h.tapMgr.Catalog().Entries(), filterAnalyses)
		findings = append(findings, catalogFindings...)
	}

	resp := analyzeResponse{
		Config:         cfg,
		Findings:       findings,
		FilterAnalyses: filterAnalyses,
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
