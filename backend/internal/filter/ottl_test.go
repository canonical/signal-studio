package filter

import "testing"

func TestParseOTTLNameEquals(t *testing.T) {
	tests := []struct {
		expr    string
		want    MatchType
		pattern string
	}{
		{`name == "http.server.duration"`, MatchTypeOTTLNameEq, "http.server.duration"},
		{`name == 'http.server.duration'`, MatchTypeOTTLNameEq, "http.server.duration"},
		{`name  ==  "spaced"`, MatchTypeOTTLNameEq, "spaced"},
		{`  name == "trimmed"  `, MatchTypeOTTLNameEq, "trimmed"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			rule := parseOTTLNameExpression(tt.expr)
			if rule.MatchType != tt.want {
				t.Errorf("expected match type %s, got %s", tt.want, rule.MatchType)
			}
			if rule.Pattern != tt.pattern {
				t.Errorf("expected pattern %q, got %q", tt.pattern, rule.Pattern)
			}
			if rule.Action != ActionDrop {
				t.Errorf("expected action drop, got %s", rule.Action)
			}
		})
	}
}

func TestParseOTTLIsMatch(t *testing.T) {
	tests := []struct {
		expr    string
		want    MatchType
		pattern string
	}{
		{`IsMatch(name, "system\.cpu\..*")`, MatchTypeOTTLIsMatch, `system\.cpu\..*`},
		{`IsMatch(name, 'http\.server\..*')`, MatchTypeOTTLIsMatch, `http\.server\..*`},
		// Double-backslash from YAML (e.g. '...\\d+...') should unescape to single backslash
		{"IsMatch(name, \"^otelcol_\\\\d+$\")", MatchTypeOTTLIsMatch, `^otelcol_\d+$`},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			rule := parseOTTLNameExpression(tt.expr)
			if rule.MatchType != tt.want {
				t.Errorf("expected match type %s, got %s", tt.want, rule.MatchType)
			}
			if rule.Pattern != tt.pattern {
				t.Errorf("expected pattern %q, got %q", tt.pattern, rule.Pattern)
			}
		})
	}
}

func TestParseOTTLUnsupported(t *testing.T) {
	tests := []string{
		`type == METRIC_DATA_TYPE_HISTOGRAM`,
		``,
	}

	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			rule := parseOTTLNameExpression(expr)
			if rule.MatchType != MatchTypeUnsupported {
				t.Errorf("expected unsupported for %q, got %s", expr, rule.MatchType)
			}
		})
	}
}

func TestParseOTTLResourceAttrEquals(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		key     string
		value   string
	}{
		{
			name:  "double quotes",
			expr:  `resource.attributes["service.name"] == "frontend"`,
			key:   "service.name",
			value: "frontend",
		},
		{
			name:  "single quotes",
			expr:  `resource.attributes['service.name'] == 'frontend'`,
			key:   "service.name",
			value: "frontend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parseOTTLExpression(tt.expr)
			if rule.MatchType != MatchTypeOTTLResourceAttr {
				t.Errorf("expected %s, got %s", MatchTypeOTTLResourceAttr, rule.MatchType)
			}
			if rule.AttrKey != tt.key {
				t.Errorf("expected attrKey %q, got %q", tt.key, rule.AttrKey)
			}
			if rule.AttrValue != tt.value {
				t.Errorf("expected attrValue %q, got %q", tt.value, rule.AttrValue)
			}
			if rule.Pattern != tt.value {
				t.Errorf("expected pattern %q, got %q", tt.value, rule.Pattern)
			}
			if rule.Action != ActionDrop {
				t.Errorf("expected action drop, got %s", rule.Action)
			}
		})
	}
}

func TestParseOTTLDatapointAttrEquals(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		key     string
		value   string
	}{
		{
			name:  "double quotes",
			expr:  `attributes["http.method"] == "GET"`,
			key:   "http.method",
			value: "GET",
		},
		{
			name:  "single quotes",
			expr:  `attributes['http.method'] == 'GET'`,
			key:   "http.method",
			value: "GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parseOTTLExpression(tt.expr)
			if rule.MatchType != MatchTypeOTTLDatapointAttr {
				t.Errorf("expected %s, got %s", MatchTypeOTTLDatapointAttr, rule.MatchType)
			}
			if rule.AttrKey != tt.key {
				t.Errorf("expected attrKey %q, got %q", tt.key, rule.AttrKey)
			}
			if rule.AttrValue != tt.value {
				t.Errorf("expected attrValue %q, got %q", tt.value, rule.AttrValue)
			}
			if rule.Action != ActionDrop {
				t.Errorf("expected action drop, got %s", rule.Action)
			}
		})
	}
}

func TestParseOTTLIsMatchResourceAttr(t *testing.T) {
	expr := `IsMatch(resource.attributes["service.name"], "front.*")`
	rule := parseOTTLExpression(expr)

	if rule.MatchType != MatchTypeOTTLResourceAttrMatch {
		t.Errorf("expected %s, got %s", MatchTypeOTTLResourceAttrMatch, rule.MatchType)
	}
	if rule.AttrKey != "service.name" {
		t.Errorf("expected attrKey %q, got %q", "service.name", rule.AttrKey)
	}
	if rule.Pattern != "front.*" {
		t.Errorf("expected pattern %q, got %q", "front.*", rule.Pattern)
	}
	if rule.Action != ActionDrop {
		t.Errorf("expected action drop, got %s", rule.Action)
	}
}

func TestParseOTTLIsMatchDatapointAttr(t *testing.T) {
	expr := `IsMatch(attributes["http.method"], "GET|POST")`
	rule := parseOTTLExpression(expr)

	if rule.MatchType != MatchTypeOTTLDatapointAttrMatch {
		t.Errorf("expected %s, got %s", MatchTypeOTTLDatapointAttrMatch, rule.MatchType)
	}
	if rule.AttrKey != "http.method" {
		t.Errorf("expected attrKey %q, got %q", "http.method", rule.AttrKey)
	}
	if rule.Pattern != "GET|POST" {
		t.Errorf("expected pattern %q, got %q", "GET|POST", rule.Pattern)
	}
	if rule.Action != ActionDrop {
		t.Errorf("expected action drop, got %s", rule.Action)
	}
}

func TestParseOTTLHasAttrKeyOnDatapoint(t *testing.T) {
	expr := `HasAttrKeyOnDatapoint("http.method")`
	rule := parseOTTLExpression(expr)

	if rule.MatchType != MatchTypeOTTLHasAttrKey {
		t.Errorf("expected %s, got %s", MatchTypeOTTLHasAttrKey, rule.MatchType)
	}
	if rule.AttrKey != "http.method" {
		t.Errorf("expected attrKey %q, got %q", "http.method", rule.AttrKey)
	}
	if rule.Action != ActionDrop {
		t.Errorf("expected action drop, got %s", rule.Action)
	}
}

func TestParseOTTLHasAttrOnDatapoint(t *testing.T) {
	expr := `HasAttrOnDatapoint("http.method", "GET")`
	rule := parseOTTLExpression(expr)

	if rule.MatchType != MatchTypeOTTLHasAttr {
		t.Errorf("expected %s, got %s", MatchTypeOTTLHasAttr, rule.MatchType)
	}
	if rule.AttrKey != "http.method" {
		t.Errorf("expected attrKey %q, got %q", "http.method", rule.AttrKey)
	}
	if rule.AttrValue != "GET" {
		t.Errorf("expected attrValue %q, got %q", "GET", rule.AttrValue)
	}
	if rule.Action != ActionDrop {
		t.Errorf("expected action drop, got %s", rule.Action)
	}
}

func TestParseOTTLExpression_FallbackToName(t *testing.T) {
	expr := `name == "foo"`
	rule := parseOTTLExpression(expr)

	if rule.MatchType != MatchTypeOTTLNameEq {
		t.Errorf("expected %s, got %s", MatchTypeOTTLNameEq, rule.MatchType)
	}
	if rule.Pattern != "foo" {
		t.Errorf("expected pattern %q, got %q", "foo", rule.Pattern)
	}
	if rule.Action != ActionDrop {
		t.Errorf("expected action drop, got %s", rule.Action)
	}
}

func TestParseOTTLExpression_CompoundRemains_Unsupported(t *testing.T) {
	expr := `name == "x" and attributes["y"] == "z"`
	rule := parseOTTLExpression(expr)

	if rule.MatchType != MatchTypeUnsupported {
		t.Errorf("expected %s, got %s", MatchTypeUnsupported, rule.MatchType)
	}
}
