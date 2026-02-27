package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/canonical/signal-studio/internal/alertcoverage"
	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/filter"
	"github.com/canonical/signal-studio/internal/tap"
)

type alertCoverageHandler struct {
	tapMgr *tap.Manager
}

type alertCoverageRequest struct {
	Rules      string `json:"rules"`
	ConfigYAML string `json:"configYaml"`
	RulesURL   string `json:"rulesUrl,omitempty"`
	Token      string `json:"token,omitempty"`
	OrgID      string `json:"orgId,omitempty"`
}

func (h *alertCoverageHandler) handleAlertCoverage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "failed to read request body"})
		return
	}

	var req alertCoverageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON"})
		return
	}

	var alertRules []alertcoverage.AlertRule
	var rulesYAML string
	switch {
	case req.RulesURL != "":
		result, err := alertcoverage.FetchRules(alertcoverage.ClientOptions{
			URL:   req.RulesURL,
			Token: req.Token,
			OrgID: req.OrgID,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to fetch rules: " + err.Error()})
			return
		}
		alertRules = result.Rules
		rulesYAML = result.RulesYAML
	case req.Rules != "":
		parsed, err := alertcoverage.ParseRules([]byte(req.Rules))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "failed to parse alert rules: " + err.Error()})
			return
		}
		alertRules = parsed
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "either rules or rulesUrl field is required"})
		return
	}

	// Build filter analyses from the collector config if provided.
	var analyses []filter.FilterAnalysis
	if req.ConfigYAML != "" {
		cfg, err := config.Parse([]byte(req.ConfigYAML))
		if err == nil {
			analyses = h.buildFilterAnalyses(cfg)
		}
	}

	var knownMetrics map[string]struct{}
	if h.tapMgr != nil && h.tapMgr.Catalog().Len() > 0 {
		entries := h.tapMgr.Catalog().Entries()
		knownMetrics = make(map[string]struct{}, len(entries))
		for _, e := range entries {
			knownMetrics[e.Name] = struct{}{}
		}
	}

	report := alertcoverage.Analyze(alertRules, analyses, knownMetrics)
	report.RulesYAML = rulesYAML
	writeJSON(w, http.StatusOK, report)
}

func (h *alertCoverageHandler) buildFilterAnalyses(cfg *config.CollectorConfig) []filter.FilterAnalysis {
	fcs := filter.ExtractFilterConfigs(cfg)
	var analyses []filter.FilterAnalysis

	hasCatalog := h.tapMgr != nil && h.tapMgr.Catalog().Len() > 0
	for _, fc := range fcs {
		if len(fc.Rules) == 0 {
			continue
		}
		if hasCatalog {
			entries := h.tapMgr.Catalog().Entries()
			metricInfos := convertEntriesToMetricInfos(entries)
			analyses = append(analyses, filter.AnalyzeFilterWithAttributes(fc, metricInfos))
		} else {
			// Without catalog, still extract metric names from filter rules for basic matching.
			names := extractNamesFromFilterRules(fc)
			if len(names) > 0 {
				analyses = append(analyses, filter.AnalyzeFilter(fc, names))
			}
		}
	}
	return analyses
}

// extractNamesFromFilterRules extracts any literal metric names that appear
// in filter rules for basic name-level analysis.
func extractNamesFromFilterRules(fc filter.FilterConfig) []string {
	seen := make(map[string]struct{})
	var names []string
	for _, r := range fc.Rules {
		if r.Pattern != "" {
			if _, ok := seen[r.Pattern]; !ok {
				seen[r.Pattern] = struct{}{}
				names = append(names, r.Pattern)
			}
		}
	}
	return names
}
