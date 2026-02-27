package alertcoverage

import (
	"testing"

	"github.com/canonical/signal-studio/internal/filter"
)

func TestAnalyze_Safe(t *testing.T) {
	rules := []AlertRule{{
		Name:        "HealthCheck",
		Type:        "alert",
		Group:       "test",
		Expr:        "up == 0",
		MetricNames: []string{"up"},
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "up", Outcome: filter.OutcomeKept},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Status != AlertSafe {
		t.Errorf("expected safe, got %s", report.Results[0].Status)
	}
	if report.Summary.Safe != 1 {
		t.Errorf("summary safe: got %d, want 1", report.Summary.Safe)
	}
}

func TestAnalyze_Broken(t *testing.T) {
	rules := []AlertRule{{
		Name:        "HighErrors",
		Type:        "alert",
		Group:       "test",
		Expr:        "rate(http_errors_total[5m]) > 0.05",
		MetricNames: []string{"http_errors_total"},
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "http_errors_total", Outcome: filter.OutcomeDropped},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Results[0].Status != AlertBroken {
		t.Errorf("expected broken, got %s", report.Results[0].Status)
	}
	if report.Summary.Broken != 1 {
		t.Errorf("summary broken: got %d, want 1", report.Summary.Broken)
	}
}

func TestAnalyze_AtRisk(t *testing.T) {
	rules := []AlertRule{{
		Name:        "PartialAlert",
		Type:        "alert",
		Group:       "test",
		Expr:        "foo > 1",
		MetricNames: []string{"foo"},
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "foo", Outcome: filter.OutcomePartial},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Results[0].Status != AlertAtRisk {
		t.Errorf("expected at_risk, got %s", report.Results[0].Status)
	}
}

func TestAnalyze_WouldActivate(t *testing.T) {
	rules := []AlertRule{{
		Name:        "TargetDown",
		Type:        "alert",
		Group:       "heartbeat",
		Expr:        `absent(up{job="api"})`,
		MetricNames: []string{"up"},
		UsesAbsent:  true,
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "up", Outcome: filter.OutcomeDropped},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Results[0].Status != AlertWouldActivate {
		t.Errorf("expected would_activate, got %s", report.Results[0].Status)
	}
}

func TestAnalyze_SafeWhenNotFiltered(t *testing.T) {
	rules := []AlertRule{{
		Name:        "ExternalMetric",
		Type:        "alert",
		Group:       "test",
		Expr:        "external_metric > 100",
		MetricNames: []string{"external_metric"},
	}}
	// No filter analyses, no tap — assume safe.
	analyses := []filter.FilterAnalysis{}

	report := Analyze(rules, analyses, nil)
	if report.Results[0].Status != AlertSafe {
		t.Errorf("expected safe (no tap, not filtered), got %s", report.Results[0].Status)
	}
}

func TestAnalyze_SafeWhenInCatalogNotFiltered(t *testing.T) {
	rules := []AlertRule{{
		Name:        "CatalogMetric",
		Type:        "alert",
		Group:       "test",
		Expr:        "system_cpu_time > 0.85",
		MetricNames: []string{"system_cpu_time"},
	}}
	analyses := []filter.FilterAnalysis{}
	known := map[string]struct{}{"system_cpu_time": {}}

	report := Analyze(rules, analyses, known)
	if report.Results[0].Status != AlertSafe {
		t.Errorf("expected safe (in catalog, not filtered), got %s", report.Results[0].Status)
	}
}

func TestAnalyze_UnknownWhenTapActiveButNotInCatalog(t *testing.T) {
	rules := []AlertRule{{
		Name:        "MissingMetric",
		Type:        "alert",
		Group:       "test",
		Expr:        "nonexistent_metric > 100",
		MetricNames: []string{"nonexistent_metric"},
	}}
	analyses := []filter.FilterAnalysis{}
	// Tap is active (non-nil map) but metric not seen.
	known := map[string]struct{}{"some_other_metric": {}}

	report := Analyze(rules, analyses, known)
	if report.Results[0].Status != AlertUnknown {
		t.Errorf("expected unknown (tap active, not in catalog), got %s", report.Results[0].Status)
	}
}

func TestAnalyze_MultipleProcessors(t *testing.T) {
	rules := []AlertRule{{
		Name:        "SurvivorAlert",
		Type:        "alert",
		Group:       "test",
		Expr:        "foo > 1",
		MetricNames: []string{"foo"},
	}}
	// First filter keeps it, second drops it.
	analyses := []filter.FilterAnalysis{
		{Results: []filter.MatchResult{{MetricName: "foo", Outcome: filter.OutcomeKept}}},
		{Results: []filter.MatchResult{{MetricName: "foo", Outcome: filter.OutcomeDropped}}},
	}

	report := Analyze(rules, analyses, nil)
	if report.Results[0].Status != AlertBroken {
		t.Errorf("expected broken (dropped by second processor), got %s", report.Results[0].Status)
	}
}

func TestAnalyze_MultipleMetricsInAlert(t *testing.T) {
	rules := []AlertRule{{
		Name:        "RatioAlert",
		Type:        "alert",
		Group:       "test",
		Expr:        "errors / requests > 0.5",
		MetricNames: []string{"errors", "requests"},
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "errors", Outcome: filter.OutcomeKept},
			{MetricName: "requests", Outcome: filter.OutcomeDropped},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Results[0].Status != AlertBroken {
		t.Errorf("expected broken (one metric dropped), got %s", report.Results[0].Status)
	}
}

func TestAnalyze_SkipsRecordingRules(t *testing.T) {
	rules := []AlertRule{
		{Name: "recording_rule", Type: "record", Group: "test", Expr: "sum(foo)", MetricNames: []string{"foo"}},
		{Name: "AlertRule", Type: "alert", Group: "test", Expr: "bar > 1", MetricNames: []string{"bar"}},
	}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "bar", Outcome: filter.OutcomeKept},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result (recording rule skipped), got %d", len(report.Results))
	}
	if report.Results[0].AlertName != "AlertRule" {
		t.Errorf("expected AlertRule, got %s", report.Results[0].AlertName)
	}
}

func TestAnalyze_SummaryTotals(t *testing.T) {
	rules := []AlertRule{
		{Name: "Safe1", Type: "alert", Group: "g", Expr: "a", MetricNames: []string{"a"}},
		{Name: "Broken1", Type: "alert", Group: "g", Expr: "b", MetricNames: []string{"b"}},
		{Name: "Unknown1", Type: "alert", Group: "g", Expr: "c", MetricNames: []string{"c"}},
	}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "a", Outcome: filter.OutcomeKept},
			{MetricName: "b", Outcome: filter.OutcomeDropped},
			{MetricName: "c", Outcome: filter.OutcomeUnknown},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Summary.Total != 3 {
		t.Errorf("total: got %d, want 3", report.Summary.Total)
	}
	if report.Summary.Safe != 1 {
		t.Errorf("safe: got %d, want 1", report.Summary.Safe)
	}
	if report.Summary.Broken != 1 {
		t.Errorf("broken: got %d, want 1", report.Summary.Broken)
	}
	if report.Summary.Unknown != 1 {
		t.Errorf("unknown: got %d, want 1", report.Summary.Unknown)
	}
}

func TestAnalyze_EmptyRules(t *testing.T) {
	report := Analyze(nil, nil, nil)
	if report.Summary.Total != 0 {
		t.Errorf("expected 0 total, got %d", report.Summary.Total)
	}
}

func TestAnalyze_RegexMetricName(t *testing.T) {
	rules := []AlertRule{{
		Name:        "WildcardAlert",
		Type:        "alert",
		Group:       "test",
		Expr:        `{__name__=~"http_.*"} > 0`,
		MetricNames: []string{"~http_.*"},
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "http_requests_total", Outcome: filter.OutcomeDropped},
			{MetricName: "http_errors_total", Outcome: filter.OutcomeKept},
			{MetricName: "cpu_seconds_total", Outcome: filter.OutcomeKept},
		},
	}}

	report := Analyze(rules, analyses, nil)
	// http_requests_total is dropped → broken.
	if report.Results[0].Status != AlertBroken {
		t.Errorf("expected broken (regex matches a dropped metric), got %s", report.Results[0].Status)
	}
}

func TestAnalyze_RegexNoMatch(t *testing.T) {
	rules := []AlertRule{{
		Name:        "NoMatchRegex",
		Type:        "alert",
		Group:       "test",
		Expr:        `{__name__=~"custom_.*"} > 0`,
		MetricNames: []string{"~custom_.*"},
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "http_requests_total", Outcome: filter.OutcomeKept},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Results[0].Status != AlertUnknown {
		t.Errorf("expected unknown (no regex match), got %s", report.Results[0].Status)
	}
}

func TestComposeOutcome_AllCombinations(t *testing.T) {
	tests := []struct {
		a, b filter.MatchOutcome
		want filter.MatchOutcome
	}{
		{filter.OutcomeKept, filter.OutcomeKept, filter.OutcomeKept},
		{filter.OutcomeKept, filter.OutcomeDropped, filter.OutcomeDropped},
		{filter.OutcomeDropped, filter.OutcomeKept, filter.OutcomeDropped},
		{filter.OutcomeKept, filter.OutcomePartial, filter.OutcomePartial},
		{filter.OutcomePartial, filter.OutcomeKept, filter.OutcomePartial},
		{filter.OutcomeKept, filter.OutcomeUnknown, filter.OutcomeUnknown},
		{filter.OutcomeUnknown, filter.OutcomeKept, filter.OutcomeUnknown},
		{filter.OutcomePartial, filter.OutcomeUnknown, filter.OutcomePartial},
		{filter.OutcomeDropped, filter.OutcomeDropped, filter.OutcomeDropped},
	}
	for _, tt := range tests {
		got := composeOutcome(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("composeOutcome(%s, %s) = %s, want %s", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestDeriveStatus_EmptyMetrics(t *testing.T) {
	status := deriveStatus(nil, false)
	if status != AlertUnknown {
		t.Errorf("expected unknown for empty metrics, got %s", status)
	}
}

func TestAnalyze_WouldActivateSummary(t *testing.T) {
	rules := []AlertRule{{
		Name:        "AbsentAlert",
		Type:        "alert",
		Group:       "test",
		Expr:        "absent(foo)",
		MetricNames: []string{"foo"},
		UsesAbsent:  true,
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "foo", Outcome: filter.OutcomeDropped},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Summary.WouldActivate != 1 {
		t.Errorf("summary wouldActivate: got %d, want 1", report.Summary.WouldActivate)
	}
}

func TestAnalyze_AtRiskSummary(t *testing.T) {
	rules := []AlertRule{{
		Name:        "PartialAlert",
		Type:        "alert",
		Group:       "test",
		Expr:        "foo",
		MetricNames: []string{"foo"},
	}}
	analyses := []filter.FilterAnalysis{{
		Results: []filter.MatchResult{
			{MetricName: "foo", Outcome: filter.OutcomePartial},
		},
	}}

	report := Analyze(rules, analyses, nil)
	if report.Summary.AtRisk != 1 {
		t.Errorf("summary atRisk: got %d, want 1", report.Summary.AtRisk)
	}
}
