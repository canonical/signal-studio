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
		`attributes["key"] == "value"`,
		`HasAttrKeyOnDatapoint("http.method")`,
		`resource.attributes["service.name"] == "foo"`,
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
