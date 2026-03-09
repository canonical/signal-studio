package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/canonical/signal-studio/internal/alertcoverage"
	"github.com/canonical/signal-studio/internal/analyze"
	"github.com/canonical/signal-studio/internal/rules"
)

func sampleReport() *analyze.Report {
	return &analyze.Report{
		Findings: []rules.Finding{
			{
				RuleID:         "missing-memory-limiter",
				Title:          "Missing memory limiter",
				Severity:       rules.SeverityCritical,
				Confidence:     rules.ConfidenceHigh,
				Evidence:       "Pipeline metrics/default has no memory_limiter",
				Implication:    "Without memory limiting, the Collector can OOM under load. Collector process may be killed by the OS.",
				Scope:          "pipeline:metrics/default",
				Snippet:        "processors:\n  memory_limiter:\n    check_interval: 1s\n    limit_mib: 400",
				Recommendation: "Add memory_limiter as the first processor in the pipeline",
			},
			{
				RuleID:         "receiver-endpoint-wildcard",
				Title:          "Receiver binds to all interfaces",
				Severity:       rules.SeverityWarning,
				Confidence:     rules.ConfidenceMedium,
				Implication:    "Receiver otlp binds to 0.0.0.0",
				Scope:          "receiver:otlp",
				Recommendation: "Bind to localhost instead",
			},
			{
				RuleID:      "no-trace-sampling",
				Title:       "No trace sampling configured",
				Severity:    rules.SeverityInfo,
				Confidence:  rules.ConfidenceLow,
				Implication: "Traces pipeline has no sampling processor",
				Scope:       "pipeline:traces",
			},
		},
		Summary: analyze.Summary{
			Total:    3,
			Critical: 1,
			Warning:  1,
			Info:     1,
		},
	}
}

func emptyReport() *analyze.Report {
	return &analyze.Report{
		Findings: []rules.Finding{},
		Summary:  analyze.Summary{},
	}
}

// --- Text formatter tests ---

func TestTextFormatter_Output(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	// Severity group headers.
	if !strings.Contains(out, "CRITICAL\n") {
		t.Error("expected CRITICAL header")
	}
	if !strings.Contains(out, "WARNING\n") {
		t.Error("expected WARNING header")
	}
	if !strings.Contains(out, "INFO\n") {
		t.Error("expected INFO header")
	}
	// Rule ID with scope in brackets.
	if !strings.Contains(out, "missing-memory-limiter [pipeline:metrics/default]") {
		t.Error("expected rule ID with scope")
	}
	// Evidence section.
	if !strings.Contains(out, "Evidence: Pipeline metrics/default has no memory_limiter") {
		t.Error("expected evidence section")
	}
	// Implication section.
	if !strings.Contains(out, "Implication: Without memory limiting, the Collector can OOM under load. Collector process may be killed by the OS.") {
		t.Error("expected implication section")
	}
	// Recommendation section.
	if !strings.Contains(out, "Recommendation: Add memory_limiter as the first processor in the pipeline") {
		t.Error("expected recommendation section")
	}
	if !strings.Contains(out, "3 findings") {
		t.Error("expected summary line")
	}
	// CRITICAL should appear before WARNING.
	critIdx := strings.Index(out, "CRITICAL")
	warnIdx := strings.Index(out, "WARNING")
	infoIdx := strings.Index(out, "INFO")
	if critIdx >= warnIdx {
		t.Error("CRITICAL should appear before WARNING")
	}
	if warnIdx >= infoIdx {
		t.Error("WARNING should appear before INFO")
	}
}

func TestTextFormatter_EmptyReport(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	if err := f.Format(emptyReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "0 findings") {
		t.Errorf("expected '0 findings', got: %s", out)
	}
}

func TestTextFormatter_NoColor(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "\033[") {
		t.Error("expected no ANSI codes when NoColor is true")
	}
}

func TestTextFormatter_ColorDisabledForNonTTY(t *testing.T) {
	// Writing to a bytes.Buffer (not a TTY) should never produce color.
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: false}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "\033[") {
		t.Error("expected no ANSI codes when writing to non-TTY")
	}
}

// --- JSON formatter tests ---

func TestJSONFormatter_Output(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if v, ok := parsed["formatVersion"]; !ok || v != "1" {
		t.Errorf("expected formatVersion=1, got %v", v)
	}

	findings, ok := parsed["findings"].([]any)
	if !ok {
		t.Fatal("expected findings array")
	}
	if len(findings) != 3 {
		t.Errorf("expected 3 findings, got %d", len(findings))
	}

	summary, ok := parsed["summary"].(map[string]any)
	if !ok {
		t.Fatal("expected summary object")
	}
	if total, ok := summary["total"].(float64); !ok || total != 3 {
		t.Errorf("expected summary.total=3, got %v", summary["total"])
	}
}

func TestJSONFormatter_EmptyReport(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	if err := f.Format(emptyReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	findings, ok := parsed["findings"].([]any)
	if !ok {
		t.Fatal("expected findings array")
	}
	if len(findings) != 0 {
		t.Error("expected empty findings array")
	}
}

func TestJSONFormatter_FieldStructure(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)

	findings := parsed["findings"].([]any)
	first := findings[0].(map[string]any)

	requiredFields := []string{"ruleId", "title", "severity", "confidence", "implication", "snippet", "recommendation"}
	for _, field := range requiredFields {
		if _, ok := first[field]; !ok {
			t.Errorf("expected field %q in finding", field)
		}
	}
}

// --- SARIF formatter tests ---

func TestSARIFFormatter_Output(t *testing.T) {
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid SARIF JSON output: %v", err)
	}

	if v := parsed["version"]; v != "2.1.0" {
		t.Errorf("expected SARIF version 2.1.0, got %v", v)
	}

	if v := parsed["$schema"]; v == nil {
		t.Error("expected $schema field")
	}

	runs, ok := parsed["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatal("expected at least one run")
	}
	run := runs[0].(map[string]any)

	// Check tool info.
	tool := run["tool"].(map[string]any)
	driver := tool["driver"].(map[string]any)
	if name := driver["name"]; name != "signal-studio" {
		t.Errorf("expected tool name signal-studio, got %v", name)
	}

	// Check rules registered.
	driverRules, ok := driver["rules"].([]any)
	if !ok {
		t.Fatal("expected rules array in driver")
	}
	if len(driverRules) < 2 {
		t.Errorf("expected at least 2 rules, got %d", len(driverRules))
	}

	// Check results.
	results, ok := run["results"].([]any)
	if !ok {
		t.Fatal("expected results array")
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestSARIFFormatter_SeverityMapping(t *testing.T) {
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)

	runs := parsed["runs"].([]any)
	results := runs[0].(map[string]any)["results"].([]any)

	// First finding is critical → "error"
	first := results[0].(map[string]any)
	if level := first["level"]; level != "error" {
		t.Errorf("critical should map to 'error', got %v", level)
	}

	// Second finding is warning → "warning"
	second := results[1].(map[string]any)
	if level := second["level"]; level != "warning" {
		t.Errorf("warning should map to 'warning', got %v", level)
	}

	// Third finding is info → "note"
	third := results[2].(map[string]any)
	if level := third["level"]; level != "note" {
		t.Errorf("info should map to 'note', got %v", level)
	}
}

func TestSARIFFormatter_Properties(t *testing.T) {
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)

	runs := parsed["runs"].([]any)
	results := runs[0].(map[string]any)["results"].([]any)
	first := results[0].(map[string]any)

	props, ok := first["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties bag on result")
	}
	if props["confidence"] != "high" {
		t.Errorf("expected confidence=high, got %v", props["confidence"])
	}
	if props["evidence"] == nil {
		t.Error("expected evidence in properties")
	}
}

func TestSARIFFormatter_LogicalLocations(t *testing.T) {
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)

	runs := parsed["runs"].([]any)
	results := runs[0].(map[string]any)["results"].([]any)
	first := results[0].(map[string]any)

	locations, ok := first["locations"].([]any)
	if !ok || len(locations) == 0 {
		t.Fatal("expected locations")
	}
	loc := locations[0].(map[string]any)
	logLocs, ok := loc["logicalLocations"].([]any)
	if !ok || len(logLocs) == 0 {
		t.Fatal("expected logical locations")
	}
	logLoc := logLocs[0].(map[string]any)
	if name := logLoc["name"]; name != "pipeline:metrics/default" {
		t.Errorf("expected logical location name 'pipeline:metrics/default', got %v", name)
	}
}

func TestSARIFFormatter_Fixes(t *testing.T) {
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)

	runs := parsed["runs"].([]any)
	results := runs[0].(map[string]any)["results"].([]any)
	first := results[0].(map[string]any)

	fixes, ok := first["fixes"].([]any)
	if !ok || len(fixes) == 0 {
		t.Fatal("expected fixes for finding with snippet")
	}
}

func TestSARIFFormatter_EmptyReport(t *testing.T) {
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(emptyReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid SARIF JSON: %v", err)
	}
	if v := parsed["version"]; v != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %v", v)
	}
}

func TestSarifLevel(t *testing.T) {
	tests := []struct {
		severity rules.Severity
		want     string
	}{
		{rules.SeverityCritical, "error"},
		{rules.SeverityWarning, "warning"},
		{rules.SeverityInfo, "note"},
		{"unknown", "none"},
	}
	for _, tt := range tests {
		if got := sarifLevel(tt.severity); got != tt.want {
			t.Errorf("sarifLevel(%q) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}

// --- Markdown formatter tests ---

func TestMarkdownFormatter_Output(t *testing.T) {
	var buf bytes.Buffer
	f := &MarkdownFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "## Signal Studio Analysis") {
		t.Error("expected heading")
	}
	if !strings.Contains(out, "| Severity | Rule | Finding | Scope |") {
		t.Error("expected table header")
	}
	if !strings.Contains(out, "| critical |") {
		t.Error("expected critical row")
	}
	if !strings.Contains(out, "| warning |") {
		t.Error("expected warning row")
	}
	if !strings.Contains(out, "**Summary:**") {
		t.Error("expected summary line")
	}
}

func TestMarkdownFormatter_EmptyReport(t *testing.T) {
	var buf bytes.Buffer
	f := &MarkdownFormatter{}
	if err := f.Format(emptyReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No findings.") {
		t.Error("expected 'No findings.' for empty report")
	}
	if strings.Contains(out, "| Severity |") {
		t.Error("should not have table for empty report")
	}
}

func TestMarkdownFormatter_TableRowCount(t *testing.T) {
	var buf bytes.Buffer
	f := &MarkdownFormatter{}
	if err := f.Format(sampleReport(), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3 findings = header + separator + 3 data rows = lines starting with "|"
	pipeLines := 0
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.HasPrefix(line, "|") {
			pipeLines++
		}
	}
	if pipeLines != 5 { // header + separator + 3 data rows
		t.Errorf("expected 5 table lines, got %d", pipeLines)
	}
}

// --- Color/severity helpers ---

func TestColorSeverity(t *testing.T) {
	f := &TextFormatter{}
	tests := []struct {
		severity rules.Severity
		want     string
	}{
		{rules.SeverityCritical, ansiBold + ansiRed},
		{rules.SeverityWarning, ansiYellow},
		{rules.SeverityInfo, ansiCyan},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := f.colorSeverity(tt.severity)
		if got != tt.want {
			t.Errorf("colorSeverity(%q) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}

func TestTextFormatter_WriteFindingWithScope(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	f.writeFinding(&buf, rules.Finding{
		RuleID:         "test-rule",
		Severity:       rules.SeverityWarning,
		Evidence:       "binds to 0.0.0.0",
		Implication:    "exposes the receiver to all network interfaces",
		Recommendation: "Bind to localhost instead",
		Scope:          "receiver:otlp",
	}, false)
	out := buf.String()
	if !strings.Contains(out, "test-rule [receiver:otlp]") {
		t.Error("expected rule ID with scope in brackets")
	}
	if !strings.Contains(out, "Evidence: binds to 0.0.0.0") {
		t.Error("expected evidence section")
	}
	if !strings.Contains(out, "Implication: exposes the receiver to all network interfaces") {
		t.Error("expected implication section")
	}
	if !strings.Contains(out, "Recommendation: Bind to localhost instead") {
		t.Error("expected recommendation section")
	}
}

func TestTextFormatter_WriteFindingWithColorAndScope(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{}
	f.writeFinding(&buf, rules.Finding{
		RuleID:         "test-rule",
		Severity:       rules.SeverityWarning,
		Evidence:       "test evidence",
		Implication:    "test implication",
		Recommendation: "test recommendation",
		Scope:          "receiver:otlp",
	}, true)
	out := buf.String()
	if !strings.Contains(out, ansiBold+"test-rule"+ansiReset) {
		t.Error("expected bold rule ID with color")
	}
	if !strings.Contains(out, ansiMagenta+"[receiver:otlp]"+ansiReset) {
		t.Error("expected magenta scope with color")
	}
	// Detail labels should be bold in color mode.
	if !strings.Contains(out, ansiBold+"Evidence:"+ansiReset+" test evidence") {
		t.Error("expected bold evidence label with color")
	}
	if !strings.Contains(out, ansiBold+"Recommendation:"+ansiReset+" test recommendation") {
		t.Error("expected bold recommendation label with color")
	}
}

func TestTextFormatter_WriteFindingWithoutScope(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	f.writeFinding(&buf, rules.Finding{
		RuleID:      "test-rule",
		Severity:    rules.SeverityInfo,
		Implication: "test implication",
	}, false)
	out := buf.String()
	if !strings.Contains(out, "test-rule\n") {
		t.Error("expected rule ID without scope")
	}
	if !strings.Contains(out, "Implication: test implication") {
		t.Error("expected implication section")
	}
	if strings.Contains(out, "[") {
		t.Error("should not have scope brackets when scope is empty")
	}
}

func TestTextFormatter_WriteFindingWithScopeDisplay(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	f.writeFinding(&buf, rules.Finding{
		RuleID:         "test-rule",
		Recommendation: "add sampling",
		Scope:          "pipeline:traces",
	}, false)
	out := buf.String()
	if !strings.Contains(out, "test-rule [pipeline:traces]") {
		t.Error("expected scope in brackets")
	}
	if !strings.Contains(out, "Recommendation: add sampling") {
		t.Error("expected recommendation section")
	}
}

func TestTextFormatter_WriteFindingNoDetails(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	f.writeFinding(&buf, rules.Finding{
		RuleID: "test-rule",
		Scope:  "test:scope",
	}, false)
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Should be just the rule ID line, no detail sections.
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d: %q", len(lines), out)
	}
}



func TestTextFormatter_WriteSummaryWithColor(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{}
	f.writeSummary(&buf, analyze.Summary{
		Total: 3, Critical: 1, Warning: 1, Info: 1,
	}, true)
	out := buf.String()
	if !strings.Contains(out, ansiRed) {
		t.Error("expected red ANSI code for critical")
	}
	if !strings.Contains(out, ansiYellow) {
		t.Error("expected yellow ANSI code for warning")
	}
	if !strings.Contains(out, ansiCyan) {
		t.Error("expected cyan ANSI code for info")
	}
}

func TestTextFormatter_SeverityHeaderWithColor(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{}
	f.writeSeverityHeader(&buf, rules.SeverityCritical, true)
	out := buf.String()
	if !strings.Contains(out, ansiRed) {
		t.Error("expected red ANSI for critical header")
	}
	if !strings.Contains(out, "CRITICAL") {
		t.Error("expected CRITICAL label")
	}
}

func TestGroupBySeverity(t *testing.T) {
	findings := []rules.Finding{
		{RuleID: "a", Severity: rules.SeverityInfo},
		{RuleID: "b", Severity: rules.SeverityCritical},
		{RuleID: "c", Severity: rules.SeverityWarning},
		{RuleID: "d", Severity: rules.SeverityInfo},
	}
	grouped := groupBySeverity(findings)
	if len(grouped[rules.SeverityCritical]) != 1 {
		t.Errorf("expected 1 critical, got %d", len(grouped[rules.SeverityCritical]))
	}
	if len(grouped[rules.SeverityWarning]) != 1 {
		t.Errorf("expected 1 warning, got %d", len(grouped[rules.SeverityWarning]))
	}
	if len(grouped[rules.SeverityInfo]) != 2 {
		t.Errorf("expected 2 info, got %d", len(grouped[rules.SeverityInfo]))
	}
}

// --- isTerminal ---

func TestIsTerminal_Buffer(t *testing.T) {
	var buf bytes.Buffer
	if isTerminal(&buf) {
		t.Error("bytes.Buffer should not be detected as terminal")
	}
}

// --- JSON edge case ---

func TestJSONFormatter_NilFindings(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	rpt := &analyze.Report{
		Findings: nil,
		Summary:  analyze.Summary{},
	}
	if err := f.Format(rpt, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)
	findings := parsed["findings"].([]any)
	if len(findings) != 0 {
		t.Error("nil findings should serialize as empty array")
	}
}

// --- SARIF edge case ---

func TestSARIFFormatter_DuplicateRuleIDs(t *testing.T) {
	rpt := &analyze.Report{
		Findings: []rules.Finding{
			{RuleID: "same-rule", Title: "Title", Severity: rules.SeverityWarning, Implication: "first"},
			{RuleID: "same-rule", Title: "Title", Severity: rules.SeverityWarning, Implication: "second"},
		},
		Summary: analyze.Summary{Total: 2, Warning: 2},
	}
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(rpt, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)
	runs := parsed["runs"].([]any)
	run := runs[0].(map[string]any)
	tool := run["tool"].(map[string]any)
	driver := tool["driver"].(map[string]any)
	driverRules := driver["rules"].([]any)
	// Should only register the rule once.
	if len(driverRules) != 1 {
		t.Errorf("expected 1 registered rule for duplicate IDs, got %d", len(driverRules))
	}
	results := run["results"].([]any)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// --- Text alert coverage ---

func TestTextFormatter_WriteAlertCoverage(t *testing.T) {
	rpt := &analyze.Report{
		Findings: []rules.Finding{},
		Summary:  analyze.Summary{},
		AlertCoverage: &alertcoverage.CoverageReport{
			Results: []alertcoverage.AlertCoverageResult{
				{
					AlertName:  "HighErrorRate",
					AlertGroup: "slo",
					Status:     alertcoverage.AlertSafe,
					Metrics: []alertcoverage.AlertMetricResult{
						{MetricName: "http_requests_total", FilterOutcome: "kept"},
					},
				},
				{
					AlertName:  "DiskFull",
					AlertGroup: "infra",
					Status:     alertcoverage.AlertBroken,
					Metrics: []alertcoverage.AlertMetricResult{
						{MetricName: "node_disk_bytes", FilterOutcome: "dropped"},
					},
				},
				{
					AlertName:  "MemoryHigh",
					AlertGroup: "infra",
					Status:     alertcoverage.AlertAtRisk,
				},
				{
					AlertName:  "CPUIdle",
					AlertGroup: "infra",
					Status:     alertcoverage.AlertUnknown,
				},
				{
					AlertName:  "NewMetric",
					AlertGroup: "infra",
					Status:     alertcoverage.AlertWouldActivate,
				},
			},
			Summary: alertcoverage.CoverageSummary{
				Total: 5, Safe: 1, AtRisk: 1, Broken: 1, WouldActivate: 1, Unknown: 1,
			},
		},
	}

	var buf bytes.Buffer
	f := &TextFormatter{NoColor: true}
	if err := f.Format(rpt, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "ALERT COVERAGE") {
		t.Error("expected ALERT COVERAGE header")
	}
	if !strings.Contains(out, "HighErrorRate [slo]") {
		t.Error("expected alert name with group")
	}
	if !strings.Contains(out, "http_requests_total: kept") {
		t.Error("expected metric detail line")
	}
	if !strings.Contains(out, "5 alerts") {
		t.Error("expected summary line")
	}
}

func TestTextFormatter_AlertStatusBadge(t *testing.T) {
	f := &TextFormatter{}
	// Without color — plain labels.
	for _, status := range []alertcoverage.AlertStatus{
		alertcoverage.AlertSafe,
		alertcoverage.AlertBroken,
		alertcoverage.AlertAtRisk,
		alertcoverage.AlertWouldActivate,
		alertcoverage.AlertUnknown,
	} {
		badge := f.alertStatusBadge(status, false)
		if badge != strings.ToUpper(string(status)) {
			t.Errorf("no-color badge for %q: got %q", status, badge)
		}
	}
	// With color — should contain ANSI codes.
	for _, status := range []alertcoverage.AlertStatus{
		alertcoverage.AlertSafe,
		alertcoverage.AlertBroken,
		alertcoverage.AlertAtRisk,
		alertcoverage.AlertWouldActivate,
		alertcoverage.AlertUnknown,
	} {
		badge := f.alertStatusBadge(status, true)
		if !strings.Contains(badge, "\033[") {
			t.Errorf("color badge for %q should contain ANSI codes", status)
		}
	}
}

// --- SARIF no-location finding ---

func TestSARIFFormatter_NoLocation(t *testing.T) {
	rpt := &analyze.Report{
		Findings: []rules.Finding{
			{RuleID: "global-rule", Title: "Title", Severity: rules.SeverityInfo, Implication: "global"},
		},
		Summary: analyze.Summary{Total: 1, Info: 1},
	}
	var buf bytes.Buffer
	f := &SARIFFormatter{}
	if err := f.Format(rpt, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(buf.Bytes(), &parsed)
	runs := parsed["runs"].([]any)
	results := runs[0].(map[string]any)["results"].([]any)
	first := results[0].(map[string]any)
	if _, ok := first["locations"]; ok {
		t.Error("expected no locations for finding without scope")
	}
}
