package analyze

import (
	"testing"

	"github.com/canonical/signal-studio/internal/rules"
)

const validYAML = `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
processors:
  batch:
  memory_limiter:
    check_interval: 1s
    limit_mib: 400
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug]
`

const minimalYAML = `
receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`

func TestRun_ValidConfig(t *testing.T) {
	rpt, err := Run([]byte(validYAML), Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rpt.Config == nil {
		t.Fatal("expected non-nil config")
	}
	if rpt.Summary.Total != len(rpt.Findings) {
		t.Errorf("summary total %d != findings count %d", rpt.Summary.Total, len(rpt.Findings))
	}
}

func TestRun_InvalidYAML(t *testing.T) {
	_, err := Run([]byte("not: [valid: yaml: {{"), Options{})
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestRun_EmptyConfig(t *testing.T) {
	yaml := `
receivers: {}
exporters: {}
service:
  pipelines: {}
`
	rpt, err := Run([]byte(yaml), Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rpt.Config == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestRun_MinSeverityFilter(t *testing.T) {
	// minimalYAML should produce findings at multiple severity levels.
	rptAll, err := Run([]byte(minimalYAML), Options{MinSeverity: rules.SeverityInfo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rptCritical, err := Run([]byte(minimalYAML), Options{MinSeverity: rules.SeverityCritical})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rptCritical.Summary.Total >= rptAll.Summary.Total && rptAll.Summary.Total > 0 {
		// This could happen if all findings are critical. Check that filtering works
		// by verifying no non-critical findings in the critical-only report.
		for _, f := range rptCritical.Findings {
			if f.Severity != rules.SeverityCritical {
				t.Errorf("found non-critical finding %q with severity %q in critical-only report",
					f.RuleID, f.Severity)
			}
		}
	}

	// All findings in critical-only should be critical.
	for _, f := range rptCritical.Findings {
		if f.Severity != rules.SeverityCritical {
			t.Errorf("expected only critical findings, got %q with severity %q",
				f.RuleID, f.Severity)
		}
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		severity rules.Severity
		rank     int
	}{
		{rules.SeverityCritical, 3},
		{rules.SeverityWarning, 2},
		{rules.SeverityInfo, 1},
		{"unknown", 0},
	}
	for _, tt := range tests {
		if got := SeverityRank(tt.severity); got != tt.rank {
			t.Errorf("SeverityRank(%q) = %d, want %d", tt.severity, got, tt.rank)
		}
	}
}

func TestExceedsThreshold(t *testing.T) {
	findings := []rules.Finding{
		{RuleID: "a", Severity: rules.SeverityInfo},
		{RuleID: "b", Severity: rules.SeverityWarning},
	}

	tests := []struct {
		threshold rules.Severity
		want      bool
	}{
		{rules.SeverityCritical, false},
		{rules.SeverityWarning, true},
		{rules.SeverityInfo, true},
	}
	for _, tt := range tests {
		if got := ExceedsThreshold(findings, tt.threshold); got != tt.want {
			t.Errorf("ExceedsThreshold(threshold=%q) = %v, want %v", tt.threshold, got, tt.want)
		}
	}
}

func TestExceedsThreshold_EmptyFindings(t *testing.T) {
	if ExceedsThreshold(nil, rules.SeverityInfo) {
		t.Error("expected false for nil findings")
	}
	if ExceedsThreshold([]rules.Finding{}, rules.SeverityInfo) {
		t.Error("expected false for empty findings")
	}
}

func TestBuildSummary(t *testing.T) {
	findings := []rules.Finding{
		{Severity: rules.SeverityCritical},
		{Severity: rules.SeverityCritical},
		{Severity: rules.SeverityWarning},
		{Severity: rules.SeverityInfo},
	}
	s := buildSummary(findings)
	if s.Total != 4 {
		t.Errorf("total = %d, want 4", s.Total)
	}
	if s.Critical != 2 {
		t.Errorf("critical = %d, want 2", s.Critical)
	}
	if s.Warning != 1 {
		t.Errorf("warning = %d, want 1", s.Warning)
	}
	if s.Info != 1 {
		t.Errorf("info = %d, want 1", s.Info)
	}
}

func TestFilterBySeverity(t *testing.T) {
	findings := []rules.Finding{
		{RuleID: "info-rule", Severity: rules.SeverityInfo},
		{RuleID: "warn-rule", Severity: rules.SeverityWarning},
		{RuleID: "crit-rule", Severity: rules.SeverityCritical},
	}

	// Info: keep all
	result := filterBySeverity(findings, rules.SeverityInfo)
	if len(result) != 3 {
		t.Errorf("info filter: got %d findings, want 3", len(result))
	}

	// Warning: drop info
	result = filterBySeverity(findings, rules.SeverityWarning)
	if len(result) != 2 {
		t.Errorf("warning filter: got %d findings, want 2", len(result))
	}

	// Critical: keep only critical
	result = filterBySeverity(findings, rules.SeverityCritical)
	if len(result) != 1 {
		t.Errorf("critical filter: got %d findings, want 1", len(result))
	}

	// Empty severity: keep all
	result = filterBySeverity(findings, "")
	if len(result) != 3 {
		t.Errorf("empty filter: got %d findings, want 3", len(result))
	}
}
