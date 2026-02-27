package alertcoverage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchRules_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(prometheusRulesResponse{
			Status: "success",
			Data: struct {
				Groups []prometheusAPIGroup `json:"groups"`
			}{
				Groups: []prometheusAPIGroup{{
					Name: "test_group",
					Rules: []prometheusAPIRule{
						{Type: "alerting", Name: "HighLatency", Query: `histogram_quantile(0.99, rate(duration_bucket[5m])) > 1`},
						{Type: "recording", Name: "job:requests:rate5m", Query: `sum(rate(http_requests_total[5m])) by (job)`},
					},
				}},
			},
		})
	}))
	defer srv.Close()

	result, err := FetchRules(ClientOptions{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(result.Rules))
	}
	if result.Rules[0].Name != "HighLatency" || result.Rules[0].Type != "alert" {
		t.Errorf("rule[0]: got %q/%q", result.Rules[0].Name, result.Rules[0].Type)
	}
	if result.Rules[1].Name != "job:requests:rate5m" || result.Rules[1].Type != "record" {
		t.Errorf("rule[1]: got %q/%q", result.Rules[1].Name, result.Rules[1].Type)
	}
	if result.RulesYAML == "" {
		t.Error("expected non-empty reconstructed YAML")
	}
}

func TestFetchRules_BearerToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected bearer token, got %q", auth)
		}
		json.NewEncoder(w).Encode(prometheusRulesResponse{Status: "success"})
	}))
	defer srv.Close()

	_, err := FetchRules(ClientOptions{URL: srv.URL, Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFetchRules_MimirOrgID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get("X-Scope-OrgID")
		if orgID != "tenant-1" {
			t.Errorf("expected X-Scope-OrgID=tenant-1, got %q", orgID)
		}
		json.NewEncoder(w).Encode(prometheusRulesResponse{Status: "success"})
	}))
	defer srv.Close()

	_, err := FetchRules(ClientOptions{URL: srv.URL, OrgID: "tenant-1"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFetchRules_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	_, err := FetchRules(ClientOptions{URL: srv.URL})
	if err == nil {
		t.Error("expected error for 403 response")
	}
}

func TestFetchRules_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := FetchRules(ClientOptions{URL: srv.URL})
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestFetchRules_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(prometheusRulesResponse{Status: "error"})
	}))
	defer srv.Close()

	_, err := FetchRules(ClientOptions{URL: srv.URL})
	if err == nil {
		t.Error("expected error for status=error")
	}
}

func TestMergeRules_Dedup(t *testing.T) {
	set1 := []AlertRule{
		{Name: "A", Group: "g1", Type: "alert"},
		{Name: "B", Group: "g1", Type: "alert"},
	}
	set2 := []AlertRule{
		{Name: "A", Group: "g1", Type: "alert"}, // duplicate
		{Name: "C", Group: "g2", Type: "alert"},
	}

	merged := MergeRules(set1, set2)
	if len(merged) != 3 {
		t.Errorf("expected 3 merged rules, got %d", len(merged))
	}
}

func TestReconstructYAML_SortedOutput(t *testing.T) {
	groups := []prometheusAPIGroup{
		{
			Name: "z_group",
			Rules: []prometheusAPIRule{
				{Type: "alerting", Name: "Beta", Query: "up == 0"},
				{Type: "alerting", Name: "Alpha", Query: "rate(errors[5m]) > 0.1", Duration: 300, Labels: map[string]string{"severity": "critical"}, Annotations: map[string]string{"summary": "high errors"}},
				{Type: "recording", Name: "job:requests:rate5m", Query: "sum(rate(http_total[5m])) by (job)"},
			},
		},
		{
			Name: "a_group",
			Rules: []prometheusAPIRule{
				{Type: "alerting", Name: "Gamma", Query: "node_load1 > 4"},
			},
		},
	}

	yaml := reconstructYAML(groups)

	// Groups should be sorted: a_group before z_group.
	aIdx := strings.Index(yaml, "a_group")
	zIdx := strings.Index(yaml, "z_group")
	if aIdx < 0 || zIdx < 0 {
		t.Fatalf("missing group names in YAML:\n%s", yaml)
	}
	if aIdx >= zIdx {
		t.Errorf("expected a_group before z_group, got a_group@%d z_group@%d", aIdx, zIdx)
	}

	// Within z_group, alerts should come before recording rules, and be sorted by name.
	alphaIdx := strings.Index(yaml, "- alert: Alpha")
	betaIdx := strings.Index(yaml, "- alert: Beta")
	recordIdx := strings.Index(yaml, "- record: job:requests:rate5m")
	if alphaIdx >= betaIdx || betaIdx >= recordIdx {
		t.Errorf("expected Alpha < Beta < record, got %d %d %d", alphaIdx, betaIdx, recordIdx)
	}

	// Check that for, labels, annotations appear for Alpha.
	if !strings.Contains(yaml, "for: 5m") {
		t.Error("expected 'for: 5m' in YAML")
	}
	if !strings.Contains(yaml, "severity: critical") {
		t.Error("expected label 'severity: critical' in YAML")
	}
	if !strings.Contains(yaml, "summary: high errors") {
		t.Error("expected annotation 'summary: high errors' in YAML")
	}
}

func TestReconstructYAML_EmptyGroups(t *testing.T) {
	yaml := reconstructYAML(nil)
	if yaml != "groups:\n" {
		t.Errorf("expected bare groups header, got:\n%s", yaml)
	}
}

func TestMergeRules_SameNameDifferentGroup(t *testing.T) {
	set1 := []AlertRule{{Name: "A", Group: "g1", Type: "alert"}}
	set2 := []AlertRule{{Name: "A", Group: "g2", Type: "alert"}}

	merged := MergeRules(set1, set2)
	if len(merged) != 2 {
		t.Errorf("expected 2 rules (same name, different group), got %d", len(merged))
	}
}
