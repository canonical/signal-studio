package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/rules"
	"github.com/canonical/signal-studio/internal/rules/engine"
	"github.com/canonical/signal-studio/internal/tap"
)

type analyzeResponse struct {
	Config         *config.CollectorConfig `json:"config"`
	Findings       []rules.Finding         `json:"findings"`
	FilterAnalyses []filter.FilterAnalysis  `json:"filterAnalyses,omitempty"`
	Summary        *analyzeResponseSummary `json:"summary,omitempty"`
}

type analyzeResponseSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
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

	eng := engine.NewDefaultEngine()

	// Use live rules when metrics are connected
	var findings []rules.Finding
	status, _ := h.mgr.Status()
	if status == metrics.StatusConnected {
		findings = eng.EvaluateWithMetrics(cfg, h.mgr.Store())
	} else {
		findings = eng.Evaluate(cfg)
	}

	// Compute filter analyses first so they're available for catalog rules
	var filterAnalyses []filter.FilterAnalysis
	metricCatalogHasData := h.tapMgr != nil && h.tapMgr.Catalog().Len() > 0
	spanCatalogHasData := h.tapMgr != nil && h.tapMgr.SpanCatalog().Len() > 0
	if metricCatalogHasData || spanCatalogHasData {
		fcs := filter.ExtractFilterConfigs(cfg)
		for _, fc := range fcs {
			if len(fc.Rules) == 0 {
				continue
			}
			pipelineSignal := pipelineSignalType(cfg, fc.Pipeline)
			if pipelineSignal == config.SignalTraces && spanCatalogHasData {
				spanEntries := h.tapMgr.SpanCatalog().Entries()
				spanNames := make([]string, len(spanEntries))
				for i, e := range spanEntries {
					spanNames[i] = e.SpanName
				}
				filterAnalyses = append(filterAnalyses, filter.AnalyzeFilter(fc, spanNames))
			} else if metricCatalogHasData {
				entries := h.tapMgr.Catalog().Entries()
				metricInfos := convertEntriesToMetricInfos(entries)
				filterAnalyses = append(filterAnalyses, filter.AnalyzeFilterWithAttributes(fc, metricInfos))
			}
		}
	}

	// Evaluate catalog rules when tap catalog has data
	if h.tapMgr != nil && h.tapMgr.Catalog().Len() > 0 {
		catalogFindings := eng.EvaluateWithCatalog(cfg, h.tapMgr.Catalog().Entries(), filterAnalyses)
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

// pipelineSignalType returns the signal type for a named pipeline.
func pipelineSignalType(cfg *config.CollectorConfig, pipelineName string) config.Signal {
	if p, ok := cfg.Pipelines[pipelineName]; ok {
		return p.Signal
	}
	return ""
}

// convertEntriesToMetricInfos converts tap catalog entries to filter-compatible metric infos.
func convertEntriesToMetricInfos(entries []tap.MetricEntry) []filter.MetricAttributeInfo {
	infos := make([]filter.MetricAttributeInfo, len(entries))
	for i, e := range entries {
		infos[i] = filter.MetricAttributeInfo{
			Name: e.Name,
		}
		if len(e.Attributes) > 0 {
			attrs := make([]filter.AttrMeta, len(e.Attributes))
			for j, a := range e.Attributes {
				attrs[j] = filter.AttrMeta{
					Key:          a.Key,
					Level:        string(a.Level),
					SampleValues: a.SampleValues,
					Capped:       a.Capped,
				}
			}
			infos[i].Attributes = attrs
		}
	}
	return infos
}
