package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/tap"
)

// NewRouter creates the HTTP handler with all routes.
// If staticHandler is non-nil it is registered as the catch-all "/" route
// to serve the embedded frontend SPA.
func NewRouter(mgr *metrics.Manager, tapMgr *tap.Manager, staticHandler http.Handler) http.Handler {
	mux := http.NewServeMux()

	ah := &analyzeHandler{mgr: mgr, tapMgr: tapMgr}
	mux.HandleFunc("POST /api/config/analyze", ah.handleAnalyzeConfig)
	mux.HandleFunc("GET /api/health", handleHealth)

	mh := &metricsHandler{mgr: mgr}
	mux.HandleFunc("POST /api/metrics/connect", mh.handleConnect)
	mux.HandleFunc("POST /api/metrics/disconnect", mh.handleDisconnect)
	mux.HandleFunc("GET /api/metrics/snapshot", mh.handleSnapshot)
	mux.HandleFunc("GET /api/metrics/status", mh.handleStatus)
	mux.HandleFunc("POST /api/metrics/reset", mh.handleReset)

	grpcAddr := os.Getenv("TAP_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":5317"
	}
	httpAddr := os.Getenv("TAP_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":5318"
	}
	ach := &alertCoverageHandler{tapMgr: tapMgr}
	mux.HandleFunc("POST /api/alert-coverage", ach.handleAlertCoverage)

	th := &tapHandler{mgr: tapMgr, defaultGRPCAddr: grpcAddr, defaultHTTPAddr: httpAddr}
	mux.HandleFunc("POST /api/tap/start", th.handleStart)
	mux.HandleFunc("POST /api/tap/stop", th.handleStop)
	mux.HandleFunc("GET /api/tap/status", th.handleStatus)
	mux.HandleFunc("GET /api/tap/catalog", th.handleCatalog)
	mux.HandleFunc("POST /api/tap/reset", th.handleReset)

	if staticHandler != nil {
		mux.Handle("/", staticHandler)
	}

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
