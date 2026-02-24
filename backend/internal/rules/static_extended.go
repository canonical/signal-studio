package rules

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/simskij/signal-studio/internal/config"
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
			_, inReceivers := cfg.Receivers[rcv]
			_, inConnectors := cfg.Connectors[rcv]
			if !inReceivers && !inConnectors {
				findings = append(findings, Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined receiver %s in %s pipeline", rcv, name),
					Severity:     SeverityCritical,
					Evidence:     fmt.Sprintf("Pipeline %q references receiver %q which is not defined in receivers or connectors.", name, rcv),
					Explanation:  "This pipeline references a receiver that does not exist in the receivers or connectors section.",
					WhyItMatters: "The collector will fail to start with an undefined component reference.",
					Impact:       "Define the receiver/connector or remove it from the pipeline.",
					Snippet:      fmt.Sprintf("receivers:\n  %s:\n    # Add configuration here", rcv),
					Placement:    "Add the missing receiver to the receivers or connectors section, or remove it from the pipeline.",
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
			_, inExporters := cfg.Exporters[exp]
			_, inConnectors := cfg.Connectors[exp]
			if !inExporters && !inConnectors {
				findings = append(findings, Finding{
					RuleID:       r.ID(),
					Title:        fmt.Sprintf("Undefined exporter %s in %s pipeline", exp, name),
					Severity:     SeverityCritical,
					Evidence:     fmt.Sprintf("Pipeline %q references exporter %q which is not defined in exporters or connectors.", name, exp),
					Explanation:  "This pipeline references an exporter that does not exist in the exporters or connectors section.",
					WhyItMatters: "The collector will fail to start with an undefined component reference.",
					Impact:       "Define the exporter/connector or remove it from the pipeline.",
					Snippet:      fmt.Sprintf("exporters:\n  %s:\n    # Add configuration here", exp),
					Placement:    "Add the missing exporter to the exporters or connectors section, or remove it from the pipeline.",
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

// ScrapeIntervalMismatch fires when receivers in the same metrics pipeline use
// different scrape or collection intervals, causing uneven data density.
type ScrapeIntervalMismatch struct{}

func (r *ScrapeIntervalMismatch) ID() string { return "scrape-interval-mismatch" }

func (r *ScrapeIntervalMismatch) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for pName, p := range cfg.Pipelines {
		if p.Signal != config.SignalMetrics {
			continue
		}
		intervals := collectReceiverIntervals(cfg, p.Receivers)
		if len(intervals) < 2 {
			continue
		}

		// Deduplicate
		unique := map[time.Duration]bool{}
		for _, iv := range intervals {
			unique[iv.interval] = true
		}
		if len(unique) < 2 {
			continue
		}

		sort.Slice(intervals, func(i, j int) bool {
			return intervals[i].interval < intervals[j].interval
		})
		var parts []string
		for _, iv := range intervals {
			parts = append(parts, fmt.Sprintf("%s (%s)", iv.receiver, iv.interval))
		}
		shortest := intervals[0]
		longest := intervals[len(intervals)-1]

		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Title:    fmt.Sprintf("Mismatched scrape intervals in %s pipeline", pName),
			Severity: SeverityWarning,
			Evidence: fmt.Sprintf("Receivers have different intervals: %s", strings.Join(parts, ", ")),
			Explanation: fmt.Sprintf(
				"Receiver %s collects every %s while %s collects every %s. "+
					"Different intervals in the same pipeline produce uneven data density.",
				shortest.receiver, shortest.interval, longest.receiver, longest.interval),
			WhyItMatters: "Mismatched intervals cause inconsistent granularity on dashboards and can trigger misleading alerts when data from different receivers is correlated.",
			Impact:       "Aligning intervals makes data easier to reason about and avoids sparse-series artifacts.",
			Snippet: fmt.Sprintf(`receivers:
  %s:
    collection_interval: %s`, longest.receiver, shortest.interval),
			Placement: "Align collection/scrape intervals across receivers in the same pipeline.",
			Pipeline:  pName,
		})
	}
	return findings
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

// ExporterInsecureTLS fires when an exporter has tls.insecure: true, sending
// telemetry data unencrypted over the network.
type ExporterInsecureTLS struct{}

func (r *ExporterInsecureTLS) ID() string { return "exporter-insecure-tls" }

func (r *ExporterInsecureTLS) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if !hasNestedBool(comp.Config, "tls", "insecure", true) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s uses insecure TLS", name),
			Severity:     SeverityCritical,
			Evidence:     fmt.Sprintf("Exporter %q has tls.insecure: true.", name),
			Explanation:  "This exporter sends telemetry data without TLS encryption.",
			WhyItMatters: "Unencrypted telemetry may contain sensitive information such as HTTP headers, database queries, or user identifiers. Transmitting this in plaintext exposes it to network interception.",
			Impact:       "Enabling TLS encrypts data in transit and prevents eavesdropping.",
			Snippet: fmt.Sprintf(`exporters:
  %s:
    tls:
      insecure: false
      # cert_file: /path/to/cert.pem
      # key_file: /path/to/key.pem`, name),
			Placement: "Remove tls.insecure or set it to false, and configure proper TLS certificates.",
		})
	}
	return findings
}

// NoHealthCheckExtension fires when no health_check extension is configured.
type NoHealthCheckExtension struct{}

func (r *NoHealthCheckExtension) ID() string { return "no-health-check-extension" }

func (r *NoHealthCheckExtension) Evaluate(cfg *config.CollectorConfig) []Finding {
	for name := range cfg.Extensions {
		if config.ComponentType(name) == "health_check" {
			return nil
		}
	}
	// Also check if referenced in service extensions
	for _, ext := range cfg.ServiceExtensions {
		if config.ComponentType(ext) == "health_check" {
			return nil
		}
	}
	return []Finding{{
		RuleID:       r.ID(),
		Title:        "No health check extension configured",
		Severity:     SeverityWarning,
		Evidence:     "No health_check extension found in the configuration.",
		Explanation:  "The collector has no health check endpoint for liveness and readiness probes.",
		WhyItMatters: "Without a health check, orchestrators like Kubernetes cannot detect collector failures, leading to silent data loss as traffic continues to route to an unhealthy instance.",
		Impact:       "Adding a health check extension enables automated failure detection and recovery.",
		Snippet: `extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]`,
		Placement: "Add a health_check extension and reference it in the service extensions list.",
	}}
}

// ExporterEndpointLocalhost fires when a network exporter points to localhost,
// which is almost always a development leftover.
type ExporterEndpointLocalhost struct{}

func (r *ExporterEndpointLocalhost) ID() string { return "exporter-endpoint-localhost" }

func (r *ExporterEndpointLocalhost) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if comp.Config == nil {
			continue
		}
		endpoint, ok := comp.Config["endpoint"].(string)
		if !ok {
			continue
		}
		if !isLocalhostEndpoint(endpoint) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s points to localhost", name),
			Severity:     SeverityInfo,
			Evidence:     fmt.Sprintf("Exporter %q has endpoint: %s", name, endpoint),
			Explanation:  "This exporter sends data to localhost, which is typically a leftover from development.",
			WhyItMatters: "In production, the exporter should point to the actual backend address. Sending to localhost means telemetry data is either lost or never leaves the host.",
			Impact:       "Update the endpoint to the production backend address.",
			Snippet: fmt.Sprintf(`exporters:
  %s:
    endpoint: <backend-host>:4317`, name),
			Placement: "Replace the localhost endpoint with the production backend address.",
		})
	}
	return findings
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

// ExporterNoCompression fires when an OTLP exporter does not have compression
// configured, wasting network bandwidth.
type ExporterNoCompression struct{}

func (r *ExporterNoCompression) ID() string { return "exporter-no-compression" }

func (r *ExporterNoCompression) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, comp := range cfg.Exporters {
		if !networkExporterTypes[comp.Type] {
			continue
		}
		if comp.Config == nil {
			continue
		}
		// Only flag if explicitly set to "none" or "".
		// If the key is absent, the exporter uses the default (gzip), which is fine.
		c, ok := comp.Config["compression"].(string)
		if !ok {
			continue
		}
		if c != "none" && c != "" {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Exporter %s has compression disabled", name),
			Severity:     SeverityInfo,
			Evidence:     fmt.Sprintf("Exporter %q has compression: %q.", name, c),
			Explanation:  "This exporter has compression explicitly disabled.",
			WhyItMatters: "Telemetry data compresses well (often 5-10x). Disabling compression wastes network bandwidth and increases transfer time.",
			Impact:       "Enabling gzip compression significantly reduces network usage with minimal CPU overhead.",
			Snippet: fmt.Sprintf(`exporters:
  %s:
    compression: gzip`, name),
			Placement: "Set compression to gzip or remove the field to use the default.",
		})
	}
	return findings
}

// TailSamplingWithoutMemoryLimiter fires when a traces pipeline has
// tail_sampling but no memory_limiter, risking uncontrolled memory growth.
type TailSamplingWithoutMemoryLimiter struct{}

func (r *TailSamplingWithoutMemoryLimiter) ID() string {
	return "tail-sampling-without-memory-limiter"
}

func (r *TailSamplingWithoutMemoryLimiter) Evaluate(cfg *config.CollectorConfig) []Finding {
	var findings []Finding
	for name, p := range cfg.Pipelines {
		if p.Signal != config.SignalTraces {
			continue
		}
		hasTailSampling := false
		hasMemoryLimiter := false
		for _, proc := range p.Processors {
			pt := config.ComponentType(proc)
			if pt == "tail_sampling" {
				hasTailSampling = true
			}
			if pt == "memory_limiter" {
				hasMemoryLimiter = true
			}
		}
		if !hasTailSampling || hasMemoryLimiter {
			continue
		}
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Tail sampling without memory limiter in %s pipeline", name),
			Severity:     SeverityWarning,
			Evidence:     fmt.Sprintf("Pipeline %q has tail_sampling but no memory_limiter processor.", name),
			Explanation:  "Tail sampling holds complete traces in memory until a sampling decision is made.",
			WhyItMatters: "Without a memory limiter, a traffic spike can cause unbounded memory growth and OOM kills. Tail sampling is particularly memory-intensive because it buffers entire traces.",
			Impact:       "Adding memory_limiter before tail_sampling provides backpressure and prevents OOM crashes.",
			Snippet: `processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
  tail_sampling:
    decision_wait: 10s
    policies: [...]`,
			Placement: "Add memory_limiter as the first processor in the pipeline, before tail_sampling.",
			Pipeline:  name,
		})
	}
	return findings
}

// ConnectorLoop detects cycles in the pipeline graph formed by connectors.
// A loop causes infinite data circulation and eventual resource exhaustion.
type ConnectorLoop struct{}

func (r *ConnectorLoop) ID() string { return "connector-loop" }

func (r *ConnectorLoop) Evaluate(cfg *config.CollectorConfig) []Finding {
	if len(cfg.Connectors) == 0 {
		return nil
	}

	// Build adjacency: for each pipeline, find pipelines it feeds via connectors.
	adj := map[string][]string{}
	// Map connector name → pipelines that receive from it.
	connReceivers := map[string][]string{}
	for pName, p := range cfg.Pipelines {
		for _, recv := range p.Receivers {
			if _, ok := cfg.Connectors[recv]; ok {
				connReceivers[recv] = append(connReceivers[recv], pName)
			}
		}
		_ = pName // ensure pName used
	}
	for pName, p := range cfg.Pipelines {
		for _, exp := range p.Exporters {
			if _, ok := cfg.Connectors[exp]; ok {
				adj[pName] = append(adj[pName], connReceivers[exp]...)
			}
		}
	}

	// DFS cycle detection.
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // finished
	)
	color := map[string]int{}
	parent := map[string]string{}
	var cycles [][]string

	var dfs func(u string)
	dfs = func(u string) {
		color[u] = gray
		for _, v := range adj[u] {
			if color[v] == gray {
				// Found a cycle — reconstruct the path.
				cycle := []string{v}
				for cur := u; cur != v; cur = parent[cur] {
					cycle = append(cycle, cur)
				}
				cycle = append(cycle, v)
				// Reverse to get forward order.
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				cycles = append(cycles, cycle)
			} else if color[v] == white {
				parent[v] = u
				dfs(v)
			}
		}
		color[u] = black
	}

	for pName := range cfg.Pipelines {
		if color[pName] == white {
			dfs(pName)
		}
	}

	if len(cycles) == 0 {
		return nil
	}

	var findings []Finding
	for _, cycle := range cycles {
		path := strings.Join(cycle, " → ")
		findings = append(findings, Finding{
			RuleID:       r.ID(),
			Title:        fmt.Sprintf("Pipeline loop detected: %s", path),
			Severity:     SeverityCritical,
			Evidence:     fmt.Sprintf("Pipelines form a cycle via connectors: %s", path),
			Explanation:  "Data flowing through these pipelines will circulate indefinitely via connectors.",
			WhyItMatters: "A pipeline loop causes infinite data amplification, leading to unbounded memory growth, CPU saturation, and eventual collector crash.",
			Impact:       "Break the loop by removing or reconfiguring the connector that closes the cycle.",
			Snippet:      "# Review the connector routing to ensure no circular dependencies exist.",
			Placement:    "Restructure the pipeline topology to eliminate the cycle.",
		})
	}
	return findings
}

// NoHealthCheckTraceFilter checks that traces pipelines have a filter processor
// that drops health check spans (e.g. /healthz, /readyz, /livez). Without such
// a filter, liveness and readiness probes generate high-volume, low-value traces.
type NoHealthCheckTraceFilter struct{}

func (r *NoHealthCheckTraceFilter) ID() string { return "no-health-check-trace-filter" }

func (r *NoHealthCheckTraceFilter) Evaluate(cfg *config.CollectorConfig) []Finding {
	healthPaths := []string{"/healthz", "/readyz", "/livez", "/health", "/ready"}

	var findings []Finding
	for pName, p := range cfg.Pipelines {
		if p.Signal != config.SignalTraces {
			continue
		}

		hasHealthFilter := false
		for _, proc := range p.Processors {
			procType := config.ComponentType(proc)
			if procType != "filter" {
				continue
			}
			comp, ok := cfg.Processors[proc]
			if !ok || comp.Config == nil {
				continue
			}
			if filterDropsHealthSpans(comp.Config, healthPaths) {
				hasHealthFilter = true
				break
			}
		}

		if !hasHealthFilter {
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Title:    fmt.Sprintf("No health check filter in %s pipeline", pName),
				Severity: SeverityInfo,
				Evidence: fmt.Sprintf("Traces pipeline %q has no filter processor dropping health check spans (/healthz, /readyz).", pName),
				Explanation: "Kubernetes liveness and readiness probes generate a constant stream of spans " +
					"for endpoints like /healthz and /readyz. These traces are high-volume and rarely useful for debugging.",
				WhyItMatters: "Health check traces add storage cost and clutter trace search results without providing diagnostic value.",
				Impact:       "Filtering health check spans typically reduces trace volume by 10-30% in Kubernetes environments.",
				Snippet: `processors:
  filter/health:
    error_mode: ignore
    traces:
      span:
        - 'attributes["url.path"] == "/healthz"'
        - 'attributes["url.path"] == "/readyz"'
        - 'attributes["url.path"] == "/livez"'`,
				Placement: "Add as the first processor after memory_limiter in the traces pipeline.",
				Pipeline:  pName,
			})
		}
	}
	return findings
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
