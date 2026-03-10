package static

import (
	"fmt"
	"time"

	"github.com/canonical/signal-studio/internal/config"
	"github.com/canonical/signal-studio/internal/rules"
)

// ShortScrapeInterval fires when a receiver uses a sub-minute scrape/collection interval.
type ShortScrapeInterval struct{}

func (r *ShortScrapeInterval) ID() string { return "short-scrape-interval" }

func (r *ShortScrapeInterval) Description() string {
	return "Receiver uses a sub-minute scrape or collection interval"
}

func (r *ShortScrapeInterval) DefaultSeverity() rules.Severity { return rules.SeverityInfo }

func (r *ShortScrapeInterval) Evaluate(cfg *config.CollectorConfig) []rules.Finding {
	var findings []rules.Finding

	for name, recv := range cfg.Receivers {
		recvType := config.ComponentType(name)

		switch recvType {
		case "prometheus":
			findings = append(findings, checkPrometheusScrapeIntervals(r.ID(), name, recv.Config)...)
		case "hostmetrics":
			findings = append(findings, checkHostMetricsInterval(r.ID(), name, recv.Config)...)
		}
	}

	return findings
}

func checkPrometheusScrapeIntervals(ruleID, name string, raw map[string]any) []rules.Finding {
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

	var findings []rules.Finding
	for _, scRaw := range scrapeConfigs {
		sc, ok := scRaw.(map[string]any)
		if !ok {
			continue
		}
		intervalStr, ok := sc["scrape_interval"].(string)
		if !ok {
			continue
		}
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			continue
		}
		if d < 60*time.Second {
			jobName, _ := sc["job_name"].(string)
			evidence := fmt.Sprintf("Receiver %q", name)
			if jobName != "" {
				evidence = fmt.Sprintf("Receiver %q job %q", name, jobName)
			}
			findings = append(findings, rules.Finding{
				RuleID:     ruleID,
				Title:      fmt.Sprintf("Sub-minute scrape interval on %s", name),
				Severity:   rules.SeverityInfo,
				Confidence: rules.ConfidenceHigh,
				Evidence:   fmt.Sprintf("%s has scrape_interval: %s", evidence, intervalStr),
				Implication: "Sub-minute intervals should be used sparingly; unless the service is critical and operates in bursts, it will not provide additional value. Longer scrape intervals reduce metric volume and collector load." +
					"\nHowever, short intervals are appropriate for metrics that change rapidly and require high-resolution monitoring.",
				Scope: fmt.Sprintf("receiver:%s", name),
				Snippet: fmt.Sprintf(`receivers:
  %s:
    config:
      scrape_configs:
        - scrape_interval: 60s`, name),
				Recommendation: "Increase the scrape interval to 60s or longer unless sub-minute resolution is required.",
			})
		}
	}
	return findings
}

func checkHostMetricsInterval(ruleID, name string, raw map[string]any) []rules.Finding {
	if raw == nil {
		return nil
	}
	intervalStr, ok := raw["collection_interval"].(string)
	if !ok {
		return nil
	}
	d, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil
	}
	if d >= 60*time.Second {
		return nil
	}

	return []rules.Finding{{
		RuleID:     ruleID,
		Title:      fmt.Sprintf("Sub-minute collection interval on %s", name),
		Severity:   rules.SeverityInfo,
		Confidence: rules.ConfidenceHigh,
		Evidence:   fmt.Sprintf("Receiver %q has collection_interval: %s", name, intervalStr),
		Implication: "Sub-minute intervals should be used sparingly; unless the service is critical and operates in bursts, it will not provide additional value. Longer collection intervals reduce metric volume and collector load." +
			"\nHowever, short intervals are appropriate for metrics that change rapidly and require high-resolution monitoring.",
		Scope: fmt.Sprintf("receiver:%s", name),
		Snippet: fmt.Sprintf(`receivers:
  %s:
    collection_interval: 60s`, name),
		Recommendation: "Increase the collection interval to 60s or longer unless sub-minute resolution is required.",
	}}
}
