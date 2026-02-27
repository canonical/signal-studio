# absent() and absent_over_time() Detection

**Package:** `github.com/canonical/signal-studio/internal/alertcoverage`
**Source files:** `extract.go` (detection), `coverage.go` (status derivation)
**ADR:** [ADR-0018: Alert Rule Coverage Analysis](../adr/0018-alert-rule-coverage-analysis.md)

## Purpose

`absent()` and `absent_over_time()` are PromQL functions that return a value
when a metric is **missing**. This creates a semantic inversion: if a filter
drops a metric, an `absent()`-based alert **activates** rather than **breaks**.
The alert coverage system must detect this pattern and report it as a distinct
status (`would_activate`) so operators understand the true impact of their
filter configuration.

## The Semantic Inversion

### Normal alerts

A typical alert like `rate(http_requests_total[5m]) > 0.05` depends on the
metric being present. If the filter drops `http_requests_total`, the alert
cannot evaluate and effectively breaks.

### absent() alerts

An alert like `absent(up{job="api"})` fires when `up` is **not** present. If
the filter drops `up`, the `absent()` condition becomes true and the alert
**starts firing**. This is the opposite of a normal alert breaking — the alert
becomes perpetually active, which may trigger pages, Slack notifications, or
PagerDuty incidents.

This distinction matters because:
- A **broken** alert is silently dangerous — it cannot fire when needed.
- A **would-activate** alert is noisily dangerous — it fires continuously
  without a real incident, causing alert fatigue and potentially masking real
  problems.

## Detection Mechanism

### AST Walking (extract.go)

During the `VisitAll` traversal in `ExtractMetrics`, the visitor callback checks
every `*metricsql.FuncExpr` node:

```go
case *metricsql.FuncExpr:
    lower := strings.ToLower(t.Name)
    if lower == "absent" || lower == "absent_over_time" {
        usesAbsent = true
    }
```

Key design decisions:

1. **Case-insensitive matching** — `strings.ToLower` is applied to the function
   name. While PromQL function names are conventionally lowercase, the parser
   may preserve case from user input.

2. **Expression-level flag** — `usesAbsent` is a single boolean for the entire
   expression, not per-metric. If any part of a PromQL expression uses
   `absent()`, the entire expression is flagged. This is a deliberate
   simplification: in practice, expressions that mix `absent()` with normal
   selectors (e.g., `absent(up) or rate(errors[5m]) > 0`) are rare, and
   flagging the entire alert as `would_activate` errs on the side of
   visibility.

3. **No nesting depth tracking** — the walker does not track whether `absent()`
   is at the top level or nested inside another function (e.g.,
   `count(absent(up))`). Any occurrence anywhere in the AST sets the flag.
   This is correct because `absent()` inverts semantics regardless of where
   it appears in the expression tree.

### Propagation Path

The `usesAbsent` flag flows through the system as follows:

```
ExtractMetrics(expr)
    → returns (names, usesAbsent, err)
        → stored in AlertRule.UsesAbsent
            → passed to deriveStatus(metrics, usesAbsent)
                → influences AlertStatus determination
```

## Status Derivation (coverage.go)

The `deriveStatus` function in the cross-reference engine uses `UsesAbsent` to
determine the alert status when a metric is dropped:

```go
if hasDropped {
    if usesAbsent {
        return AlertWouldActivate
    }
    return AlertBroken
}
```

The `AlertWouldActivate` status is only assigned when **both** conditions are
met:
1. At least one metric referenced by the alert has a `Dropped` filter outcome.
2. The alert's PromQL expression contains `absent()` or `absent_over_time()`.

If the metric is kept (not dropped), the `absent()` alert remains in its normal
state (`safe`), because the metric's presence prevents `absent()` from firing.

### Status Priority

The status determination follows a strict priority order:

1. `Dropped` + `UsesAbsent` = `would_activate`
2. `Dropped` + no absent = `broken`
3. `Partial` = `at_risk`
4. `Unknown` = `unknown`
5. All `Kept` = `safe`

The `would_activate` and `broken` checks happen first because `Dropped` is the
most severe outcome. This means that if an expression references multiple
metrics where some are dropped and some are partial, the dropped outcome
dominates.

## Edge Cases

| Scenario | Behavior |
|---|---|
| `absent(up{job="api"})` | `usesAbsent=true`, metric `up` extracted. If `up` is dropped, status is `would_activate`. |
| `absent_over_time(up[5m])` | Same treatment as `absent()`. The `_over_time` variant checks for absence over a time range but the semantic inversion is identical. |
| `absent(rate(foo[5m]))` | `usesAbsent=true`. The walker finds both the `FuncExpr` for `absent` and the inner `MetricExpr` for `foo`. |
| `absent(up) or rate(errors[5m]) > 0` | `usesAbsent=true` for the entire expression. Both `up` and `errors` are extracted. If `up` is dropped, the alert is `would_activate` even though `errors` may still be kept. |
| `absent(up)` where `up` is kept | Status is `safe`. The metric is present, so `absent()` does not fire, and no inversion occurs. |
| Expression with no absent | `usesAbsent=false`. Normal status derivation applies. |

## Testing Strategy

Tests span both `extract_test.go` and `coverage_test.go`:

### Detection tests (extract_test.go)

- **`TestExtractMetrics_Absent`** — verifies `usesAbsent=true` for
  `absent(up{job="api"})` and that the inner metric `up` is extracted.
- **`TestExtractMetrics_AbsentOverTime`** — verifies `usesAbsent=true` for
  `absent_over_time(up{job="api"}[5m])`.
- **`TestExtractMetrics_MixedAbsentAndNormal`** — expression with both
  `absent()` and normal selectors. Verifies flag is true and both metrics are
  extracted.
- **`TestExtractMetrics_SimpleReference`** — verifies `usesAbsent=false` for a
  normal expression (negative case).

### Status derivation tests (coverage_test.go)

- **`TestAnalyze_WouldActivate`** — alert with `UsesAbsent=true` and a dropped
  metric produces `AlertWouldActivate`.
- **`TestAnalyze_WouldActivateSummary`** — the summary counter
  `WouldActivate` is incremented correctly.
- **`TestAnalyze_Broken`** — alert without `UsesAbsent` and a dropped metric
  produces `AlertBroken` (confirming the inversion only applies when the flag
  is set).

### Integration via parser (parser_test.go)

- **`TestParseRules_AbsentDetection`** — end-to-end: a YAML rule file with
  `absent(up{job="api"})` is parsed and the resulting `AlertRule` has
  `UsesAbsent=true`.
