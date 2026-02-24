package config

// Signal represents a telemetry signal type.
type Signal string

const (
	SignalTraces  Signal = "traces"
	SignalMetrics Signal = "metrics"
	SignalLogs    Signal = "logs"
)

// Pipeline represents a single signal pipeline within the collector.
type Pipeline struct {
	Signal     Signal   `json:"signal"`
	Receivers  []string `json:"receivers"`
	Processors []string `json:"processors"`
	Exporters  []string `json:"exporters"`
}

// ComponentConfig holds the parsed configuration for a single collector component.
type ComponentConfig struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config,omitempty"`
}

// CollectorConfig represents the fully parsed collector configuration.
type CollectorConfig struct {
	Receivers         map[string]ComponentConfig `json:"receivers"`
	Processors        map[string]ComponentConfig `json:"processors"`
	Exporters         map[string]ComponentConfig `json:"exporters"`
	Connectors        map[string]ComponentConfig `json:"connectors,omitempty"`
	Extensions        map[string]ComponentConfig `json:"extensions,omitempty"`
	Pipelines         map[string]Pipeline        `json:"pipelines"`
	ServiceExtensions []string                   `json:"serviceExtensions,omitempty"`
}

// ComponentType extracts the base type from a component name.
// For example, "otlp/grpc" returns "otlp".
func ComponentType(name string) string {
	for i, c := range name {
		if c == '/' {
			return name[:i]
		}
	}
	return name
}
