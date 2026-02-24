package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// rawConfig mirrors the top-level structure of an OTel Collector YAML config.
type rawConfig struct {
	Receivers  map[string]any `yaml:"receivers"`
	Processors map[string]any `yaml:"processors"`
	Exporters  map[string]any `yaml:"exporters"`
	Connectors map[string]any `yaml:"connectors"`
	Extensions map[string]any `yaml:"extensions"`
	Service    rawService     `yaml:"service"`
}

type rawService struct {
	Pipelines  map[string]rawPipeline `yaml:"pipelines"`
	Extensions []string               `yaml:"extensions"`
}

type rawPipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

// Parse takes raw OTel Collector YAML and produces a CollectorConfig.
func Parse(data []byte) (*CollectorConfig, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	cfg := &CollectorConfig{
		Receivers:         parseComponents(raw.Receivers),
		Processors:        parseComponents(raw.Processors),
		Exporters:         parseComponents(raw.Exporters),
		Connectors:        parseComponents(raw.Connectors),
		Extensions:        parseComponents(raw.Extensions),
		Pipelines:         make(map[string]Pipeline),
		ServiceExtensions: raw.Service.Extensions,
	}

	for name, rp := range raw.Service.Pipelines {
		signal, err := signalFromPipelineName(name)
		if err != nil {
			return nil, err
		}
		cfg.Pipelines[name] = Pipeline{
			Signal:     signal,
			Receivers:  rp.Receivers,
			Processors: rp.Processors,
			Exporters:  rp.Exporters,
		}
	}

	return cfg, nil
}

func parseComponents(raw map[string]any) map[string]ComponentConfig {
	components := make(map[string]ComponentConfig, len(raw))
	for name, v := range raw {
		cc := ComponentConfig{
			Type: ComponentType(name),
			Name: name,
		}
		if m, ok := v.(map[string]any); ok {
			cc.Config = m
		}
		components[name] = cc
	}
	return components
}

// signalFromPipelineName extracts the signal type from a pipeline key.
// Supports formats: "traces", "metrics", "logs", "traces/example", etc.
func signalFromPipelineName(name string) (Signal, error) {
	base := name
	for i, c := range name {
		if c == '/' {
			base = name[:i]
			break
		}
	}

	switch base {
	case "traces":
		return SignalTraces, nil
	case "metrics":
		return SignalMetrics, nil
	case "logs":
		return SignalLogs, nil
	default:
		return "", fmt.Errorf("unknown signal type in pipeline %q", name)
	}
}
