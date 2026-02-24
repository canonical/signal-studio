package api

import (
	"net/http"
	"os"
	"strings"
)

// NewRouter creates the HTTP handler with all routes.
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/config/analyze", handleAnalyzeConfig)
	mux.HandleFunc("GET /api/health", handleHealth)

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
