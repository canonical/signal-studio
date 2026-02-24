package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/simskij/otel-signal-lens/internal/api"
	"github.com/simskij/otel-signal-lens/internal/metrics"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	scrapeInterval := 10 * time.Second
	if v := os.Getenv("SCRAPE_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 5 && n <= 30 {
			scrapeInterval = time.Duration(n) * time.Second
		}
	}

	mgr := metrics.NewManager(scrapeInterval)
	router := api.NewRouter(mgr)

	log.Printf("otel-signal-lens listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
