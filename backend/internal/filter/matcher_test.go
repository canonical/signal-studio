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

// --- AnalyzeFilterWithAttributes tests ---

func TestAnalyzeFilterWithAttributes_ResourceAttrMatch(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `resource.attributes["service.name"] == "frontend"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLResourceAttr, AttrKey: "service.name", AttrValue: "frontend"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "http.server.duration", Attributes: []AttrMeta{
			{Key: "service.name", Level: "resource", SampleValues: []string{"frontend", "backend"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
}

func TestAnalyzeFilterWithAttributes_ResourceAttrNoMatch(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `resource.attributes["service.name"] == "frontend"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLResourceAttr, AttrKey: "service.name", AttrValue: "frontend"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "http.server.duration", Attributes: []AttrMeta{
			{Key: "service.name", Level: "resource", SampleValues: []string{"backend"}, Capped: false},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilterWithAttributes_CappedUnknown(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `resource.attributes["service.name"] == "mystery"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLResourceAttr, AttrKey: "service.name", AttrValue: "mystery"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "http.server.duration", Attributes: []AttrMeta{
			{Key: "service.name", Level: "resource", SampleValues: []string{"a", "b"}, Capped: true},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.UnknownCount != 1 {
		t.Errorf("expected 1 unknown (capped), got %d", fa.UnknownCount)
	}
}

func TestAnalyzeFilterWithAttributes_DatapointAttr(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `attributes["http.method"] == "GET"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLDatapointAttr, AttrKey: "http.method", AttrValue: "GET"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "http.server.duration", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"GET", "POST"}},
		}},
		{Name: "system.cpu.time", Attributes: []AttrMeta{
			{Key: "cpu", Level: "datapoint", SampleValues: []string{"0", "1"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	// "GET" matches 1 of 2 values → partial, not dropped
	if fa.PartialCount != 1 {
		t.Errorf("expected 1 partial, got %d", fa.PartialCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
	// Check droppedRatio
	for _, r := range fa.Results {
		if r.MetricName == "http.server.duration" {
			if r.Outcome != OutcomePartial {
				t.Errorf("expected partial for http.server.duration, got %s", r.Outcome)
			}
			if r.DroppedRatio < 0.49 || r.DroppedRatio > 0.51 {
				t.Errorf("expected droppedRatio ~0.5, got %f", r.DroppedRatio)
			}
		}
	}
}

func TestAnalyzeFilterWithAttributes_RegexMatch(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `IsMatch(resource.attributes["service.name"], "front.*")`, Action: ActionDrop,
				MatchType: MatchTypeOTTLResourceAttrMatch, AttrKey: "service.name", Pattern: "front.*"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "m1", Attributes: []AttrMeta{
			{Key: "service.name", Level: "resource", SampleValues: []string{"frontend"}},
		}},
		{Name: "m2", Attributes: []AttrMeta{
			{Key: "service.name", Level: "resource", SampleValues: []string{"backend"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilterWithAttributes_HasAttrKey(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `HasAttrKeyOnDatapoint("http.method")`, Action: ActionDrop,
				MatchType: MatchTypeOTTLHasAttrKey, AttrKey: "http.method"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "http.server.duration", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"GET"}},
		}},
		{Name: "system.cpu.time", Attributes: []AttrMeta{
			{Key: "cpu", Level: "datapoint", SampleValues: []string{"0"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilterWithAttributes_HasAttr(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `HasAttrOnDatapoint("http.method", "GET")`, Action: ActionDrop,
				MatchType: MatchTypeOTTLHasAttr, AttrKey: "http.method", AttrValue: "GET"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "http.server.duration", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"GET", "POST"}},
		}},
		{Name: "http.client.duration", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"POST"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	// "GET" matches 1 of 2 values on http.server.duration → partial
	if fa.PartialCount != 1 {
		t.Errorf("expected 1 partial, got %d", fa.PartialCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilterWithAttributes_MixedRules(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `name == "system.cpu.time"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLNameEq, Pattern: "system.cpu.time"},
			{Raw: `attributes["http.method"] == "DELETE"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLDatapointAttr, AttrKey: "http.method", AttrValue: "DELETE"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "system.cpu.time", Attributes: nil},
		{Name: "http.server.duration", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"DELETE"}},
		}},
		{Name: "http.client.duration", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"GET"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.DroppedCount != 2 {
		t.Errorf("expected 2 dropped, got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

func TestAnalyzeFilterWithAttributes_BackwardCompat(t *testing.T) {
	// AnalyzeFilter (name-only) should still work unchanged
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `name == "http.server.duration"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLNameEq, Pattern: "http.server.duration"},
		},
	}
	names := []string{"http.server.duration", "other.metric"}
	fa := AnalyzeFilter(fc, names)
	if fa.DroppedCount != 1 || fa.KeptCount != 1 {
		t.Errorf("expected dropped=1 kept=1, got dropped=%d kept=%d", fa.DroppedCount, fa.KeptCount)
	}
}

func TestAnalyzeFilterWithAttributes_DatapointAttrRegex(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `IsMatch(attributes["http.method"], "GET|POST")`, Action: ActionDrop,
				MatchType: MatchTypeOTTLDatapointAttrMatch, AttrKey: "http.method", Pattern: "GET|POST"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "m1", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"GET"}},
		}},
		{Name: "m2", Attributes: []AttrMeta{
			{Key: "http.method", Level: "datapoint", SampleValues: []string{"DELETE"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
}

// --- Partial outcome tests ---

func TestPartial_DatapointRegex_SomeMatch(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `IsMatch(attributes["device"], "^loop\\d+$")`, Action: ActionDrop,
				MatchType: MatchTypeOTTLDatapointAttrMatch, AttrKey: "device", Pattern: `loop\d+`},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "system.disk.io", Attributes: []AttrMeta{
			{Key: "device", Level: "datapoint", SampleValues: []string{"sda", "loop0", "loop1", "nvme0n1"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.PartialCount != 1 {
		t.Errorf("expected 1 partial, got %d", fa.PartialCount)
	}
	r := fa.Results[0]
	if r.Outcome != OutcomePartial {
		t.Errorf("expected partial, got %s", r.Outcome)
	}
	// 2 of 4 match → ratio 0.5
	if r.DroppedRatio < 0.49 || r.DroppedRatio > 0.51 {
		t.Errorf("expected droppedRatio ~0.5, got %f", r.DroppedRatio)
	}
}

func TestPartial_AllMatchNotCapped_Dropped(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `attributes["env"] == "staging"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLDatapointAttr, AttrKey: "env", AttrValue: "staging"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "m1", Attributes: []AttrMeta{
			{Key: "env", Level: "datapoint", SampleValues: []string{"staging"}, Capped: false},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped (all match, not capped), got dropped=%d partial=%d", fa.DroppedCount, fa.PartialCount)
	}
}

func TestPartial_AllMatchButCapped_Partial(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `attributes["env"] == "staging"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLDatapointAttr, AttrKey: "env", AttrValue: "staging"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "m1", Attributes: []AttrMeta{
			// All sampled values match but capped → there may be unmatched values
			{Key: "env", Level: "datapoint", SampleValues: []string{"staging"}, Capped: true},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.PartialCount != 1 {
		t.Errorf("expected 1 partial (all sampled match but capped), got partial=%d dropped=%d", fa.PartialCount, fa.DroppedCount)
	}
}

func TestPartial_CappedSomeMatch_Partial(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `attributes["method"] == "GET"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLDatapointAttr, AttrKey: "method", AttrValue: "GET"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "m1", Attributes: []AttrMeta{
			{Key: "method", Level: "datapoint", SampleValues: []string{"GET", "POST", "DELETE"}, Capped: true},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.PartialCount != 1 {
		t.Errorf("expected 1 partial, got partial=%d", fa.PartialCount)
	}
	r := fa.Results[0]
	// 1 of 3 sampled match → ratio ~0.333
	if r.DroppedRatio < 0.32 || r.DroppedRatio > 0.34 {
		t.Errorf("expected droppedRatio ~0.333, got %f", r.DroppedRatio)
	}
}

func TestPartial_ResourceStaysDropped(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `resource.attributes["service.name"] == "frontend"`, Action: ActionDrop,
				MatchType: MatchTypeOTTLResourceAttr, AttrKey: "service.name", AttrValue: "frontend"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "m1", Attributes: []AttrMeta{
			// 1 of 2 values match, but resource-level → dropped (not partial)
			{Key: "service.name", Level: "resource", SampleValues: []string{"frontend", "backend"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped (resource-level stays binary), got dropped=%d partial=%d", fa.DroppedCount, fa.PartialCount)
	}
}

func TestPartial_HasAttrKey_StaysBinary(t *testing.T) {
	fc := FilterConfig{
		Style: "ottl",
		Rules: []FilterRule{
			{Raw: `HasAttrKeyOnDatapoint("device")`, Action: ActionDrop,
				MatchType: MatchTypeOTTLHasAttrKey, AttrKey: "device"},
		},
	}
	infos := []MetricAttributeInfo{
		{Name: "m1", Attributes: []AttrMeta{
			{Key: "device", Level: "datapoint", SampleValues: []string{"sda", "loop0"}},
		}},
		{Name: "m2", Attributes: []AttrMeta{
			{Key: "cpu", Level: "datapoint", SampleValues: []string{"0"}},
		}},
	}
	fa := AnalyzeFilterWithAttributes(fc, infos)
	// HasAttrKey is binary: key exists → dropped, key absent → kept
	if fa.DroppedCount != 1 {
		t.Errorf("expected 1 dropped, got %d", fa.DroppedCount)
	}
	if fa.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", fa.KeptCount)
	}
	if fa.PartialCount != 0 {
		t.Errorf("expected 0 partial (HasAttrKey is binary), got %d", fa.PartialCount)
	}
}
