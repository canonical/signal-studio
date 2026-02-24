package rules

import (
	"fmt"
	"strings"

	"github.com/simskij/signal-studio/internal/config"
)

// Rule 1: Missing memory_limiter processor

type MissingMemoryLimiter struct{}

func (r *MissingMemoryLimiter) ID() string { return "missing-memory-limiter" }

func (r *MissingMemoryLimiter) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		if hasProcessorType(p.Processors, "memory_limiter") {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Missing memory_limiter in %s pipeline", name),
			Severity:     SeverityCritical,
			Evidence:     fmt.Sprintf("Pipeline %q processors: [%s]", name, strings.Join(p.Processors, ", ")),
			Explanation:  "The memory_limiter processor is not present in this pipeline.",
			WhyItMatters: "Without memory_limiter, the collector can experience uncontrolled memory growth and OOM kills under load.",
			Impact:       "Prevents OOM crashes and provides backpressure when memory usage is high.",
			Snippet: `processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128`,
			Placement: "Add memory_limiter as the first processor in the pipeline.",
			Pipeline:  name,
		})
	}
	return findings
}

// Rule 2: Missing batch processor

type MissingBatch struct{}

func (r *MissingBatch) ID() string { return "missing-batch" }

func (r *MissingBatch) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		if hasProcessorType(p.Processors, "batch") {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Missing batch processor in %s pipeline", name),
			Severity:     SeverityWarning,
			Evidence:     fmt.Sprintf("Pipeline %q processors: [%s]", name, strings.Join(p.Processors, ", ")),
			Explanation:  "The batch processor is not present in this pipeline.",
			WhyItMatters: "Batching reduces exporter overhead and improves throughput stability by grouping telemetry before export.",
			Impact:       "Reduces number of export requests and improves throughput.",
			Snippet: `processors:
  batch:
    timeout: 5s
    send_batch_size: 512`,
			Placement: "Add batch as the last processor in the pipeline (after memory_limiter and any filtering).",
			Pipeline:  name,
		})
	}
	return findings
}

// Rule 3: No trace sampling configured

type NoTraceSampling struct{}

func (r *NoTraceSampling) ID() string { return "no-trace-sampling" }

func (r *NoTraceSampling) Evaluate(cfg *config.CollectorConfig) []Finding {
	samplingProcessors := []string{
		"probabilistic_sampler",
		"tail_sampling",
	}

	var findings []Finding
	for name, p := range cfg.Pipelines {
		if p.Signal != config.SignalTraces {
			continue
		}
		hasSampling := false
		for _, proc := range p.Processors {
			procType := config.ComponentType(proc)
			for _, sp := range samplingProcessors {
				if procType == sp {
					hasSampling = true
					break
				}
			}
		}
		if hasSampling {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("No sampling configured in %s pipeline", name),
			Severity:     SeverityWarning,
			Evidence:     fmt.Sprintf("Traces pipeline %q has no sampling processor. Processors: [%s]", name, strings.Join(p.Processors, ", ")),
			Explanation:  "No probabilistic or tail sampling processor was found in this traces pipeline.",
			WhyItMatters: "High trace volume is a primary cost driver. Sampling reduces volume while preserving representative data.",
			Impact:       "Probabilistic sampling at 20% would reduce trace export volume by ~80%.",
			Snippet: `processors:
  probabilistic_sampler:
    sampling_percentage: 20`,
			Placement: "Add after memory_limiter but before batch processor.",
			Pipeline:  name,
		})
	}
	return findings
}

// Rule 8: Unused components

type UnusedComponents struct{}

func (r *UnusedComponents) ID() string { return "unused-components" }

func (r *UnusedComponents) Evaluate(cfg *config.CollectorConfig) []Finding {
	usedReceivers := make(map[string]bool)
	usedProcessors := make(map[string]bool)
	usedExporters := make(map[string]bool)

	for _, p := range cfg.Pipelines {
		for _, r := range p.Receivers {
			usedReceivers[r] = true
		}
		for _, proc := range p.Processors {
			usedProcessors[proc] = true
		}
		for _, e := range p.Exporters {
			usedExporters[e] = true
		}
	}

	var findings []Finding

	for name := range cfg.Receivers {
		if !usedReceivers[name] {
			findings = append(findings, unusedFinding(r.ID(), "receiver", name))
		}
	}
	for name := range cfg.Processors {
		if !usedProcessors[name] {
			findings = append(findings, unusedFinding(r.ID(), "processor", name))
		}
	}
	for name := range cfg.Exporters {
		if !usedExporters[name] {
			findings = append(findings, unusedFinding(r.ID(), "exporter", name))
		}
	}

	return findings
}

func unusedFinding(ruleID, kind, name string) Finding {
	return Finding{
		RuleID:       ruleID,
		Title:        fmt.Sprintf("Unused %s: %s", kind, name),
		Severity:     SeverityInfo,
		Evidence:     fmt.Sprintf("%s %q is defined but not referenced by any pipeline.", kind, name),
		Explanation:  fmt.Sprintf("The %s %q is configured but no pipeline uses it.", kind, name),
		WhyItMatters: "Unused components add confusion and increase configuration drift over time.",
		Impact:       "Removing unused components simplifies the configuration.",
		Snippet:      fmt.Sprintf("# Remove the unused %s:\n# %s: ...", kind, name),
		Placement:    fmt.Sprintf("Remove the %q block from the %ss section.", name, kind),
	}
}

// Rule 9: Multiple exporters without routing clarity

type MultipleExportersNoRouting struct{}

func (r *MultipleExportersNoRouting) ID() string { return "multiple-exporters-no-routing" }

func (r *MultipleExportersNoRouting) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		if len(p.Exporters) < 2 {
			continue
		}
		if hasRoutingConnector(cfg, p.Exporters) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Title:    fmt.Sprintf("Multiple exporters without routing in %s pipeline", name),
			Severity: SeverityInfo,
			Evidence: fmt.Sprintf("Pipeline %q has %d exporters [%s] but no routing connector.",
				name, len(p.Exporters), strings.Join(p.Exporters, ", ")),
			Explanation:  "Multiple exporters receive identical data without a routing connector to direct traffic.",
			WhyItMatters: "This can unintentionally duplicate telemetry, increase cost, and complicate troubleshooting.",
			Impact:       "Adding a routing connector or splitting into separate pipelines gives explicit control over data flow.",
			Snippet: `connectors:
  routing:
    match_once: true
    default_pipelines: [traces/primary]
    table:
      - condition: attributes["env"] == "production"
        pipelines: [traces/secondary]

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [routing]
    traces/primary:
      receivers: [routing]
      exporters: [otlp/primary]
    traces/secondary:
      receivers: [routing]
      exporters: [otlp/secondary]`,
			Placement: "Replace the multi-exporter pipeline with a routing connector that fans out to sub-pipelines.",
			Pipeline:  name,
		})
	}
	return findings
}

// hasRoutingConnector checks if any exporter in the list is a routing connector.
func hasRoutingConnector(cfg *config.CollectorConfig, exporters []string) bool {
	for _, exp := range exporters {
		if config.ComponentType(exp) == "routing" {
			if _, ok := cfg.Connectors[exp]; ok {
				return true
			}
		}
	}
	return false
}

// Rule 10: No log severity filtering

type NoLogSeverityFilter struct{}

func (r *NoLogSeverityFilter) ID() string { return "no-log-severity-filter" }

func (r *NoLogSeverityFilter) Evaluate(cfg *config.CollectorConfig) []Finding {
	filterTypes := []string{"filter", "transform"}

	var findings []Finding
	for name, p := range cfg.Pipelines {
		if p.Signal != config.SignalLogs {
			continue
		}
		hasFilter := false
		for _, proc := range p.Processors {
			procType := config.ComponentType(proc)
			for _, ft := range filterTypes {
				if procType == ft {
					hasFilter = true
					break
				}
			}
		}
		if hasFilter {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("No log severity filtering in %s pipeline", name),
			Severity:     SeverityInfo,
			Evidence:     fmt.Sprintf("Logs pipeline %q has no filter or transform processor. Processors: [%s]", name, strings.Join(p.Processors, ", ")),
			Explanation:  "No filter or transform processor was found to control log severity levels.",
			WhyItMatters: "DEBUG and INFO log floods are common cost drivers. Filtering by severity reduces volume significantly.",
			Impact:       "Filtering out DEBUG logs typically reduces log volume by 30-60%.",
			Snippet: `processors:
  filter/severity:
    error_mode: ignore
    logs:
      log_record:
        - 'severity_number < SEVERITY_NUMBER_INFO'`,
			Placement: "Add after memory_limiter, before batch processor.",
			Pipeline:  name,
		})
	}
	return findings
}

// hasProcessorType checks if any processor in the list matches the given type.
func hasProcessorType(processors []string, typeName string) bool {
	for _, p := range processors {
		if config.ComponentType(p) == typeName {
			return true
		}
	}
	return false
}
