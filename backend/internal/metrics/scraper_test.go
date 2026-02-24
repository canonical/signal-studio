package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const samplePrometheusOutput = `# HELP otelcol_receiver_accepted_spans Number of spans accepted.
# TYPE otelcol_receiver_accepted_spans counter
otelcol_receiver_accepted_spans{receiver="otlp",transport="grpc"} 12500
otelcol_receiver_accepted_spans{receiver="otlp",transport="http"} 3200
# HELP otelcol_receiver_accepted_log_records Number of log records accepted.
# TYPE otelcol_receiver_accepted_log_records counter
otelcol_receiver_accepted_log_records{receiver="filelog"} 45000
# HELP otelcol_exporter_sent_spans Number of spans sent.
# TYPE otelcol_exporter_sent_spans counter
otelcol_exporter_sent_spans{exporter="otlp/backend"} 11000
# HELP otelcol_exporter_send_failed_spans Number of spans failed to send.
# TYPE otelcol_exporter_send_failed_spans counter
otelcol_exporter_send_failed_spans{exporter="otlp/backend"} 50
# HELP otelcol_exporter_queue_size Current queue size.
# TYPE otelcol_exporter_queue_size gauge
otelcol_exporter_queue_size{exporter="otlp/backend"} 145
# HELP otelcol_exporter_queue_capacity Queue capacity.
# TYPE otelcol_exporter_queue_capacity gauge
otelcol_exporter_queue_capacity{exporter="otlp/backend"} 1000
`

// Same metrics but with _total suffix (Collector >= v0.119.0)
const samplePrometheusOutputWithTotal = `# HELP otelcol_receiver_accepted_spans_total Number of spans accepted.
# TYPE otelcol_receiver_accepted_spans_total counter
otelcol_receiver_accepted_spans_total{receiver="otlp",transport="grpc"} 12500
# HELP otelcol_exporter_sent_spans_total Number of spans sent.
# TYPE otelcol_exporter_sent_spans_total counter
otelcol_exporter_sent_spans_total{exporter="otlp/backend"} 11000
`

func TestParsePrometheusText(t *testing.T) {
	now := time.Now()
	snap, err := parsePrometheusText(strings.NewReader(samplePrometheusOutput), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snap.CollectedAt != now {
		t.Errorf("collectedAt = %v, want %v", snap.CollectedAt, now)
	}

	// Should have multiple samples
	if len(snap.Samples) == 0 {
		t.Fatal("expected samples, got none")
	}

	// Find receiver accepted spans for otlp/grpc
	found := false
	for _, s := range snap.Samples {
		if s.Name == "otelcol_receiver_accepted_spans" &&
			s.Labels["receiver"] == "otlp" &&
			s.Labels["transport"] == "grpc" {
			found = true
			if s.Value != 12500 {
				t.Errorf("value = %f, want 12500", s.Value)
			}
		}
	}
	if !found {
		t.Error("did not find otelcol_receiver_accepted_spans{receiver=otlp,transport=grpc}")
	}

	// Find queue gauge
	for _, s := range snap.Samples {
		if s.Name == "otelcol_exporter_queue_size" &&
			s.Labels["exporter"] == "otlp/backend" {
			if s.Value != 145 {
				t.Errorf("queue_size = %f, want 145", s.Value)
			}
		}
	}
}

func TestNormalizeMetricName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"otelcol_receiver_accepted_spans_total", "otelcol_receiver_accepted_spans"},
		{"otelcol_receiver_accepted_spans", "otelcol_receiver_accepted_spans"},
		{"otelcol_exporter_queue_size", "otelcol_exporter_queue_size"},
		{"some_counter_total", "some_counter"},
	}
	for _, tc := range tests {
		got := normalizeMetricName(tc.input)
		if got != tc.want {
			t.Errorf("normalizeMetricName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseWithTotalSuffix(t *testing.T) {
	snap, err := parsePrometheusText(strings.NewReader(samplePrometheusOutputWithTotal), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Metric names should be normalized (without _total)
	for _, s := range snap.Samples {
		if strings.HasSuffix(s.Name, "_total") {
			t.Errorf("metric name %q still has _total suffix", s.Name)
		}
	}

	// Should still find by normalized name
	val, ok := lookupValue(snap, "otelcol_receiver_accepted_spans", map[string]string{"receiver": "otlp"})
	if !ok {
		t.Error("could not find otelcol_receiver_accepted_spans after normalization")
	}
	if val != 12500 {
		t.Errorf("value = %f, want 12500", val)
	}
}

func TestScraperWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(samplePrometheusOutput))
	}))
	defer srv.Close()

	scraper := NewScraper(ScrapeConfig{URL: srv.URL})
	snap, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape error: %v", err)
	}

	if len(snap.Samples) == 0 {
		t.Error("expected samples from mock server")
	}
}

func TestScraperWithBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(samplePrometheusOutput))
	}))
	defer srv.Close()

	scraper := NewScraper(ScrapeConfig{URL: srv.URL, Token: "test-token"})
	_, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("scrape error: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
}

func TestScraperNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	scraper := NewScraper(ScrapeConfig{URL: srv.URL})
	_, err := scraper.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}
