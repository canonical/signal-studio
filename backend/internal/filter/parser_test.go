package filter

import (
	"testing"

	"github.com/simskij/signal-studio/internal/config"
)

func TestExtractFilterConfigs_Legacy(t *testing.T) {
	cfg := &config.CollectorConfig{
		Processors: map[string]config.ComponentConfig{
			"filter": {
				Type: "filter",
				Name: "filter",
				Config: map[string]any{
					"metrics": map[string]any{
						"include": map[string]any{
							"match_type":   "regexp",
							"metric_names": []any{`http\.server\..*`, `rpc\..*`},
						},
					},
				},
			},
		},
		Pipelines: map[string]config.Pipeline{
			"metrics": {
				Signal:     config.SignalMetrics,
				Processors: []string{"filter"},
			},
		},
	}

	fcs := ExtractFilterConfigs(cfg)
	if len(fcs) != 1 {
		t.Fatalf("expected 1 filter config, got %d", len(fcs))
	}

	fc := fcs[0]
	if fc.Style != "legacy" {
		t.Errorf("expected style legacy, got %s", fc.Style)
	}
	if fc.Pipeline != "metrics" {
		t.Errorf("expected pipeline metrics, got %s", fc.Pipeline)
	}
	if len(fc.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(fc.Rules))
	}
	if fc.Rules[0].Action != ActionInclude {
		t.Errorf("expected action include, got %s", fc.Rules[0].Action)
	}
	if fc.Rules[0].MatchType != MatchTypeRegexp {
		t.Errorf("expected match type regexp, got %s", fc.Rules[0].MatchType)
	}
}

func TestExtractFilterConfigs_LegacyExclude(t *testing.T) {
	cfg := &config.CollectorConfig{
		Processors: map[string]config.ComponentConfig{
			"filter/drop": {
				Type: "filter",
				Name: "filter/drop",
				Config: map[string]any{
					"metrics": map[string]any{
						"exclude": map[string]any{
							"match_type":   "strict",
							"metric_names": []any{"system.cpu.time"},
						},
					},
				},
			},
		},
		Pipelines: map[string]config.Pipeline{
			"metrics": {
				Signal:     config.SignalMetrics,
				Processors: []string{"filter/drop"},
			},
		},
	}

	fcs := ExtractFilterConfigs(cfg)
	if len(fcs) != 1 {
		t.Fatalf("expected 1 filter config, got %d", len(fcs))
	}
	if fcs[0].Rules[0].Action != ActionExclude {
		t.Errorf("expected action exclude, got %s", fcs[0].Rules[0].Action)
	}
	if fcs[0].Rules[0].MatchType != MatchTypeStrict {
		t.Errorf("expected match type strict, got %s", fcs[0].Rules[0].MatchType)
	}
}

func TestExtractFilterConfigs_OTTL(t *testing.T) {
	cfg := &config.CollectorConfig{
		Processors: map[string]config.ComponentConfig{
			"filter/ottl": {
				Type: "filter",
				Name: "filter/ottl",
				Config: map[string]any{
					"metrics": map[string]any{
						"metric": []any{
							`name == "http.server.duration"`,
							`IsMatch(name, "system\.cpu\..*")`,
						},
					},
				},
			},
		},
		Pipelines: map[string]config.Pipeline{
			"metrics/internal": {
				Signal:     config.SignalMetrics,
				Processors: []string{"filter/ottl"},
			},
		},
	}

	fcs := ExtractFilterConfigs(cfg)
	if len(fcs) != 1 {
		t.Fatalf("expected 1 filter config, got %d", len(fcs))
	}
	fc := fcs[0]
	if fc.Style != "ottl" {
		t.Errorf("expected style ottl, got %s", fc.Style)
	}
	if len(fc.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(fc.Rules))
	}
	if fc.Rules[0].MatchType != MatchTypeOTTLNameEq {
		t.Errorf("expected ottl_name_eq, got %s", fc.Rules[0].MatchType)
	}
	if fc.Rules[1].MatchType != MatchTypeOTTLIsMatch {
		t.Errorf("expected ottl_ismatch, got %s", fc.Rules[1].MatchType)
	}
}

func TestExtractFilterConfigs_MultiplePipelines(t *testing.T) {
	cfg := &config.CollectorConfig{
		Processors: map[string]config.ComponentConfig{
			"filter": {
				Type: "filter",
				Name: "filter",
				Config: map[string]any{
					"metrics": map[string]any{
						"exclude": map[string]any{
							"match_type":   "strict",
							"metric_names": []any{"unwanted"},
						},
					},
				},
			},
		},
		Pipelines: map[string]config.Pipeline{
			"metrics/a": {
				Signal:     config.SignalMetrics,
				Processors: []string{"filter"},
			},
			"metrics/b": {
				Signal:     config.SignalMetrics,
				Processors: []string{"filter"},
			},
		},
	}

	fcs := ExtractFilterConfigs(cfg)
	if len(fcs) != 2 {
		t.Fatalf("expected 2 filter configs (one per pipeline), got %d", len(fcs))
	}
}

func TestExtractFilterConfigs_NonFilterIgnored(t *testing.T) {
	cfg := &config.CollectorConfig{
		Processors: map[string]config.ComponentConfig{
			"batch": {
				Type:   "batch",
				Name:   "batch",
				Config: map[string]any{},
			},
		},
		Pipelines: map[string]config.Pipeline{},
	}

	fcs := ExtractFilterConfigs(cfg)
	if len(fcs) != 0 {
		t.Errorf("expected 0 filter configs, got %d", len(fcs))
	}
}

func TestExtractFilterConfigs_EmptyConfig(t *testing.T) {
	cfg := &config.CollectorConfig{
		Processors: map[string]config.ComponentConfig{
			"filter": {
				Type:   "filter",
				Name:   "filter",
				Config: nil,
			},
		},
		Pipelines: map[string]config.Pipeline{},
	}

	fcs := ExtractFilterConfigs(cfg)
	if len(fcs) != 1 {
		t.Fatalf("expected 1 filter config, got %d", len(fcs))
	}
	if len(fcs[0].Rules) != 0 {
		t.Errorf("expected 0 rules for empty config, got %d", len(fcs[0].Rules))
	}
}
