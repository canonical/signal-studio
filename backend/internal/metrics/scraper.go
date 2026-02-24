package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// MetricSample is a single metric value with its labels.
type MetricSample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// Snapshot holds all metric samples from a single scrape.
type Snapshot struct {
	CollectedAt time.Time
	Samples     []MetricSample
}

// ScrapeConfig holds connection details for the metrics endpoint.
type ScrapeConfig struct {
	URL   string
	Token string
}

// Scraper fetches and parses Prometheus metrics from a Collector endpoint.
type Scraper struct {
	config ScrapeConfig
	client *http.Client
}

// NewScraper creates a scraper for the given endpoint.
func NewScraper(cfg ScrapeConfig) *Scraper {
	return &Scraper{
		config: cfg,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Scrape fetches metrics from the endpoint and returns a snapshot.
func (s *Scraper) Scrape(ctx context.Context) (*Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	if s.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.Token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from metrics endpoint", resp.StatusCode)
	}

	return parsePrometheusText(resp.Body, time.Now())
}

// parsePrometheusText parses Prometheus text exposition format into a Snapshot.
func parsePrometheusText(r io.Reader, collectedAt time.Time) (*Snapshot, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return nil, fmt.Errorf("parsing metrics: %w", err)
	}

	var samples []MetricSample
	for name, family := range families {
		normalizedName := normalizeMetricName(name)
		for _, m := range family.GetMetric() {
			labels := make(map[string]string)
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			value := extractValue(family.GetType(), m)
			samples = append(samples, MetricSample{
				Name:   normalizedName,
				Labels: labels,
				Value:  value,
			})
		}
	}

	return &Snapshot{
		CollectedAt: collectedAt,
		Samples:     samples,
	}, nil
}

// normalizeMetricName strips the _total suffix that Collector ≥v0.119.0 appends
// to counter names, so lookups are consistent across versions.
func normalizeMetricName(name string) string {
	return strings.TrimSuffix(name, "_total")
}

// extractValue gets the numeric value from a metric based on its type.
func extractValue(t dto.MetricType, m *dto.Metric) float64 {
	switch t {
	case dto.MetricType_COUNTER:
		if c := m.GetCounter(); c != nil {
			return c.GetValue()
		}
	case dto.MetricType_GAUGE:
		if g := m.GetGauge(); g != nil {
			return g.GetValue()
		}
	case dto.MetricType_UNTYPED:
		if u := m.GetUntyped(); u != nil {
			return u.GetValue()
		}
	}
	return 0
}
