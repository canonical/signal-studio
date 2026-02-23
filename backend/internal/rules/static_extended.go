package rules

import (
	"fmt"
	"strings"

	"github.com/simskij/otel-signal-lens/internal/config"
)

// R11: memory_limiter not first processor

type MemoryLimiterNotFirst struct{}

func (r *MemoryLimiterNotFirst) ID() string { return "memory-limiter-not-first" }

func (r *MemoryLimiterNotFirst) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		if len(p.Processors) == 0 {
			continue
		}
		// Find index of memory_limiter
		idx := -1
		for i, proc := range p.Processors {
			if config.ComponentType(proc) == "memory_limiter" {
				idx = i
				break
			}
		}
		if idx <= 0 { // not found (-1) or already first (0)
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("memory_limiter is not the first processor in %s pipeline", name),
			Severity:     SeverityCritical,
			Evidence:     fmt.Sprintf("Pipeline %q processors: [%s] — memory_limiter is at index %d.", name, strings.Join(p.Processors, ", "), idx),
			Explanation:  "The memory_limiter processor must be the first processor in the pipeline.",
			WhyItMatters: "If other processors (like batch) come first, data accumulates in memory before the limiter can apply backpressure, leading to OOM crashes.",
			Impact:       "Moving memory_limiter to the first position ensures backpressure is applied before any buffering occurs.",
			Snippet: fmt.Sprintf(`service:
  pipelines:
    %s:
      processors: [memory_limiter, ...]`, name),
			Placement: "Move memory_limiter to the first position in the processors list.",
			Pipeline:  name,
		})
	}
	return findings
}

// R12: batch processor before sampling

type BatchBeforeSampling struct{}

func (r *BatchBeforeSampling) ID() string { return "batch-before-sampling" }

func (r *BatchBeforeSampling) Evaluate(cfg *config.CollectorConfig) []Finding {
	samplers := map[string]bool{
		"tail_sampling":        true,
		"probabilistic_sampler": true,
	}
	var findings []Finding
	for name, p := range cfg.Pipelines {
		batchIdx := -1
		firstSamplerIdx := -1
		for i, proc := range p.Processors {
			pt := config.ComponentType(proc)
			if pt == "batch" && batchIdx == -1 {
				batchIdx = i
			}
			if samplers[pt] && firstSamplerIdx == -1 {
				firstSamplerIdx = i
			}
		}
		if batchIdx == -1 || firstSamplerIdx == -1 || batchIdx > firstSamplerIdx {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Batch processor before sampling in %s pipeline", name),
			Severity:     SeverityCritical,
			Evidence:     fmt.Sprintf("Pipeline %q processors: [%s] — batch at index %d, sampler at index %d.", name, strings.Join(p.Processors, ", "), batchIdx, firstSamplerIdx),
			Explanation:  "The batch processor appears before a sampling processor in the pipeline.",
			WhyItMatters: "Batch can split spans from the same trace into different batches, causing the sampler to see incomplete traces and make incorrect decisions.",
			Impact:       "Moving batch after sampling ensures trace-complete sampling decisions.",
			Snippet: fmt.Sprintf(`service:
  pipelines:
    %s:
      processors: [memory_limiter, tail_sampling, batch]`, name),
			Placement: "Move batch processor after all sampling processors.",
			Pipeline:  name,
		})
	}
	return findings
}

// R13: batch processor not near end of pipeline

type BatchNotNearEnd struct{}

func (r *BatchNotNearEnd) ID() string { return "batch-not-near-end" }

func (r *BatchNotNearEnd) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		if len(p.Processors) < 2 {
			continue
		}
		batchIdx := -1
		for i, proc := range p.Processors {
			if config.ComponentType(proc) == "batch" {
				batchIdx = i
				break
			}
		}
		if batchIdx == -1 {
			continue
		}
		// Allow batch at last or second-to-last position
		if batchIdx >= len(p.Processors)-2 {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Batch processor not near end of %s pipeline", name),
			Severity:     SeverityInfo,
			Evidence:     fmt.Sprintf("Pipeline %q processors: [%s] — batch at index %d of %d.", name, strings.Join(p.Processors, ", "), batchIdx, len(p.Processors)),
			Explanation:  "The batch processor is not near the end of the pipeline.",
			WhyItMatters: "Processors after batch operate on individual items, negating the batching benefit for compression and connection efficiency.",
			Impact:       "Moving batch to the end of the pipeline optimizes export efficiency.",
			Snippet:      "# Move batch to the last (or second-to-last) position in the processors list.",
			Placement:    "Move batch to the end of the pipeline, after all filtering and transformation processors.",
			Pipeline:     name,
		})
	}
	return findings
}

// R14: Receiver endpoint bound to 0.0.0.0

type ReceiverEndpointWildcard struct{}

func (r *ReceiverEndpointWildcard) ID() string { return "receiver-endpoint-wildcard" }

func (r *ReceiverEndpointWildcard) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, comp := range cfg.Receivers {
		endpoints := extractEndpoints(comp.Config)
		for _, ep := range endpoints {
			if strings.Contains(ep, "0.0.0.0") {
				findings = append(findings, Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Receiver %s binds to 0.0.0.0", name),
					Severity:     SeverityWarning,
					Evidence:     fmt.Sprintf("Receiver %q has endpoint %q.", name, ep),
					Explanation:  "This receiver is bound to all network interfaces.",
					WhyItMatters: "Binding to 0.0.0.0 exposes the receiver to untrusted networks, increasing the attack surface for DoS and data injection.",
					Impact:       "Binding to localhost or a specific interface restricts access to trusted sources.",
					Snippet: fmt.Sprintf(`receivers:
  %s:
    endpoint: localhost:4317`, name),
					Placement: "Change the endpoint to localhost or a specific trusted interface.",
				})
			}
		}
	}
	return findings
}

// R15: Debug exporter in pipeline

type DebugExporterInPipeline struct{}

func (r *DebugExporterInPipeline) ID() string { return "debug-exporter-in-pipeline" }

func (r *DebugExporterInPipeline) Evaluate(cfg *config.CollectorConfig) []Finding {
	debugExporters := map[string]bool{"debug": true, "logging": true}
	var findings []Finding
	for name, p := range cfg.Pipelines {
		for _, exp := range p.Exporters {
			if debugExporters[config.ComponentType(exp)] {
				findings = append(findings, Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Debug exporter %s in %s pipeline", exp, name),
					Severity:     SeverityWarning,
					Evidence:     fmt.Sprintf("Pipeline %q includes exporter %q.", name, exp),
					Explanation:  "A debug/logging exporter is wired into this pipeline.",
					WhyItMatters: "The debug exporter prints all telemetry to stdout, causing performance overhead in production and potentially exposing sensitive data in log files.",
					Impact:       "Removing the debug exporter reduces I/O overhead and prevents sensitive data leakage.",
					Snippet:      fmt.Sprintf("# Remove %q from the exporters list in the %s pipeline.", exp, name),
					Placement:    "Remove the debug exporter from production pipeline configurations.",
					Pipeline:     name,
				})
			}
		}
	}
	return findings
}

// R18: pprof extension enabled

type PprofExtensionEnabled struct{}

func (r *PprofExtensionEnabled) ID() string { return "pprof-extension-enabled" }

func (r *PprofExtensionEnabled) Evaluate(cfg *config.CollectorConfig) []Finding {
	for _, ext := range cfg.ServiceExtensions {
		if config.ComponentType(ext) == "pprof" {
			return []Finding{{
				RuleID:       r.ID(),
				Title:        "pprof extension is enabled",
				Severity:     SeverityInfo,
				Evidence:     fmt.Sprintf("Service extensions include %q.", ext),
				Explanation:  "The pprof extension exposes Go runtime profiling data.",
				WhyItMatters: "In production, pprof can be used to gather operational intelligence or trigger expensive profiling that degrades performance.",
				Impact:       "Disabling pprof in production reduces the attack surface.",
				Snippet: `# Remove pprof from service extensions:
service:
  extensions: [health_check]`,
				Placement: "Remove pprof from the service extensions list in production.",
			}}
		}
	}
	return nil
}

// R19: memory_limiter without limit_mib or limit_percentage

type MemoryLimiterWithoutLimits struct{}

func (r *MemoryLimiterWithoutLimits) ID() string { return "memory-limiter-without-limits" }

func (r *MemoryLimiterWithoutLimits) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, comp := range cfg.Processors {
		if config.ComponentType(name) != "memory_limiter" {
			continue
		}
		_, hasLimitMib := comp.Config["limit_mib"]
		_, hasLimitPct := comp.Config["limit_percentage"]
		if hasLimitMib || hasLimitPct {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("memory_limiter %s has no limit configured", name),
			Severity:     SeverityCritical,
			Evidence:     fmt.Sprintf("Processor %q has neither limit_mib nor limit_percentage set.", name),
			Explanation:  "The memory_limiter processor has no memory limit configured.",
			WhyItMatters: "Without limit_mib or limit_percentage, the memory limiter does nothing, creating a false sense of security.",
			Impact:       "Setting a limit enables actual memory protection and backpressure.",
			Snippet: fmt.Sprintf(`processors:
  %s:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128`, name),
			Placement: "Add limit_mib or limit_percentage to the memory_limiter configuration.",
		})
	}
	return findings
}

// R20: Exporter sending_queue not enabled

type ExporterNoSendingQueue struct{}

func (r *ExporterNoSendingQueue) ID() string { return "exporter-no-sending-queue" }

// networkExporterTypes are exporter types known to support sending_queue.
var networkExporterTypes = map[string]bool{
	"otlp":     true,
	"otlphttp": true,
}

func (r *ExporterNoSendingQueue) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if hasNestedBool(comp.Config, "sending_queue", "enabled", true) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s has no sending queue enabled", name),
			Severity:     SeverityWarning,
			Evidence:     fmt.Sprintf("Exporter %q does not have sending_queue.enabled: true.", name),
			Explanation:  "The exporter does not have a sending queue configured.",
			WhyItMatters: "Without a sending queue, transient backend unavailability causes immediate data loss. The queue buffers data during failures.",
			Impact:       "Enabling sending_queue provides resilience against transient backend failures.",
			Snippet: fmt.Sprintf(`exporters:
  %s:
    sending_queue:
      enabled: true
      queue_size: 5000`, name),
			Placement: "Add sending_queue configuration to the exporter.",
		})
	}
	return findings
}

// R21: Exporter retry_on_failure not enabled

type ExporterNoRetry struct{}

func (r *ExporterNoRetry) ID() string { return "exporter-no-retry" }

func (r *ExporterNoRetry) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if hasNestedBool(comp.Config, "retry_on_failure", "enabled", true) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s has no retry on failure enabled", name),
			Severity:     SeverityWarning,
			Evidence:     fmt.Sprintf("Exporter %q does not have retry_on_failure.enabled: true.", name),
			Explanation:  "The exporter does not have retry logic configured.",
			WhyItMatters: "Without retry, any transient network error causes permanent data loss for the affected batch.",
			Impact:       "Enabling retry_on_failure with exponential backoff recovers from transient failures.",
			Snippet: fmt.Sprintf(`exporters:
  %s:
    retry_on_failure:
      enabled: true
      max_elapsed_time: 300s`, name),
			Placement: "Add retry_on_failure configuration to the exporter.",
		})
	}
	return findings
}

// R22: Pipeline references undefined component

type UndefinedComponentRef struct{}

func (r *UndefinedComponentRef) ID() string { return "undefined-component-ref" }

func (r *UndefinedComponentRef) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		for _, rcv := range p.Receivers {
			if _, ok := cfg.Receivers[rcv]; !ok {
				findings = append(findings, Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined receiver %s in %s pipeline", rcv, name),
					Severity:     SeverityCritical,
					Evidence:     fmt.Sprintf("Pipeline %q references receiver %q which is not defined.", name, rcv),
					Explanation:  "This pipeline references a receiver that does not exist in the receivers section.",
					WhyItMatters: "The collector will fail to start with an undefined component reference.",
					Impact:       "Define the receiver or remove it from the pipeline.",
					Snippet:      fmt.Sprintf("receivers:\n  %s:\n    # Add configuration here", rcv),
					Placement:    "Add the missing receiver to the receivers section or remove it from the pipeline.",
					Pipeline:     name,
				})
			}
		}
		for _, proc := range p.Processors {
			if _, ok := cfg.Processors[proc]; !ok {
				findings = append(findings, Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined processor %s in %s pipeline", proc, name),
					Severity:     SeverityCritical,
					Evidence:     fmt.Sprintf("Pipeline %q references processor %q which is not defined.", name, proc),
					Explanation:  "This pipeline references a processor that does not exist in the processors section.",
					WhyItMatters: "The collector will fail to start with an undefined component reference.",
					Impact:       "Define the processor or remove it from the pipeline.",
					Snippet:      fmt.Sprintf("processors:\n  %s:\n    # Add configuration here", proc),
					Placement:    "Add the missing processor to the processors section or remove it from the pipeline.",
					Pipeline:     name,
				})
			}
		}
		for _, exp := range p.Exporters {
			if _, ok := cfg.Exporters[exp]; !ok {
				findings = append(findings, Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined exporter %s in %s pipeline", exp, name),
					Severity:     SeverityCritical,
					Evidence:     fmt.Sprintf("Pipeline %q references exporter %q which is not defined.", name, exp),
					Explanation:  "This pipeline references an exporter that does not exist in the exporters section.",
					WhyItMatters: "The collector will fail to start with an undefined component reference.",
					Impact:       "Define the exporter or remove it from the pipeline.",
					Snippet:      fmt.Sprintf("exporters:\n  %s:\n    # Add configuration here", exp),
					Placement:    "Add the missing exporter to the exporters section or remove it from the pipeline.",
					Pipeline:     name,
				})
			}
		}
	}
	return findings
}

// R23: Empty pipeline

type EmptyPipeline struct{}

func (r *EmptyPipeline) ID() string { return "empty-pipeline" }

func (r *EmptyPipeline) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		if len(p.Receivers) == 0 {
			findings = append(findings, Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Pipeline %s has no receivers", name),
				Severity:     SeverityCritical,
				Evidence:     fmt.Sprintf("Pipeline %q has an empty receivers list.", name),
				Explanation:  "This pipeline has no receivers configured.",
				WhyItMatters: "A pipeline without receivers will never receive any data.",
				Impact:       "Add at least one receiver to make this pipeline functional.",
				Snippet:      fmt.Sprintf("service:\n  pipelines:\n    %s:\n      receivers: [otlp]", name),
				Placement:    "Add receivers to the pipeline.",
				Pipeline:     name,
			})
		}
		if len(p.Exporters) == 0 {
			findings = append(findings, Finding{
				RuleID:       r.ID(),
				Title:        fmt.Sprintf("Pipeline %s has no exporters", name),
				Severity:     SeverityCritical,
				Evidence:     fmt.Sprintf("Pipeline %q has an empty exporters list.", name),
				Explanation:  "This pipeline has no exporters configured.",
				WhyItMatters: "A pipeline without exporters receives and processes data but silently drops it all.",
				Impact:       "Add at least one exporter to prevent silent data loss.",
				Snippet:      fmt.Sprintf("service:\n  pipelines:\n    %s:\n      exporters: [otlp]", name),
				Placement:    "Add exporters to the pipeline.",
				Pipeline:     name,
			})
		}
	}
	return findings
}

// R24: filter/transform processor with error_mode propagate

type FilterErrorModePropagateRule struct{}

func (r *FilterErrorModePropagateRule) ID() string { return "filter-error-mode-propagate" }

func (r *FilterErrorModePropagateRule) Evaluate(cfg *config.CollectorConfig) []Finding {
	riskyTypes := map[string]bool{"filter": true, "transform": true}
	var findings []Finding
	for name, comp := range cfg.Processors {
		if !riskyTypes[comp.Type] {
			continue
		}
		// Check if error_mode is set to something other than propagate
		if em, ok := comp.Config["error_mode"]; ok {
			if s, ok := em.(string); ok && s != "propagate" {
				continue
			}
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Processor %s uses error_mode propagate", name),
			Severity:     SeverityWarning,
			Evidence:     fmt.Sprintf("Processor %q has error_mode set to propagate (or unset, which defaults to propagate).", name),
			Explanation:  "When error_mode is propagate (the default), any OTTL condition evaluation error drops the entire payload.",
			WhyItMatters: "A single typo in a filter condition can cause 100%% data loss. Setting error_mode to ignore logs the error and continues processing.",
			Impact:       "Changing to error_mode: ignore prevents silent total data loss from condition errors.",
			Snippet: fmt.Sprintf(`processors:
  %s:
    error_mode: ignore`, name),
			Placement: "Add error_mode: ignore to the processor configuration.",
		})
	}
	return findings
}

// --- Helpers ---

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
