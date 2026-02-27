package static

import (
	"strings"
	"time"

	"github.com/canonical/signal-studio/internal/config"
)

// networkExporterTypes are exporter types known to support sending_queue.
var networkExporterTypes = map[string]bool{
	"otlp":     true,
	"otlphttp": true,
}

type receiverInterval struct {
	receiver string
	interval time.Duration
}

// collectReceiverIntervals extracts scrape/collection intervals from receivers.
func collectReceiverIntervals(cfg *config.CollectorConfig, receivers []string) []receiverInterval {
	var result []receiverInterval
	for _, name := range receivers {
		recv, ok := cfg.Receivers[name]
		if !ok {
			continue
		}
		recvType := config.ComponentType(name)
		switch recvType {
		case "prometheus":
			for _, d := range prometheusIntervals(recv.Config) {
				result = append(result, receiverInterval{receiver: name, interval: d})
			}
		case "hostmetrics":
			if d := hostMetricsInterval(recv.Config); d > 0 {
				result = append(result, receiverInterval{receiver: name, interval: d})
			}
		}
	}
	return result
}

func prometheusIntervals(raw map[string]any) []time.Duration {
	if raw == nil {
		return nil
	}
	cfgRaw, ok := raw["config"]
	if !ok {
		return nil
	}
	cfgMap, ok := cfgRaw.(map[string]any)
	if !ok {
		return nil
	}
	scrapeConfigsRaw, ok := cfgMap["scrape_configs"]
	if !ok {
		return nil
	}
	scrapeConfigs, ok := scrapeConfigsRaw.([]any)
	if !ok {
		return nil
	}
	var intervals []time.Duration
	for _, scRaw := range scrapeConfigs {
		sc, ok := scRaw.(map[string]any)
		if !ok {
			continue
		}
		s, ok := sc["scrape_interval"].(string)
		if !ok {
			continue
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			continue
		}
		intervals = append(intervals, d)
	}
	return intervals
}

func hostMetricsInterval(raw map[string]any) time.Duration {
	if raw == nil {
		return 0
	}
	s, ok := raw["collection_interval"].(string)
	if !ok {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// extractEndpoints recursively finds endpoint values in a component config.
func extractEndpoints(cfg map[string]any) []string {
	var endpoints []string
	for k, v := range cfg {
		switch val := v.(type) {
		case string:
			if k == "endpoint" {
				endpoints = append(endpoints, val)
			}
		case map[string]any:
			endpoints = append(endpoints, extractEndpoints(val)...)
		}
	}
	return endpoints
}

// hasNestedBool checks if cfg[section][key] equals the expected bool value.
func hasNestedBool(cfg map[string]any, section, key string, expected bool) bool {
	s, ok := cfg[section]
	if !ok {
		return false
	}
	m, ok := s.(map[string]any)
	if !ok {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b == expected
}

func isLocalhostEndpoint(endpoint string) bool {
	// Strip scheme if present
	ep := endpoint
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(ep, prefix) {
			ep = ep[len(prefix):]
			break
		}
	}
	return strings.HasPrefix(ep, "localhost:") ||
		strings.HasPrefix(ep, "localhost/") ||
		ep == "localhost" ||
		strings.HasPrefix(ep, "127.0.0.1:") ||
		strings.HasPrefix(ep, "127.0.0.1/") ||
		ep == "127.0.0.1" ||
		strings.HasPrefix(ep, "::1]") ||
		strings.HasPrefix(ep, "[::1]")
}

// filterDropsHealthSpans checks if a filter processor config contains trace
// span expressions that reference common health check paths.
func filterDropsHealthSpans(procCfg map[string]any, healthPaths []string) bool {
	// Look for traces.span[] OTTL expressions.
	tracesRaw, ok := procCfg["traces"]
	if !ok {
		return false
	}
	tracesMap, ok := tracesRaw.(map[string]any)
	if !ok {
		return false
	}
	spanRaw, ok := tracesMap["span"]
	if !ok {
		return false
	}
	spanList, ok := spanRaw.([]any)
	if !ok {
		return false
	}

	for _, expr := range spanList {
		s, ok := expr.(string)
		if !ok {
			continue
		}
		lower := strings.ToLower(s)
		for _, hp := range healthPaths {
			if strings.Contains(lower, hp) {
				return true
			}
		}
	}
	return false
}
