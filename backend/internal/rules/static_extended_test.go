package rules

import (
	"testing"
)

// --- R11: MemoryLimiterNotFirst ---

func TestMemoryLimiterNotFirst_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, memory_limiter]
      exporters: [debug]
`)
	findings := (&MemoryLimiterNotFirst{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
	if findings[0].Pipeline != "traces" {
		t.Errorf("expected pipeline traces, got %s", findings[0].Pipeline)
	}
}

func TestMemoryLimiterNotFirst_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug]
`)
	findings := (&MemoryLimiterNotFirst{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestMemoryLimiterNotFirst_NoMemoryLimiter(t *testing.T) {
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
	findings := (&MemoryLimiterNotFirst{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when memory_limiter not present, got %d", len(findings))
	}
}

func TestMemoryLimiterNotFirst_SlashedName(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
  memory_limiter/strict:
    check_interval: 1s
    limit_mib: 512
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, memory_limiter/strict]
      exporters: [debug]
`)
	findings := (&MemoryLimiterNotFirst{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for slashed memory_limiter not first, got %d", len(findings))
	}
}

// --- R12: BatchBeforeSampling ---

func TestBatchBeforeSampling_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
  tail_sampling:
    decision_wait: 10s
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, tail_sampling]
      exporters: [debug]
`)
	findings := (&BatchBeforeSampling{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestBatchBeforeSampling_FiresWithProbabilistic(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  batch:
  probabilistic_sampler:
    sampling_percentage: 20
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, probabilistic_sampler]
      exporters: [debug]
`)
	findings := (&BatchBeforeSampling{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestBatchBeforeSampling_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  tail_sampling:
    decision_wait: 10s
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [tail_sampling, batch]
      exporters: [debug]
`)
	findings := (&BatchBeforeSampling{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestBatchBeforeSampling_NoSampler(t *testing.T) {
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
	findings := (&BatchBeforeSampling{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when no sampler, got %d", len(findings))
	}
}

// --- R13: BatchNotNearEnd ---

func TestBatchNotNearEnd_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
  batch:
  filter:
  transform:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch, filter, transform]
      exporters: [debug]
`)
	findings := (&BatchNotNearEnd{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
}

func TestBatchNotNearEnd_PassesLast(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
  filter:
  batch:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, filter, batch]
      exporters: [debug]
`)
	findings := (&BatchNotNearEnd{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when batch is last, got %d", len(findings))
	}
}

func TestBatchNotNearEnd_PassesSecondToLast(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
  batch:
  resource:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch, resource]
      exporters: [debug]
`)
	findings := (&BatchNotNearEnd{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when batch is second to last, got %d", len(findings))
	}
}

func TestBatchNotNearEnd_SingleProcessor(t *testing.T) {
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
	findings := (&BatchNotNearEnd{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for single processor, got %d", len(findings))
	}
}

// --- R14: ReceiverEndpointWildcard ---

func TestReceiverEndpointWildcard_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&ReceiverEndpointWildcard{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestReceiverEndpointWildcard_FiresTopLevel(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    endpoint: 0.0.0.0:4317
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&ReceiverEndpointWildcard{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for top-level endpoint, got %d", len(findings))
	}
}

func TestReceiverEndpointWildcard_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&ReceiverEndpointWildcard{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for localhost, got %d", len(findings))
	}
}

func TestReceiverEndpointWildcard_NoConfig(t *testing.T) {
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
	findings := (&ReceiverEndpointWildcard{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for no config, got %d", len(findings))
	}
}

// --- R15: DebugExporterInPipeline ---

func TestDebugExporterInPipeline_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug, otlp/backend]
`)
	findings := (&DebugExporterInPipeline{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestDebugExporterInPipeline_FiresLogging(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  logging:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [logging]
`)
	findings := (&DebugExporterInPipeline{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for logging exporter, got %d", len(findings))
	}
}

func TestDebugExporterInPipeline_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&DebugExporterInPipeline{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// --- R18: PprofExtensionEnabled ---

func TestPprofExtensionEnabled_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
extensions:
  pprof:
    endpoint: localhost:1777
  health_check:
service:
  extensions: [pprof, health_check]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&PprofExtensionEnabled{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
}

func TestPprofExtensionEnabled_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
extensions:
  health_check:
service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&PprofExtensionEnabled{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestPprofExtensionEnabled_NoExtensions(t *testing.T) {
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
	findings := (&PprofExtensionEnabled{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when no extensions, got %d", len(findings))
	}
}

// --- R19: MemoryLimiterWithoutLimits ---

func TestMemoryLimiterWithoutLimits_Fires(t *testing.T) {
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
	findings := (&MemoryLimiterWithoutLimits{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestMemoryLimiterWithoutLimits_PassesWithLimitMib(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [debug]
`)
	findings := (&MemoryLimiterWithoutLimits{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with limit_mib, got %d", len(findings))
	}
}

func TestMemoryLimiterWithoutLimits_PassesWithLimitPercentage(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [debug]
`)
	findings := (&MemoryLimiterWithoutLimits{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with limit_percentage, got %d", len(findings))
	}
}

// --- R20: ExporterNoSendingQueue ---

func TestExporterNoSendingQueue_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterNoSendingQueue{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestExporterNoSendingQueue_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
    sending_queue:
      enabled: true
      queue_size: 5000
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterNoSendingQueue{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestExporterNoSendingQueue_IgnoresNonNetwork(t *testing.T) {
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
	findings := (&ExporterNoSendingQueue{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-network exporter, got %d", len(findings))
	}
}

func TestExporterNoSendingQueue_FiresOtlphttp(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlphttp:
    endpoint: https://backend:4318
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp]
`)
	findings := (&ExporterNoSendingQueue{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for otlphttp, got %d", len(findings))
	}
}

// --- R21: ExporterNoRetry ---

func TestExporterNoRetry_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterNoRetry{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestExporterNoRetry_Passes(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
    retry_on_failure:
      enabled: true
      max_elapsed_time: 300s
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterNoRetry{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestExporterNoRetry_IgnoresNonNetwork(t *testing.T) {
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
	findings := (&ExporterNoRetry{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-network exporter, got %d", len(findings))
	}
}

// --- R22: UndefinedComponentRef ---

func TestUndefinedComponentRef_FiresReceiver(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp, jaeger]
      exporters: [debug]
`)
	findings := (&UndefinedComponentRef{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestUndefinedComponentRef_FiresProcessor(t *testing.T) {
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
      processors: [batch, attributes]
      exporters: [debug]
`)
	findings := (&UndefinedComponentRef{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for undefined processor, got %d", len(findings))
	}
}

func TestUndefinedComponentRef_FiresExporter(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug, otlp/missing]
`)
	findings := (&UndefinedComponentRef{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for undefined exporter, got %d", len(findings))
	}
}

func TestUndefinedComponentRef_Passes(t *testing.T) {
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
	findings := (&UndefinedComponentRef{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestUndefinedComponentRef_MultipleUndefined(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp, jaeger]
      processors: [attributes]
      exporters: [debug, otlp/missing]
`)
	findings := (&UndefinedComponentRef{}).Evaluate(cfg)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings (receiver, processor, exporter), got %d", len(findings))
	}
}

func TestUndefinedComponentRef_PassesConnectorAsReceiver(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
connectors:
  routing:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [routing]
    traces/default:
      receivers: [routing]
      exporters: [debug]
`)
	findings := (&UndefinedComponentRef{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when connector used as receiver, got %d", len(findings))
	}
}

func TestUndefinedComponentRef_PassesConnectorAsExporter(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
connectors:
  forward:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [forward]
    traces/sink:
      receivers: [forward]
      exporters: [debug]
`)
	findings := (&UndefinedComponentRef{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when connector used as exporter, got %d", len(findings))
	}
}

// --- R23: EmptyPipeline ---

func TestEmptyPipeline_FiresNoReceivers(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: []
      exporters: [debug]
`)
	findings := (&EmptyPipeline{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestEmptyPipeline_FiresNoExporters(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: []
`)
	findings := (&EmptyPipeline{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestEmptyPipeline_FiresBoth(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: []
      exporters: []
`)
	findings := (&EmptyPipeline{}).Evaluate(cfg)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (no receivers and no exporters), got %d", len(findings))
	}
}

func TestEmptyPipeline_Passes(t *testing.T) {
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
	findings := (&EmptyPipeline{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// --- R24: FilterErrorModePropagateRule ---

func TestFilterErrorModePropagate_FiresDefault(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  filter:
    logs:
      log_record:
        - 'severity_number < 9'
exporters:
  debug:
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [filter]
      exporters: [debug]
`)
	findings := (&FilterErrorModePropagateRule{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestFilterErrorModePropagate_FiresExplicit(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  filter:
    error_mode: propagate
    logs:
      log_record:
        - 'severity_number < 9'
exporters:
  debug:
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [filter]
      exporters: [debug]
`)
	findings := (&FilterErrorModePropagateRule{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for explicit propagate, got %d", len(findings))
	}
}

func TestFilterErrorModePropagate_PassesIgnore(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  filter:
    error_mode: ignore
    logs:
      log_record:
        - 'severity_number < 9'
exporters:
  debug:
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [filter]
      exporters: [debug]
`)
	findings := (&FilterErrorModePropagateRule{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with error_mode ignore, got %d", len(findings))
	}
}

func TestFilterErrorModePropagate_FiresTransform(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  transform:
    trace_statements:
      - context: span
        statements:
          - set(status.code, 1)
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [transform]
      exporters: [debug]
`)
	findings := (&FilterErrorModePropagateRule{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for transform processor, got %d", len(findings))
	}
}

func TestFilterErrorModePropagate_IgnoresOther(t *testing.T) {
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
	findings := (&FilterErrorModePropagateRule{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-filter/transform processor, got %d", len(findings))
	}
}

// --- Helper tests ---

func TestExtractEndpoints(t *testing.T) {
	cfg := map[string]any{
		"protocols": map[string]any{
			"grpc": map[string]any{
				"endpoint": "0.0.0.0:4317",
			},
			"http": map[string]any{
				"endpoint": "localhost:4318",
			},
		},
	}
	endpoints := extractEndpoints(cfg)
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}
}

func TestHasNestedBool(t *testing.T) {
	cfg := map[string]any{
		"sending_queue": map[string]any{
			"enabled":    true,
			"queue_size": 5000,
		},
	}
	if !hasNestedBool(cfg, "sending_queue", "enabled", true) {
		t.Error("expected hasNestedBool to return true")
	}
	if hasNestedBool(cfg, "sending_queue", "enabled", false) {
		t.Error("expected hasNestedBool to return false for wrong value")
	}
	if hasNestedBool(cfg, "retry_on_failure", "enabled", true) {
		t.Error("expected hasNestedBool to return false for missing section")
	}
}

// --- ScrapeIntervalMismatch ---

func TestScrapeIntervalMismatch_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: collector
          scrape_interval: 15s
  hostmetrics:
    collection_interval: 60s
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers: [prometheus, hostmetrics]
      exporters: [debug]
`)
	findings := (&ScrapeIntervalMismatch{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "scrape-interval-mismatch" {
		t.Errorf("ruleId = %q", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("severity = %q, want warning", findings[0].Severity)
	}
	if findings[0].Pipeline != "metrics" {
		t.Errorf("pipeline = %q, want metrics", findings[0].Pipeline)
	}
}

func TestScrapeIntervalMismatch_NoFire_SameIntervals(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: collector
          scrape_interval: 60s
  hostmetrics:
    collection_interval: 60s
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers: [prometheus, hostmetrics]
      exporters: [debug]
`)
	findings := (&ScrapeIntervalMismatch{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for same intervals, got %d", len(findings))
	}
}

func TestScrapeIntervalMismatch_NoFire_SingleReceiver(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  hostmetrics:
    collection_interval: 10s
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
      exporters: [debug]
`)
	findings := (&ScrapeIntervalMismatch{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for single receiver, got %d", len(findings))
	}
}

func TestScrapeIntervalMismatch_NoFire_NonMetricsPipeline(t *testing.T) {
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
	findings := (&ScrapeIntervalMismatch{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for non-metrics pipeline, got %d", len(findings))
	}
}

func TestScrapeIntervalMismatch_Fires_MultiplePrometheusJobs(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: fast
          scrape_interval: 10s
        - job_name: slow
          scrape_interval: 60s
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers: [prometheus]
      exporters: [debug]
`)
	findings := (&ScrapeIntervalMismatch{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for mismatched prometheus jobs, got %d", len(findings))
	}
}

func TestScrapeIntervalMismatch_NoFire_NoIntervalConfigured(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
  prometheus:
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers: [otlp, prometheus]
      exporters: [debug]
`)
	findings := (&ScrapeIntervalMismatch{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when no intervals are configured, got %d", len(findings))
	}
}

// --- ExporterInsecureTLS ---

func TestExporterInsecureTLS_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
    tls:
      insecure: true
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterInsecureTLS{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestExporterInsecureTLS_PassesSecure(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
    tls:
      insecure: false
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterInsecureTLS{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for secure TLS, got %d", len(findings))
	}
}

func TestExporterInsecureTLS_PassesNoTLS(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterInsecureTLS{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when tls not configured, got %d", len(findings))
	}
}

func TestExporterInsecureTLS_IgnoresNonNetwork(t *testing.T) {
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
	findings := (&ExporterInsecureTLS{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-network exporter, got %d", len(findings))
	}
}

// --- NoHealthCheckExtension ---

func TestNoHealthCheckExtension_Fires(t *testing.T) {
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
	findings := (&NoHealthCheckExtension{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
}

func TestNoHealthCheckExtension_PassesDefined(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
extensions:
  health_check:
service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&NoHealthCheckExtension{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with health_check, got %d", len(findings))
	}
}

func TestNoHealthCheckExtension_PassesServiceOnly(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&NoHealthCheckExtension{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when health_check in service extensions, got %d", len(findings))
	}
}

func TestNoHealthCheckExtension_PassesQualified(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  debug:
extensions:
  health_check/custom:
    endpoint: 0.0.0.0:8080
service:
  extensions: [health_check/custom]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`)
	findings := (&NoHealthCheckExtension{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for qualified health_check, got %d", len(findings))
	}
}

// --- ExporterEndpointLocalhost ---

func TestExporterEndpointLocalhost_FiresLocalhost(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: localhost:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterEndpointLocalhost{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
}

func TestExporterEndpointLocalhost_Fires127(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: 127.0.0.1:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterEndpointLocalhost{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for 127.0.0.1, got %d", len(findings))
	}
}

func TestExporterEndpointLocalhost_FiresWithScheme(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlphttp:
    endpoint: http://localhost:4318
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp]
`)
	findings := (&ExporterEndpointLocalhost{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for http://localhost, got %d", len(findings))
	}
}

func TestExporterEndpointLocalhost_PassesRemote(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend.example.com:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterEndpointLocalhost{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for remote endpoint, got %d", len(findings))
	}
}

func TestExporterEndpointLocalhost_IgnoresNonNetwork(t *testing.T) {
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
	findings := (&ExporterEndpointLocalhost{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-network exporter, got %d", len(findings))
	}
}

// --- ExporterNoCompression ---

func TestExporterNoCompression_FiresNone(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
    compression: none
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterNoCompression{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityInfo {
		t.Errorf("expected info severity, got %s", findings[0].Severity)
	}
}

func TestExporterNoCompression_PassesGzip(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
    compression: gzip
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterNoCompression{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with gzip, got %d", len(findings))
	}
}

func TestExporterNoCompression_PassesDefault(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
exporters:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/backend]
`)
	findings := (&ExporterNoCompression{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when compression not set (defaults to gzip), got %d", len(findings))
	}
}

func TestExporterNoCompression_IgnoresNonNetwork(t *testing.T) {
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
	findings := (&ExporterNoCompression{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-network exporter, got %d", len(findings))
	}
}

// --- TailSamplingWithoutMemoryLimiter ---

func TestTailSamplingWithoutMemoryLimiter_Fires(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  tail_sampling:
    decision_wait: 10s
    policies:
      - name: error
        type: status_code
exporters:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [tail_sampling]
      exporters: [otlp/backend]
`)
	findings := (&TailSamplingWithoutMemoryLimiter{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", findings[0].Severity)
	}
	if findings[0].Pipeline != "traces" {
		t.Errorf("expected pipeline traces, got %s", findings[0].Pipeline)
	}
}

func TestTailSamplingWithoutMemoryLimiter_PassesWithMemoryLimiter(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  tail_sampling:
    decision_wait: 10s
exporters:
  otlp/backend:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, tail_sampling]
      exporters: [otlp/backend]
`)
	findings := (&TailSamplingWithoutMemoryLimiter{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with memory_limiter present, got %d", len(findings))
	}
}

func TestTailSamplingWithoutMemoryLimiter_IgnoresNonTraces(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
processors:
  tail_sampling:
    decision_wait: 10s
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [tail_sampling]
      exporters: [debug]
`)
	findings := (&TailSamplingWithoutMemoryLimiter{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for non-traces pipeline, got %d", len(findings))
	}
}

func TestTailSamplingWithoutMemoryLimiter_NoTailSampling(t *testing.T) {
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
	findings := (&TailSamplingWithoutMemoryLimiter{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when no tail_sampling, got %d", len(findings))
	}
}

// --- ConnectorLoop ---

func TestConnectorLoop_FiresDirectLoop(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
connectors:
  routing:
    match_once: true
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp, routing]
      exporters: [routing, debug]
`)
	findings := (&ConnectorLoop{}).Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for direct self-loop, got %d", len(findings))
	}
	if findings[0].Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %s", findings[0].Severity)
	}
}

func TestConnectorLoop_FiresIndirectLoop(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
connectors:
  forward/a:
  forward/b:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [forward/a]
    traces/middle:
      receivers: [forward/a]
      exporters: [forward/b]
    traces/end:
      receivers: [forward/b]
      exporters: [forward/a, debug]
`)
	findings := (&ConnectorLoop{}).Evaluate(cfg)
	if len(findings) == 0 {
		t.Fatal("expected at least 1 finding for indirect loop")
	}
}

func TestConnectorLoop_PassesNoLoop(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
connectors:
  routing:
    match_once: true
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [routing]
    traces/default:
      receivers: [routing]
      exporters: [debug]
`)
	findings := (&ConnectorLoop{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for acyclic graph, got %d", len(findings))
	}
}

func TestConnectorLoop_PassesNoConnectors(t *testing.T) {
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
	findings := (&ConnectorLoop{}).Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when no connectors, got %d", len(findings))
	}
}

// --- isLocalhostEndpoint ---

func TestIsLocalhostEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"localhost:4317", true},
		{"127.0.0.1:4317", true},
		{"http://localhost:4318", true},
		{"https://localhost:4318", true},
		{"http://127.0.0.1:4318", true},
		{"[::1]:4317", true},
		{"backend:4317", false},
		{"example.com:4317", false},
		{"https://example.com:4317", false},
	}
	for _, tc := range tests {
		got := isLocalhostEndpoint(tc.input)
		if got != tc.want {
			t.Errorf("isLocalhostEndpoint(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// --- Updated DefaultEngine integration ---

func TestDefaultEngineExtendedRules(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
processors:
  batch:
  memory_limiter:
    check_interval: 1s
  filter:
    logs:
      log_record:
        - 'severity_number < 9'
exporters:
  debug:
  otlp/backend:
    endpoint: backend:4317
extensions:
  pprof:
    endpoint: localhost:1777
service:
  extensions: [pprof]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, memory_limiter]
      exporters: [debug, otlp/backend]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, filter, batch]
      exporters: [otlp/backend]
`)
	engine := NewDefaultEngine()
	findings := engine.Evaluate(cfg)

	ruleHits := make(map[string]int)
	for _, f := range findings {
		ruleHits[f.RuleID]++
	}

	// Expected new rule hits:
	// memory-limiter-not-first: traces (batch, memory_limiter -> idx 1)
	// batch-before-sampling: none (no sampler)
	// receiver-endpoint-wildcard: otlp (0.0.0.0)
	// debug-exporter-in-pipeline: traces (debug)
	// pprof-extension-enabled: pprof
	// memory-limiter-without-limits: memory_limiter (no limit_mib or limit_percentage)
	// exporter-no-sending-queue: otlp/backend
	// exporter-no-retry: otlp/backend
	// filter-error-mode-propagate: filter (default propagate)

	expectedNewRules := []string{
		"memory-limiter-not-first",
		"receiver-endpoint-wildcard",
		"debug-exporter-in-pipeline",
		"pprof-extension-enabled",
		"memory-limiter-without-limits",
		"exporter-no-sending-queue",
		"exporter-no-retry",
		"filter-error-mode-propagate",
		"no-health-check-trace-filter",
	}
	for _, r := range expectedNewRules {
		if ruleHits[r] == 0 {
			t.Errorf("expected at least one finding from new rule %q", r)
		}
	}
}

func TestNoHealthCheckTraceFilter_FiresNoFilter(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
processors:
  batch:
exporters:
  otlp:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp]
`)
	rule := &NoHealthCheckTraceFilter{}
	findings := rule.Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Pipeline != "traces" {
		t.Errorf("expected pipeline traces, got %s", findings[0].Pipeline)
	}
}

func TestNoHealthCheckTraceFilter_PassesWithFilter(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
processors:
  batch:
  filter/health:
    traces:
      span:
        - 'attributes["url.path"] == "/healthz"'
        - 'attributes["url.path"] == "/readyz"'
exporters:
  otlp:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [filter/health, batch]
      exporters: [otlp]
`)
	rule := &NoHealthCheckTraceFilter{}
	findings := rule.Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with health filter, got %d", len(findings))
	}
}

func TestNoHealthCheckTraceFilter_SkipsNonTraces(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
processors:
  batch:
exporters:
  otlp:
    endpoint: backend:4317
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp]
`)
	rule := &NoHealthCheckTraceFilter{}
	findings := rule.Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for metrics pipeline, got %d", len(findings))
	}
}

func TestNoHealthCheckTraceFilter_PassesWithLivez(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
processors:
  batch:
  filter/probes:
    traces:
      span:
        - 'attributes["http.route"] == "/livez"'
exporters:
  otlp:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [filter/probes, batch]
      exporters: [otlp]
`)
	rule := &NoHealthCheckTraceFilter{}
	findings := rule.Evaluate(cfg)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings with /livez filter, got %d", len(findings))
	}
}

func TestNoHealthCheckTraceFilter_FiresFilterWithoutTraces(t *testing.T) {
	cfg := mustParse(t, `
receivers:
  otlp:
    protocols:
      grpc:
processors:
  batch:
  filter/metrics:
    metrics:
      metric:
        - 'name == "unwanted"'
exporters:
  otlp:
    endpoint: backend:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [filter/metrics, batch]
      exporters: [otlp]
`)
	rule := &NoHealthCheckTraceFilter{}
	findings := rule.Evaluate(cfg)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (filter has no traces config), got %d", len(findings))
	}
}
