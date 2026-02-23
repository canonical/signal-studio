package config

import (
	"testing"
)

const testConfig = `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  prometheus:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 10s

processors:
  batch:
    timeout: 5s
    send_batch_size: 512
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

exporters:
  otlp/backend:
    endpoint: backend:4317
    tls:
      insecure: true
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/backend]
    metrics:
      receivers: [otlp, prometheus]
      processors: [memory_limiter, batch]
      exporters: [otlp/backend]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/backend, debug]
`

func TestParse(t *testing.T) {
	cfg, err := Parse([]byte(testConfig))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Receivers
	if len(cfg.Receivers) != 2 {
		t.Errorf("expected 2 receivers, got %d", len(cfg.Receivers))
	}
	if _, ok := cfg.Receivers["otlp"]; !ok {
		t.Error("expected otlp receiver")
	}
	if _, ok := cfg.Receivers["prometheus"]; !ok {
		t.Error("expected prometheus receiver")
	}

	// Processors
	if len(cfg.Processors) != 2 {
		t.Errorf("expected 2 processors, got %d", len(cfg.Processors))
	}
	if cfg.Processors["batch"].Type != "batch" {
		t.Errorf("expected batch type, got %q", cfg.Processors["batch"].Type)
	}

	// Exporters
	if len(cfg.Exporters) != 2 {
		t.Errorf("expected 2 exporters, got %d", len(cfg.Exporters))
	}
	if cfg.Exporters["otlp/backend"].Type != "otlp" {
		t.Errorf("expected otlp type for otlp/backend, got %q", cfg.Exporters["otlp/backend"].Type)
	}

	// Pipelines
	if len(cfg.Pipelines) != 3 {
		t.Errorf("expected 3 pipelines, got %d", len(cfg.Pipelines))
	}

	traces := cfg.Pipelines["traces"]
	if traces.Signal != SignalTraces {
		t.Errorf("expected traces signal, got %q", traces.Signal)
	}
	if len(traces.Receivers) != 1 || traces.Receivers[0] != "otlp" {
		t.Errorf("unexpected traces receivers: %v", traces.Receivers)
	}
	if len(traces.Processors) != 2 {
		t.Errorf("expected 2 processors in traces, got %d", len(traces.Processors))
	}
	if len(traces.Exporters) != 1 || traces.Exporters[0] != "otlp/backend" {
		t.Errorf("unexpected traces exporters: %v", traces.Exporters)
	}

	logs := cfg.Pipelines["logs"]
	if logs.Signal != SignalLogs {
		t.Error("expected logs signal")
	}
	if len(logs.Exporters) != 2 {
		t.Errorf("expected 2 exporters in logs, got %d", len(logs.Exporters))
	}
}

func TestParseNamedPipeline(t *testing.T) {
	yaml := `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces/custom:
      receivers: [otlp]
      exporters: [debug]
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p, ok := cfg.Pipelines["traces/custom"]
	if !ok {
		t.Fatal("expected traces/custom pipeline")
	}
	if p.Signal != SignalTraces {
		t.Errorf("expected traces signal, got %q", p.Signal)
	}
}

func TestParseSlashedComponentNames(t *testing.T) {
	yaml := `
receivers:
  otlp:
processors:
  filter/info:
    error_mode: ignore
    logs:
      log_record:
        - 'severity_number < SEVERITY_NUMBER_INFO'
  batch/custom:
    timeout: 10s
  memory_limiter/strict:
    check_interval: 500ms
    limit_mib: 256
exporters:
  otlp/primary:
    endpoint: backend:4317
  otlp/secondary:
    endpoint: backup:4317
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [memory_limiter/strict, filter/info, batch/custom]
      exporters: [otlp/primary, otlp/secondary]
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify processor definitions are keyed by full name
	for _, name := range []string{"filter/info", "batch/custom", "memory_limiter/strict"} {
		p, ok := cfg.Processors[name]
		if !ok {
			t.Errorf("expected processor %q to exist", name)
			continue
		}
		wantType := ComponentType(name)
		if p.Type != wantType {
			t.Errorf("processor %q: expected type %q, got %q", name, wantType, p.Type)
		}
		if p.Name != name {
			t.Errorf("processor %q: expected name %q, got %q", name, name, p.Name)
		}
	}

	// Verify pipeline references preserve full names
	logs := cfg.Pipelines["logs"]
	expectedProcs := []string{"memory_limiter/strict", "filter/info", "batch/custom"}
	if len(logs.Processors) != len(expectedProcs) {
		t.Fatalf("expected %d processors, got %d: %v", len(expectedProcs), len(logs.Processors), logs.Processors)
	}
	for i, want := range expectedProcs {
		if logs.Processors[i] != want {
			t.Errorf("processor[%d]: expected %q, got %q", i, want, logs.Processors[i])
		}
	}

	expectedExporters := []string{"otlp/primary", "otlp/secondary"}
	if len(logs.Exporters) != len(expectedExporters) {
		t.Fatalf("expected %d exporters, got %d", len(expectedExporters), len(logs.Exporters))
	}
	for i, want := range expectedExporters {
		if logs.Exporters[i] != want {
			t.Errorf("exporter[%d]: expected %q, got %q", i, want, logs.Exporters[i])
		}
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := Parse([]byte(`{{{`))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseUnknownSignal(t *testing.T) {
	yaml := `
service:
  pipelines:
    unknown:
      receivers: [otlp]
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for unknown signal type")
	}
}

func TestComponentType(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"otlp", "otlp"},
		{"otlp/grpc", "otlp"},
		{"prometheus/self", "prometheus"},
		{"debug", "debug"},
		{"filter/info", "filter"},
		{"batch/custom", "batch"},
		{"memory_limiter/strict", "memory_limiter"},
		{"probabilistic_sampler/half", "probabilistic_sampler"},
	}
	for _, tt := range tests {
		got := ComponentType(tt.name)
		if got != tt.want {
			t.Errorf("ComponentType(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
