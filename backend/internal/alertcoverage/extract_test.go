package alertcoverage

import (
	"sort"
	"testing"
)

func TestExtractMetrics_SimpleReference(t *testing.T) {
	names, absent, err := ExtractMetrics(`http_requests_total`)
	if err != nil {
		t.Fatal(err)
	}
	if absent {
		t.Error("expected usesAbsent=false")
	}
	assertNames(t, names, []string{"http_requests_total"})
}

func TestExtractMetrics_WithLabels(t *testing.T) {
	names, _, err := ExtractMetrics(`http_requests_total{job="api",status="500"}`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"http_requests_total"})
}

func TestExtractMetrics_Rate(t *testing.T) {
	names, _, err := ExtractMetrics(`rate(http_requests_total[5m])`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"http_requests_total"})
}

func TestExtractMetrics_BinaryOp(t *testing.T) {
	names, _, err := ExtractMetrics(`metric_a / metric_b > 0.5`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"metric_a", "metric_b"})
}

func TestExtractMetrics_Aggregation(t *testing.T) {
	names, _, err := ExtractMetrics(`sum(rate(foo[5m])) by (job)`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"foo"})
}

func TestExtractMetrics_NestedFunctions(t *testing.T) {
	names, _, err := ExtractMetrics(`histogram_quantile(0.99, sum(rate(request_duration_bucket[5m])) by (le))`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"request_duration_bucket"})
}

func TestExtractMetrics_NameRegex(t *testing.T) {
	names, _, err := ExtractMetrics(`{__name__=~"http_.*"}`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"~http_.*"})
}

func TestExtractMetrics_Absent(t *testing.T) {
	names, absent, err := ExtractMetrics(`absent(up{job="api"})`)
	if err != nil {
		t.Fatal(err)
	}
	if !absent {
		t.Error("expected usesAbsent=true")
	}
	assertNames(t, names, []string{"up"})
}

func TestExtractMetrics_AbsentOverTime(t *testing.T) {
	_, absent, err := ExtractMetrics(`absent_over_time(up{job="api"}[5m])`)
	if err != nil {
		t.Fatal(err)
	}
	if !absent {
		t.Error("expected usesAbsent=true for absent_over_time")
	}
}

func TestExtractMetrics_MixedAbsentAndNormal(t *testing.T) {
	names, absent, err := ExtractMetrics(`absent(up{job="api"}) or rate(http_requests_total[5m]) > 0`)
	if err != nil {
		t.Fatal(err)
	}
	if !absent {
		t.Error("expected usesAbsent=true")
	}
	assertNames(t, names, []string{"http_requests_total", "up"})
}

func TestExtractMetrics_Subquery(t *testing.T) {
	names, _, err := ExtractMetrics(`rate(foo[5m])[30m:1m]`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"foo"})
}

func TestExtractMetrics_DuplicateMetrics(t *testing.T) {
	names, _, err := ExtractMetrics(`rate(foo[5m]) / rate(foo[1m])`)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, names, []string{"foo"})
}

func TestExtractMetrics_InvalidExpr(t *testing.T) {
	_, _, err := ExtractMetrics(`not a valid expr !!!`)
	if err == nil {
		t.Error("expected parse error")
	}
}

func TestExtractMetrics_NoSelectors(t *testing.T) {
	names, _, err := ExtractMetrics(`42`)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected no names, got %v", names)
	}
}

func assertNames(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("names mismatch: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("name[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
