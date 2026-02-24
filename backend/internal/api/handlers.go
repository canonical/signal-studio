package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/simskij/otel-signal-lens/internal/config"
	"github.com/simskij/otel-signal-lens/internal/rules"
)

type analyzeResponse struct {
	Config   *config.CollectorConfig `json:"config"`
	Findings []rules.Finding         `json:"findings"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func handleAnalyzeConfig(w http.ResponseWriter, r *http.Request) {
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
	findings := engine.Evaluate(cfg)

	writeJSON(w, http.StatusOK, analyzeResponse{
		Config:   cfg,
		Findings: findings,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
