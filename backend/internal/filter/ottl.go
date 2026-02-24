package filter

import "strings"

// parseOTTLNameExpression attempts to extract a FilterRule from an OTTL expression string.
// Supported forms:
//   - name == "literal"  /  name == 'literal'
//   - IsMatch(name, "pattern")  /  IsMatch(name, 'pattern')
//
// Everything else results in MatchType "unsupported".
func parseOTTLNameExpression(expr string) FilterRule {
	expr = strings.TrimSpace(expr)

	// Try name == "literal" / name == 'literal'
	if rule, ok := parseNameEquals(expr); ok {
		return rule
	}

	// Try IsMatch(name, "pattern") / IsMatch(name, 'pattern')
	if rule, ok := parseIsMatch(expr); ok {
		return rule
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeUnsupported,
		Pattern:   "",
	}
}

// parseNameEquals tries to parse: name == "value" or name == 'value'
func parseNameEquals(expr string) (FilterRule, bool) {
	// Normalize whitespace around ==
	parts := strings.SplitN(expr, "==", 2)
	if len(parts) != 2 {
		return FilterRule{}, false
	}

	lhs := strings.TrimSpace(parts[0])
	rhs := strings.TrimSpace(parts[1])

	if lhs != "name" {
		return FilterRule{}, false
	}

	val, ok := unquote(rhs)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLNameEq,
		Pattern:   val,
	}, true
}

// parseIsMatch tries to parse: IsMatch(name, "pattern") or IsMatch(name, 'pattern')
func parseIsMatch(expr string) (FilterRule, bool) {
	// Case-sensitive check for IsMatch(
	idx := strings.Index(expr, "IsMatch(")
	if idx != 0 {
		return FilterRule{}, false
	}

	// Must end with )
	if !strings.HasSuffix(expr, ")") {
		return FilterRule{}, false
	}

	inner := expr[len("IsMatch(") : len(expr)-1]
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return FilterRule{}, false
	}

	arg1 := strings.TrimSpace(parts[0])
	arg2 := strings.TrimSpace(parts[1])

	if arg1 != "name" {
		return FilterRule{}, false
	}

	pattern, ok := unquote(arg2)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLIsMatch,
		Pattern:   pattern,
	}, true
}

// unquote removes surrounding double or single quotes from s.
func unquote(s string) (string, bool) {
	if len(s) < 2 {
		return "", false
	}
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1], true
	}
	return "", false
}
