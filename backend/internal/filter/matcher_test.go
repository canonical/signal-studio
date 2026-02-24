package filter

import "testing"

func TestAnalyzeFilter_LegacyInclude(t *testing.T) {
	fc := FilterConfig{
		ProcessorName: "filter",
		Pipeline:      "metrics",
		Style:         "legacy",
		Rules: []FilterRule{
			{Raw: `http\.server\..*`, Action: ActionInclude, MatchType: MatchTypeRegexp, Pattern: `http\.server\..*`},
		},
	}

	names := []string{"http.server.duration", "http.server.requests", "system.cpu.time"}
	fa := AnalyzeFilter(fc, names)

	if fa.KeptCount != 2 {
		t.Errorf("expected 2 kept, got %d", fa.KeptCount)
	}
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
	if fa.ProcessorName != "filter" {
		t.Errorf("expected processor name filter, got %s", fa.ProcessorName)
	}
	if fa.Pipeline != "metrics" {
		t.Errorf("expected pipeline metrics, got %s", fa.Pipeline)
	}
}

func TestAnalyzeFilter_LegacyExclude(t *testing.T) {
	fc := FilterConfig{
		ProcessorName: "filter/drop",
		Pipeline:      "metrics",
		Style:         "legacy",
		Rules: []FilterRule{
			{Raw: "system.cpu.time", Action: ActionExclude, MatchType: MatchTypeStrict, Pattern: "system.cpu.time"},
		},
	}

	names := []string{"http.server.duration", "system.cpu.time", "system.memory.usage"}
	fa := AnalyzeFilter(fc, names)

	if fa.KeptCount != 2 {
		t.Errorf("expected 2 kept, got %d", fa.KeptCount)
	}
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
}

func TestAnalyzeFilter_LegacyBoth(t *testing.T) {
	fc := FilterConfig{
		ProcessorName: "filter",
		Pipeline:      "metrics",
		Style:         "legacy",
		Rules: []FilterRule{
			{Raw: `http\..*`, Action: ActionInclude, MatchType: MatchTypeRegexp, Pattern: `http\..*`},
			{Raw: "http.client.duration", Action: ActionExclude, MatchType: MatchTypeStrict, Pattern: "http.client.duration"},
		},
	}

	names := []string{"http.server.duration", "http.client.duration", "system.cpu.time"}
	fa := AnalyzeFilter(fc, names)

	// http.server.duration: included, not excluded → kept
	// http.client.duration: included, then excluded → dropped
	// system.cpu.time: not included → dropped
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
	if fa.DroppedCount != 2 {
		t.Errorf("expected 2 dropped, got %d", fa.DroppedCount)
	}
}

func TestAnalyzeFilter_StrictMatch(t *testing.T) {
	fc := FilterConfig{
		Style: "legacy",
		Rules: []FilterRule{
			{Action: ActionExclude, MatchType: MatchTypeStrict, Pattern: "exact.name"},
		},
	}

	names := []string{"exact.name", "exact.name.suffix", "prefix.exact.name"}
	fa := AnalyzeFilter(fc, names)

	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped (strict match), got %d", fa.DroppedCount)
	}
}

func TestAnalyzeFilter_RegexpMatch(t *testing.T) {
	fc := FilterConfig{
		Style: "legacy",
		Rules: []FilterRule{
			{Action: ActionExclude, MatchType: MatchTypeRegexp, Pattern: `system\..*`},
		},
	}

	names := []string{"system.cpu.time", "system.memory.usage", "http.server.duration"}
	fa := AnalyzeFilter(fc, names)

	if fa.DroppedCount != 2 {
		t.Errorf("expected 2 dropped (regexp match), got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilter_OTTLNameEq(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `name == "http.server.duration"`, Action: ActionDrop, MatchType: MatchTypeOTTLNameEq, Pattern: "http.server.duration"},
		},
	}

	names := []string{"http.server.duration", "http.client.duration"}
	fa := AnalyzeFilter(fc, names)

	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilter_OTTLIsMatch(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `IsMatch(name, "system\.cpu\..*")`, Action: ActionDrop, MatchType: MatchTypeOTTLIsMatch, Pattern: `system\.cpu\..*`},
		},
	}

	names := []string{"system.cpu.time", "system.cpu.utilization", "system.memory.usage"}
	fa := AnalyzeFilter(fc, names)

	if fa.DroppedCount != 2 {
		t.Errorf("expected 2 dropped, got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilter_OTTLUnsupported(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `attributes["key"] == "value"`, Action: ActionDrop, MatchType: MatchTypeUnsupported, Pattern: ""},
		},
	}

	names := []string{"any.metric"}
	fa := AnalyzeFilter(fc, names)

	if fa.UnknownCount != 1 {
		t.Errorf("expected 1 unknown, got %d", fa.UnknownCount)
	}
	if !fa.HasUnsupported {
		t.Error("expected HasUnsupported to be true")
	}
}

func TestAnalyzeFilter_EmptyInputs(t *testing.T) {
	fc := FilterConfig{
		Style: "legacy",
		Rules: []FilterRule{
			{Action: ActionExclude, MatchType: MatchTypeStrict, Pattern: "foo"},
		},
	}

	fa := AnalyzeFilter(fc, nil)
	if fa.KeptCount != 0 && fa.DroppedCount != 0 {
		t.Error("expected zero counts for empty input")
	}
}

func TestAnalyzeFilter_NoRules(t *testing.T) {
	fc := FilterConfig{
		Style: "legacy",
		Rules: nil,
	}

	names := []string{"m1", "m2"}
	fa := AnalyzeFilter(fc, names)

	// No include rules = keep all, no exclude rules = drop none
	if fa.KeptCount != 2 {
		t.Errorf("expected 2 kept with no rules, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilter_Counts(t *testing.T) {
	fc := FilterConfig{
		Style: "legacy",
		Rules: []FilterRule{
			{Action: ActionInclude, MatchType: MatchTypeRegexp, Pattern: `keep\..*`},
		},
	}

	names := []string{"keep.a", "keep.b", "drop.c", "drop.d", "drop.e"}
	fa := AnalyzeFilter(fc, names)

	if fa.KeptCount != 2 {
		t.Errorf("expected kept=2, got %d", fa.KeptCount)
	}
	if fa.DroppedCount != 3 {
		t.Errorf("expected dropped=3, got %d", fa.DroppedCount)
	}
	total := fa.KeptCount + fa.DroppedCount + fa.UnknownCount
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
}
