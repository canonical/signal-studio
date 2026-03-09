package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/canonical/signal-studio/internal/api"
	"github.com/canonical/signal-studio/internal/metrics"
	"github.com/canonical/signal-studio/internal/tap"
)

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	metricsURL := fs.String("metrics-url", "", "Auto-connect to Collector metrics endpoint on startup")
	_ = fs.String("metrics-token", "", "Bearer token for metrics endpoint")
	_ = fs.String("rules-url", "", "Auto-connect to Prometheus/Mimir rules endpoint on startup")
	_ = fs.String("rules-token", "", "Bearer token for rules endpoint")
	_ = fs.String("rules-org-id", "", "X-Scope-OrgID for Mimir multi-tenant setups")

	if err := fs.Parse(args); err != nil {
		log.Fatalf("invalid flags: %v", err)
	}

	port := os.Getenv("SIGNAL_STUDIO_PORT")
	if port == "" {
		port = "8080"
	}

	scrapeInterval := 10 * time.Second
	if v := os.Getenv("SIGNAL_STUDIO_SCRAPE_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 5 && n <= 30 {
			scrapeInterval = time.Duration(n) * time.Second
		}
	}

	mgr := metrics.NewManager(scrapeInterval)
	tapDisabled := strings.EqualFold(os.Getenv("SIGNAL_STUDIO_TAP_DISABLED"), "true")
	tapMgr := tap.NewManager(tapDisabled)

	if tapDisabled {
		log.Println("OTLP sampling tap disabled via SIGNAL_STUDIO_TAP_DISABLED=true")
	}

	// Auto-connect to metrics endpoint if flag provided.
	if *metricsURL != "" {
		cfg := metrics.ScrapeConfig{URL: *metricsURL}
		if err := mgr.Connect(cfg); err != nil {
			log.Printf("warning: failed to auto-connect metrics: %v", err)
		} else {
			log.Printf("auto-connected to metrics endpoint: %s", *metricsURL)
		}
	}

	router := api.NewRouter(mgr, tapMgr, newStaticHandler())

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: router,
	}

	// Start server in a goroutine.
	go func() {
		log.Printf("signal-studio listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Block until SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	mgr.Disconnect()
	tapMgr.Stop()
	log.Println("shutdown complete")
}
