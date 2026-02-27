package alertcoverage

import (
	"testing"
)

func TestParseRules_Standard(t *testing.T) {
	data := []byte(`
groups:
  - name: service_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status="500"}[5m]) > 0.05
      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(request_duration_bucket[5m])) > 1
`)

	rules, err := ParseRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	if rules[0].Name != "HighErrorRate" {
		t.Errorf("rule[0] name: got %q, want HighErrorRate", rules[0].Name)
	}
	if rules[0].Type != "alert" {
		t.Errorf("rule[0] type: got %q, want alert", rules[0].Type)
	}
	if rules[0].Group != "service_alerts" {
		t.Errorf("rule[0] group: got %q, want service_alerts", rules[0].Group)
	}
	if len(rules[0].MetricNames) == 0 {
		t.Error("rule[0] should have extracted metric names")
	}
}

func TestParseRules_CRD(t *testing.T) {
	data := []byte(`
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: test-rules
spec:
  groups:
    - name: crd_alerts
      rules:
        - alert: PodCrashLoop
          expr: rate(kube_pod_container_status_restarts_total[15m]) > 0
`)

	rules, err := ParseRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "PodCrashLoop" {
		t.Errorf("got %q, want PodCrashLoop", rules[0].Name)
	}
	if rules[0].Group != "crd_alerts" {
		t.Errorf("got group %q, want crd_alerts", rules[0].Group)
	}
}

func TestParseRules_RecordingRule(t *testing.T) {
	data := []byte(`
groups:
  - name: recording
    rules:
      - record: job:http_requests:rate5m
        expr: sum(rate(http_requests_total[5m])) by (job)
`)

	rules, err := ParseRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Type != "record" {
		t.Errorf("expected type record, got %q", rules[0].Type)
	}
	if rules[0].Name != "job:http_requests:rate5m" {
		t.Errorf("expected name job:http_requests:rate5m, got %q", rules[0].Name)
	}
}

func TestParseRules_MixedAlertAndRecording(t *testing.T) {
	data := []byte(`
groups:
  - name: mixed
    rules:
      - record: job:errors:rate5m
        expr: sum(rate(errors_total[5m])) by (job)
      - alert: HighErrors
        expr: job:errors:rate5m > 0.1
`)

	rules, err := ParseRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestParseRules_EmptyExpr(t *testing.T) {
	data := []byte(`
groups:
  - name: test
    rules:
      - alert: NoExpr
`)

	rules, err := ParseRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules (empty expr skipped), got %d", len(rules))
	}
}

func TestParseRules_MalformedYAML(t *testing.T) {
	data := []byte(`not: valid: yaml: [[[`)
	_, err := ParseRules(data)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestParseRules_EmptyGroups(t *testing.T) {
	data := []byte(`
groups: []
`)
	rules, err := ParseRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestParseRules_AbsentDetection(t *testing.T) {
	data := []byte(`
groups:
  - name: heartbeat
    rules:
      - alert: TargetDown
        expr: absent(up{job="api"})
`)

	rules, err := ParseRules(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if !rules[0].UsesAbsent {
		t.Error("expected UsesAbsent=true for absent() expression")
	}
}
