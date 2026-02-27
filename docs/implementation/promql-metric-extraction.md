# PromQL Metric Name Extraction

**Package:** `github.com/canonical/signal-studio/internal/alertcoverage`
**Source file:** `extract.go`
**ADR:** [ADR-0018: Alert Rule Coverage Analysis](../adr/0018-alert-rule-coverage-analysis.md)

## Purpose

`ExtractMetrics` parses a PromQL expression string and returns the metric names
referenced in it. It also detects whether the expression uses `absent()` or
`absent_over_time()`, which inverts the semantic meaning of a dropped metric
(see [absent-detection.md](absent-detection.md)).

## Library Choice

The implementation uses `github.com/VictoriaMetrics/metricsql` rather than the
official `github.com/prometheus/prometheus/promql/parser`. The metricsql library
is a standalone PromQL/MetricsQL parser with minimal dependencies. It is
backwards-compatible with PromQL: all valid PromQL parses correctly, though
metricsql also accepts MetricsQL extensions. This trade-off was accepted because
the official Prometheus parser pulls in the entire `prometheus/prometheus` module
and its transitive dependencies, which would significantly increase binary size.
See ADR-0018 for the full rationale.

## Design

### Entry Point

```go
func ExtractMetrics(expr string) ([]string, bool, error)
```

Returns:
1. A deduplicated slice of metric names found in the expression.
2. A boolean indicating whether the expression uses `absent()` or
   `absent_over_time()`.
3. An error if the expression fails to parse.

### State Management

The function uses two pieces of mutable state, both captured by the visitor
closure:

- `seen map[string]struct{}` — tracks which metric names have already been
  added to the output slice, preventing duplicates. This is necessary because
  the same metric can appear multiple times in a single expression (e.g.,
  `rate(foo[5m]) / rate(foo[1m])`).

- `usesAbsent bool` — set to `true` if any `absent()` or `absent_over_time()`
  function call is encountered during traversal. Once set, it is never unset.

### AST Traversal

The function follows a two-step process:

1. **Parse** — `metricsql.Parse(expr)` converts the PromQL string into an AST
   (abstract syntax tree). If parsing fails, the error is returned immediately.
   This provides graceful degradation: a single malformed expression does not
   block analysis of other rules.

2. **Walk** — `metricsql.VisitAll(e, visitor)` performs a depth-first traversal
   of every node in the AST. The visitor callback receives each `metricsql.Expr`
   node and uses a type switch to inspect two node types:

   - `*metricsql.FuncExpr` — function call nodes. The walker checks if the
     function name (case-insensitively) is `absent` or `absent_over_time` and
     sets the `usesAbsent` flag.

   - `*metricsql.MetricExpr` — metric selector nodes. These represent vector
     selectors like `http_requests_total{job="api"}`. The walker inspects the
     `LabelFilterss` field (a slice of slices of `LabelFilter`) to find
     `__name__` label filters.

### MetricExpr and LabelFilterss

In metricsql's AST, a `MetricExpr` node represents a metric selector. The
metric name is not stored as a dedicated field; instead it appears as a
`__name__` label filter within `LabelFilterss`. This is consistent with
Prometheus's internal representation where `http_requests_total` is equivalent
to `{__name__="http_requests_total"}`.

`LabelFilterss` is a `[][]LabelFilter` — a slice of OR-joined groups of
AND-joined label filters. The walker iterates through all groups and all filters
within each group, looking for `LabelFilter` entries where `Label == "__name__"`.

### __name__ Filter Extraction

For each `__name__` label filter, the walker applies the following logic:

| Filter property | Handling |
|---|---|
| `IsNegative == true` | Skipped. Negative matchers (`__name__!="..."` or `__name__!~"..."`) do not identify a specific metric name. |
| `IsRegexp == true` | Stored with a `~` prefix (e.g., `~http_.*`). This convention allows downstream consumers (the cross-reference engine) to distinguish regex patterns from literal names without additional metadata. |
| Otherwise (equality match) | Stored as-is (e.g., `http_requests_total`). |

A `seen` map ensures deduplication. If the same metric name (or regex pattern)
appears multiple times in the expression, it is only included once in the output
slice.

### Why VisitAll Works for All PromQL Structures

`metricsql.VisitAll` is a generic depth-first walker that visits every node in
the AST regardless of its type. This means the implementation does not need
explicit handling for each PromQL construct (functions, binary operators,
aggregations, subqueries). The walker traverses all of them automatically and
calls the visitor for every node it encounters.

The key insight is that **metric names always appear as `MetricExpr` leaf
nodes** in the AST. No matter how deeply nested a metric selector is — inside a
function call, inside an aggregation, inside a binary operation, inside a
subquery — `VisitAll` will eventually reach it. This makes the implementation
both simple and robust: new PromQL constructs added to metricsql in the future
will be traversed automatically without code changes.

The only two node types the visitor inspects are `FuncExpr` (for absent
detection) and `MetricExpr` (for name extraction). All other node types are
visited but ignored.

## Edge Cases

The following table documents how each PromQL pattern is handled. The AST
traversal with `VisitAll` naturally handles all of these because it walks the
entire tree depth-first, reaching `MetricExpr` leaf nodes regardless of how
deeply they are nested.

| Case | Example | How it works |
|---|---|---|
| Simple reference | `http_requests_total` | Single `MetricExpr` with `__name__="http_requests_total"`. Extracted directly. |
| With label matchers | `http_requests_total{job="api"}` | `MetricExpr` has multiple `LabelFilter` entries. Only the `__name__` filter is extracted; other labels are ignored. |
| Wrapped in function | `rate(http_requests_total[5m])` | `VisitAll` descends into `FuncExpr` arguments and finds the inner `MetricExpr`. |
| Binary operations | `metric_a / metric_b > 0.5` | `VisitAll` visits both sides of `BinaryOpExpr` nodes, finding two `MetricExpr` nodes. |
| Aggregations | `sum(rate(foo[5m])) by (job)` | `VisitAll` descends through `AggrFuncExpr` into `FuncExpr` into `MetricExpr`. |
| `__name__` regex | `{__name__=~"http_.*"}` | `MetricExpr` with `IsRegexp=true`. Stored as `~http_.*`. |
| Subqueries | `rate(foo[5m])[30m:1m]` | Subquery wraps the inner expression. `VisitAll` reaches the `MetricExpr` inside. |
| Deeply nested | `histogram_quantile(0.99, sum(rate(x[5m])) by (le))` | Multiple nesting levels. `VisitAll` traverses all of them. |
| Duplicate references | `rate(foo[5m]) / rate(foo[1m])` | The `seen` map deduplicates. `foo` appears once in output. |
| Scalar literals | `42` | No `MetricExpr` nodes exist. Returns empty slice. |
| Invalid PromQL | `not valid !!!` | `metricsql.Parse()` returns an error. The function returns the error without visiting. |
| Negative name matcher | `{__name__!="unwanted"}` | `IsNegative == true` — skipped. Does not appear in output. |

## Testing Strategy

Tests are in `extract_test.go` and cover every edge case from the table above:

- **Simple reference** — extracts a single metric name.
- **With labels** — extracts the metric name, ignores other label matchers.
- **Rate function** — walks into function arguments.
- **Binary ops** — extracts both sides (`metric_a`, `metric_b`).
- **Aggregation** — walks through `sum(rate(...))`.
- **Nested functions** — deeply nested `histogram_quantile(0.99, sum(rate(...)))`.
- **`__name__` regex** — returns `~http_.*` with the prefix convention.
- **`absent()`** — sets `usesAbsent=true`, extracts inner metric.
- **`absent_over_time()`** — sets `usesAbsent=true`.
- **Mixed absent and normal** — both flag and all metric names are correct.
- **Subquery** — extracts metric from inside subquery syntax.
- **Duplicate metrics** — same metric referenced twice, deduplicated to one.
- **Invalid expression** — returns a parse error.
- **No selectors** — scalar literal returns empty names slice.

The `assertNames` helper sorts both expected and actual slices before comparison,
making tests order-independent.
