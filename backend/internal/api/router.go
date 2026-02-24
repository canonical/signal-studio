package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/simskij/otel-signal-lens/internal/metrics"
)

// NewRouter creates the HTTP handler with all routes.
func NewRouter(mgr *metrics.Manager) http.Handler {
	mux := http.NewServeMux()

	ah := &analyzeHandler{mgr: mgr}
	mux.HandleFunc("POST /api/config/analyze", ah.handleAnalyzeConfig)
	mux.HandleFunc("GET /api/health", handleHealth)

	mh := &metricsHandler{mgr: mgr}
	mux.HandleFunc("POST /api/metrics/connect", mh.handleConnect)
	mux.HandleFunc("POST /api/metrics/disconnect", mh.handleDisconnect)
	mux.HandleFunc("GET /api/metrics/snapshot", mh.handleSnapshot)
	mux.HandleFunc("GET /api/metrics/status", mh.handleStatus)

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	origins := os.Getenv("CORS_ORIGINS")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, origins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedOrigin(origin, allowed string) bool {
	if allowed == "" || allowed == "*" {
		return true
	}
	for _, o := range strings.Split(allowed, ",") {
		if strings.TrimSpace(o) == origin {
			return true
		}
	}
	return false
}
