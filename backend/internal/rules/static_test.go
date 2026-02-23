package rules

import (
	"testing"

	"github.com/simskij/otel-signal-lens/internal/config"
)

func mustParse(t *testing.T, yaml string) *config.CollectorConfig {
	t.Helper()
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse test config: %v", err)
	}
	return cfg
}

func findByRule(findings []Finding, ruleID string) []Finding {
	var result []Finding
	for _, f := range findings {
		if f.RuleID == ruleID {
			result = append(result, f)
		}
	}
	return result
}

// --- Rule 1: MissingMemoryLimiter ---

func TestMissingMemoryLimiter_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&MissingMemoryLimiter{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestMissingMemoryLimiter_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
    check_interval: 1s
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [debug]
`)
	findings := (&MissingMemoryLimiter{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// --- Rule 2: MissingBatch ---

func TestMissingBatch_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [debug]
`)
	findings := (&MissingBatch{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestMissingBatch_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&MissingBatch{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// --- Rule 3: NoTraceSampling ---

func TestNoTraceSampling_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&NoTraceSampling{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestNoTraceSampling_IgnoresNonTraces(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
exporters:
  debug:
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&NoTraceSampling{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-traces pipeline, got %d", len(findings))
	}
}

func TestNoTraceSampling_PassesWithSampler(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  probabilistic_sampler:
    sampling_percentage: 20
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [probabilistic_sampler]
      exporters: [debug]
`)
	findings := (&NoTraceSampling{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with sampler, got %d", len(findings))
	}
}

// --- Rule 8: UnusedComponents ---

func TestUnusedComponents_FindsUnused(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
  prometheus:
processors:
  batch:
  memory_limiter:
exporters:
  debug:
  otlp/backend:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&UnusedComponents{}).Evaluate(cfg)
	if len(findings) != 3 {
		t.Fatalf("expected 3 unused findings (prometheus, memory_limiter, otlp/backend), got %d", len(findings))
	}
}

func TestUnusedComponents_AllUsed(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&UnusedComponents{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// --- Rule 9: MultipleExportersNoRouting ---

func TestMultipleExporters_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
  otlp/backend:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug, otlp/backend]
`)
	findings := (&MultipleExportersNoRouting{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestMultipleExporters_PassesWithRouting(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  routing:
exporters:
  debug:
  otlp/backend:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [routing]
      exporters: [debug, otlp/backend]
`)
	findings := (&MultipleExportersNoRouting{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with routing, got %d", len(findings))
	}
}

func TestMultipleExporters_PassesSingleExporter(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&MultipleExportersNoRouting{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for single exporter, got %d", len(findings))
	}
}

// --- Rule 10: NoLogSeverityFilter ---

func TestNoLogSeverityFilter_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
exporters:
  debug:
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&NoLogSeverityFilter{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestNoLogSeverityFilter_PassesWithFilter(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  filter/severity:
    error_mode: ignore
exporters:
  debug:
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [filter/severity]
      exporters: [debug]
`)
	findings := (&NoLogSeverityFilter{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with filter, got %d", len(findings))
	}
}

func TestNoLogSeverityFilter_IgnoresNonLogs(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	findings := (&NoLogSeverityFilter{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-logs pipeline, got %d", len(findings))
	}
}

// --- Slashed component names across all rules ---

func TestSlashedProcessorNames(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter/strict:
    check_interval: 500ms
    limit_mib: 256
  batch/fast:
    timeout: 1s
  probabilistic_sampler/half:
    sampling_percentage: 50
  filter/info:
    error_mode: ignore
  routing/env:
    default_exporters: [otlp/primary]
exporters:
  otlp/primary:
    endpoint: backend:4317
  otlp/secondary:
    endpoint: backup:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter/strict, probabilistic_sampler/half, batch/fast]
      exporters: [otlp/primary, otlp/secondary]
    logs:
      receivers: [otlp]
      processors: [memory_limiter/strict, filter/info, batch/fast]
      exporters: [otlp/primary]
`)

	// Rule 1: memory_limiter/strict should satisfy the memory_limiter check
	mlFindings := (&MissingMemoryLimiter{}).Evaluate(cfg)
	if len(mlFindings) != 0 {
		t.Errorf("MissingMemoryLimiter: expected 0 findings with memory_limiter/strict, got %d", len(mlFindings))
	}

	// Rule 2: batch/fast should satisfy the batch check
	batchFindings := (&MissingBatch{}).Evaluate(cfg)
	if len(batchFindings) != 0 {
		t.Errorf("MissingBatch: expected 0 findings with batch/fast, got %d", len(batchFindings))
	}

	// Rule 3: probabilistic_sampler/half should satisfy the sampling check
	samplingFindings := (&NoTraceSampling{}).Evaluate(cfg)
	if len(samplingFindings) != 0 {
		t.Errorf("NoTraceSampling: expected 0 findings with probabilistic_sampler/half, got %d", len(samplingFindings))
	}

	// Rule 8: all slashed components are used, unused check should find routing/env only
	unusedFindings := (&UnusedComponents{}).Evaluate(cfg)
	for _, f := range unusedFindings {
		if f.Title != "Unused processor: routing/env" {
			t.Errorf("UnusedComponents: unexpected finding: %s", f.Title)
		}
	}
	if len(unusedFindings) != 1 {
		t.Errorf("UnusedComponents: expected 1 finding (routing/env), got %d", len(unusedFindings))
	}

	// Rule 10: filter/info should satisfy the log severity filter check
	filterFindings := (&NoLogSeverityFilter{}).Evaluate(cfg)
	if len(filterFindings) != 0 {
		t.Errorf("NoLogSeverityFilter: expected 0 findings with filter/info, got %d", len(filterFindings))
	}
}

func TestMultipleExporters_PassesWithSlashedRouting(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  routing/env:
    default_exporters: [otlp/primary]
exporters:
  otlp/primary:
  otlp/secondary:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [routing/env]
      exporters: [otlp/primary, otlp/secondary]
`)
	// Rule 9: routing/env should satisfy the routing check
	findings := (&MultipleExportersNoRouting{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with routing/env, got %d", len(findings))
	}
}

// --- Engine integration ---

func TestDefaultEngine(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
  prometheus:
exporters:
  debug:
  otlp/backend:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug, otlp/backend]
    logs:
      receivers: [otlp]
      exporters: [debug]
`)
	engine := NewDefaultEngine()
	findings := engine.Evaluate(cfg)

	// Expect findings from multiple rules
	if len(findings) == 0 {
		t.Fatal("expected findings from default engine")
	}

	// Should have: missing memory_limiter (x2), missing batch (x2),
	// no sampling (traces), unused prometheus, multiple exporters (traces),
	// no log severity filter
	ruleHits := make(map[string]int)
	for _, f := range findings {
		ruleHits[f.RuleID]++
	}

	expectedRules := []string{
		"missing-memory-limiter",
		"missing-batch",
		"no-trace-sampling",
		"unused-components",
		"multiple-exporters-no-routing",
		"no-log-severity-filter",
	}
	for _, r := range expectedRules {
		if ruleHits[r] == 0 {
			t.Errorf("expected at least one finding from rule %q", r)
		}
	}
}
