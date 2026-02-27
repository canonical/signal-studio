package filter

import "strings"

// parseOTTLExpression attempts to extract a FilterRule from an OTTL expression string.
// It tries all attribute-based parsers first, then falls back to the existing name-based
// parser (parseOTTLNameExpression).
func parseOTTLExpression(expr string) FilterRule {
	expr = strings.TrimSpace(expr)

	// Compound expressions (using "and" / "or") are not supported.
	lower := strings.ToLower(expr)
	if strings.Contains(lower, " and ") || strings.Contains(lower, " or ") {
		return FilterRule{
			Raw:       expr,
			Action:    ActionDrop,
			MatchType: MatchTypeUnsupported,
		}
	}

	// Try attribute-based parsers in order.
	if rule, ok := parseResourceAttrEquals(expr); ok {
		return rule
	}
	if rule, ok := parseDatapointAttrEquals(expr); ok {
		return rule
	}
	if rule, ok := parseIsMatchResourceAttr(expr); ok {
		return rule
	}
	if rule, ok := parseIsMatchDatapointAttr(expr); ok {
		return rule
	}
	if rule, ok := parseHasAttrKeyOnDatapoint(expr); ok {
		return rule
	}
	if rule, ok := parseHasAttrOnDatapoint(expr); ok {
		return rule
	}

	// Fall back to name-based parsing (name == "x", IsMatch(name, "x")).
	return parseOTTLNameExpression(expr)
}

// parseResourceAttrEquals parses: resource.attributes["key"] == "value"
func parseResourceAttrEquals(expr string) (FilterRule, bool) {
	const prefix = "resource.attributes["
	if !strings.HasPrefix(expr, prefix) {
		return FilterRule{}, false
	}

	rest := expr[len(prefix):]
	key, after, ok := extractBracketedAttrKey(rest)
	if !ok {
		return FilterRule{}, false
	}

	after = strings.TrimSpace(after)
	if !strings.HasPrefix(after, "==") {
		return FilterRule{}, false
	}
	after = strings.TrimSpace(after[2:])

	val, ok := unquote(after)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLResourceAttr,
		Pattern:   val,
		AttrKey:   key,
		AttrValue: val,
	}, true
}

// parseDatapointAttrEquals parses: attributes["key"] == "value"
// It must not match resource.attributes (handled separately).
func parseDatapointAttrEquals(expr string) (FilterRule, bool) {
	const prefix = "attributes["
	if !strings.HasPrefix(expr, prefix) {
		return FilterRule{}, false
	}
	// Guard against matching "resource.attributes[" which would have been
	// handled by parseResourceAttrEquals already, but be defensive.
	if strings.HasPrefix(expr, "resource.") {
		return FilterRule{}, false
	}

	rest := expr[len(prefix):]
	key, after, ok := extractBracketedAttrKey(rest)
	if !ok {
		return FilterRule{}, false
	}

	after = strings.TrimSpace(after)
	if !strings.HasPrefix(after, "==") {
		return FilterRule{}, false
	}
	after = strings.TrimSpace(after[2:])

	val, ok := unquote(after)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLDatapointAttr,
		Pattern:   val,
		AttrKey:   key,
		AttrValue: val,
	}, true
}

// parseIsMatchResourceAttr parses: IsMatch(resource.attributes["key"], "pattern")
func parseIsMatchResourceAttr(expr string) (FilterRule, bool) {
	const prefix = "IsMatch(resource.attributes["
	if !strings.HasPrefix(expr, prefix) {
		return FilterRule{}, false
	}
	if !strings.HasSuffix(expr, ")") {
		return FilterRule{}, false
	}

	// Strip outer IsMatch( ... )
	inner := expr[len("IsMatch(") : len(expr)-1]

	// inner starts with: resource.attributes["key"], "pattern"
	const raPrefix = "resource.attributes["
	if !strings.HasPrefix(inner, raPrefix) {
		return FilterRule{}, false
	}

	rest := inner[len(raPrefix):]
	key, after, ok := extractBracketedAttrKey(rest)
	if !ok {
		return FilterRule{}, false
	}

	after = strings.TrimSpace(after)
	if !strings.HasPrefix(after, ",") {
		return FilterRule{}, false
	}
	after = strings.TrimSpace(after[1:])

	pattern, ok := unquote(after)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLResourceAttrMatch,
		Pattern:   pattern,
		AttrKey:   key,
	}, true
}

// parseIsMatchDatapointAttr parses: IsMatch(attributes["key"], "pattern")
func parseIsMatchDatapointAttr(expr string) (FilterRule, bool) {
	const prefix = "IsMatch(attributes["
	if !strings.HasPrefix(expr, prefix) {
		return FilterRule{}, false
	}
	// Guard against "IsMatch(resource.attributes[..."
	if strings.HasPrefix(expr, "IsMatch(resource.") {
		return FilterRule{}, false
	}
	if !strings.HasSuffix(expr, ")") {
		return FilterRule{}, false
	}

	inner := expr[len("IsMatch(") : len(expr)-1]

	const aPrefix = "attributes["
	if !strings.HasPrefix(inner, aPrefix) {
		return FilterRule{}, false
	}

	rest := inner[len(aPrefix):]
	key, after, ok := extractBracketedAttrKey(rest)
	if !ok {
		return FilterRule{}, false
	}

	after = strings.TrimSpace(after)
	if !strings.HasPrefix(after, ",") {
		return FilterRule{}, false
	}
	after = strings.TrimSpace(after[1:])

	pattern, ok := unquote(after)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLDatapointAttrMatch,
		Pattern:   pattern,
		AttrKey:   key,
	}, true
}

// parseHasAttrKeyOnDatapoint parses: HasAttrKeyOnDatapoint("key")
func parseHasAttrKeyOnDatapoint(expr string) (FilterRule, bool) {
	const prefix = "HasAttrKeyOnDatapoint("
	if !strings.HasPrefix(expr, prefix) {
		return FilterRule{}, false
	}
	if !strings.HasSuffix(expr, ")") {
		return FilterRule{}, false
	}

	inner := expr[len(prefix) : len(expr)-1]
	inner = strings.TrimSpace(inner)

	key, ok := unquote(inner)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLHasAttrKey,
		AttrKey:   key,
	}, true
}

// parseHasAttrOnDatapoint parses: HasAttrOnDatapoint("key", "value")
func parseHasAttrOnDatapoint(expr string) (FilterRule, bool) {
	const prefix = "HasAttrOnDatapoint("
	if !strings.HasPrefix(expr, prefix) {
		return FilterRule{}, false
	}
	if !strings.HasSuffix(expr, ")") {
		return FilterRule{}, false
	}

	inner := expr[len(prefix) : len(expr)-1]

	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return FilterRule{}, false
	}

	keyStr := strings.TrimSpace(parts[0])
	valStr := strings.TrimSpace(parts[1])

	key, ok := unquote(keyStr)
	if !ok {
		return FilterRule{}, false
	}

	val, ok := unquote(valStr)
	if !ok {
		return FilterRule{}, false
	}

	return FilterRule{
		Raw:       expr,
		Action:    ActionDrop,
		MatchType: MatchTypeOTTLHasAttr,
		AttrKey:   key,
		AttrValue: val,
	}, true
}

// extractBracketedAttrKey extracts the attribute key from a string that starts
// immediately after the opening bracket, e.g. for input `"key"]...`, it returns
// the key, the remaining string after the `]`, and whether extraction succeeded.
// Supports both double and single quotes: "key"] or 'key'].
func extractBracketedAttrKey(s string) (string, string, bool) {
	// s should start with a quoted key followed by ]
	// e.g.: "service.name"] == "frontend"
	// or:   'service.name'] == "frontend"
	if len(s) < 3 {
		return "", "", false
	}

	quote := s[0]
	if quote != '"' && quote != '\'' {
		return "", "", false
	}

	// Find the closing quote
	closeQuote := strings.IndexByte(s[1:], quote)
	if closeQuote < 0 {
		return "", "", false
	}
	closeQuote++ // adjust for the offset of 1

	key := s[1:closeQuote]

	// Next character must be ]
	rest := s[closeQuote+1:]
	if len(rest) == 0 || rest[0] != ']' {
		return "", "", false
	}

	return key, rest[1:], true
}
