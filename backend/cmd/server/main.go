package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/simskij/signal-studio/internal/api"
	"github.com/simskij/signal-studio/internal/metrics"
	"github.com/simskij/signal-studio/internal/tap"
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
	tapMgr := tap.NewManager()

	// Auto-start the OTLP sampling tap if enabled
	if strings.EqualFold(os.Getenv("TAP_ENABLED"), "true") {
		grpcAddr := os.Getenv("TAP_GRPC_ADDR")
		if grpcAddr == "" {
			grpcAddr = ":4317"
		}
		httpAddr := os.Getenv("TAP_HTTP_ADDR")
		if httpAddr == "" {
			httpAddr = ":4318"
		}
		if err := tapMgr.Start(tap.TapConfig{
			GRPCAddr: grpcAddr,
			HTTPAddr: httpAddr,
		}); err != nil {
			log.Printf("warning: failed to start OTLP tap: %v", err)
		} else {
			log.Printf("OTLP tap listening on gRPC %s, HTTP %s", grpcAddr, httpAddr)
		}
	}

	router := api.NewRouter(mgr, tapMgr)

	log.Printf("signal-studio listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
