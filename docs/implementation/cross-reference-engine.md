# Cross-Reference Engine

**Package:** `github.com/canonical/signal-studio/internal/alertcoverage`
**Source file:** `coverage.go`
**ADR:** [ADR-0018: Alert Rule Coverage Analysis](../adr/0018-alert-rule-coverage-analysis.md)

## Purpose

The cross-reference engine is the core of the alert coverage feature. It takes
parsed alert rules and filter analysis results as inputs and produces a
`CoverageReport` that tells operators which alerts are safe, at risk, broken,
or would activate as a result of the current filter configuration.

## Design

### Entry Point

```go
func Analyze(rules []AlertRule, analyses []filter.FilterAnalysis) *CoverageReport
```

The function performs three steps:
1. Build an outcome map from filter analysis results.
2. Evaluate each alert rule against the outcome map.
3. Compile the results into a report with a summary.

### Step 1: Build the Outcome Map

`buildOutcomeMap` creates a `map[string]filter.MatchOutcome` that maps each
metric name to its worst-case filter outcome across all filter processors in the
pipeline.

A Collector pipeline can contain multiple filter processors. A metric must
survive **all** of them to reach the exporter. The outcome map represents this
sequential composition: for each metric, it stores the composed outcome of all
processors that mention it.

The outcome map is built by iterating through all `FilterAnalysis` results (one
per filter processor) and all `MatchResult` entries within each analysis. When a
metric appears in multiple analyses, the outcomes are composed using
`composeOutcome`.

### Step 2: Outcome Composition

`composeOutcome(a, b)` implements sequential filter composition. It answers the
question: "If processor A produces outcome `a` and processor B produces outcome
`b` for the same metric, what is the combined outcome?"

The composition follows a strict priority order:

| Priority | Outcome | Meaning |
|---|---|---|
| 1 (highest) | `dropped` | Either processor drops the metric. A dropped metric cannot be "un-dropped" by a later processor. |
| 2 | `partial` | Either processor partially filters the metric. Some data points survive but not all. |
| 3 | `unknown` | Either processor cannot determine the outcome. Uncertainty propagates. |
| 4 (lowest) | `kept` | Both processors keep the metric. This is the only "clean" outcome. |

The implementation uses short-circuit checks:

```go
func composeOutcome(a, b filter.MatchOutcome) filter.MatchOutcome {
    if a == filter.OutcomeDropped || b == filter.OutcomeDropped {
        return filter.OutcomeDropped
    }
    if a == filter.OutcomePartial || b == filter.OutcomePartial {
        return filter.OutcomePartial
    }
    if a == filter.OutcomeUnknown || b == filter.OutcomeUnknown {
        return filter.OutcomeUnknown
    }
    return filter.OutcomeKept
}
```

This means `dropped` dominates everything, `partial` dominates `unknown` and
`kept`, and `unknown` dominates `kept`.

### Step 3: Evaluate Each Alert

`evaluateAlert` processes a single `AlertRule`:

1. Iterates through the alert's `MetricNames`.
2. For each metric name, looks up the outcome in the outcome map.
3. Handles three cases:
   - **Regex pattern** (name starts with `~`) — resolved via `resolveRegexOutcome`.
   - **Known metric** (name exists in outcome map) — uses the stored outcome directly.
   - **Unknown metric** (name not in outcome map) — defaults to `OutcomeUnknown`.
4. Collects all per-metric results into `AlertMetricResult` entries.
5. Calls `deriveStatus` to determine the overall alert status.

Only alert rules (`Type == "alert"`) are evaluated. Recording rules are skipped
because they do not directly trigger notifications. (Recording rule dependency
resolution is deferred to Phase 2.)

### Regex Metric Name Resolution

When `ExtractMetrics` encounters a `__name__=~"pattern"` selector, it stores the
pattern with a `~` prefix. The cross-reference engine detects this prefix and
calls `resolveRegexOutcome`:

1. Compile the regex pattern (without the `~` prefix).
2. Iterate through all metric names in the outcome map.
3. For each name that matches the regex, compose its outcome with the running
   result using `composeOutcome`.
4. If no metrics match the regex, return `OutcomeUnknown`.
5. If any matching metric is dropped, the composed result is `dropped`.

This means a regex-based alert is only `safe` if **every** matching metric in
the outcome map is `kept`. If even one matching metric is dropped, the alert is
marked `broken` (or `would_activate` if it uses `absent()`).

If the regex itself fails to compile, the function returns `OutcomeUnknown`.

### Status Determination

`deriveStatus(metrics, usesAbsent)` maps the set of per-metric outcomes to a
single `AlertStatus`:

```
No metrics           → unknown
Any metric dropped   → broken (or would_activate if usesAbsent)
Any metric partial   → at_risk
Any metric unknown   → unknown
All metrics kept     → safe
```

The checks are ordered by severity. `dropped` is checked first because it is
the most impactful outcome. Within the `dropped` case, `usesAbsent` determines
whether the alert is `broken` or `would_activate` (see
[absent-detection.md](absent-detection.md) for the semantic inversion).

### Catalog-Present vs Catalog-Absent Paths

The cross-reference engine does not directly interact with the metric catalog.
Instead, it receives pre-computed `FilterAnalysis` results from the filter
analysis infrastructure (ADR-0006, ADR-0009).

- **Catalog present (tap running):** The filter analysis includes results for
  metrics observed in the OTLP tap. The outcome map is populated with concrete
  outcomes. Alert metrics that match catalog entries get definitive statuses.

- **Catalog absent (no tap):** The filter analysis is based purely on syntactic
  matching of filter expressions against metric names. The outcome map may be
  sparser. Alert metrics not mentioned in any filter expression default to
  `OutcomeUnknown`.

In both cases, the cross-reference engine operates identically. The difference
is in the richness of the input data, not in the engine's logic.

### Summary

`buildCoverageSummary` counts the number of alert results in each status
category and populates the `CoverageSummary` struct with `Total`, `Safe`,
`AtRisk`, `Broken`, `WouldActivate`, and `Unknown` counts.

## Edge Cases

| Scenario | Behavior |
|---|---|
| Alert references no metrics | `deriveStatus` receives empty slice, returns `unknown`. |
| Alert references metric not in any filter analysis | Defaults to `OutcomeUnknown`, status becomes `unknown`. |
| Multiple metrics, one dropped | `dropped` dominates, status is `broken`. |
| Multiple filter processors, metric kept by first but dropped by second | `composeOutcome` returns `dropped`. Alert is `broken`. |
| Regex pattern matches no known metrics | `resolveRegexOutcome` returns `unknown`. |
| Regex pattern matches mix of kept and dropped metrics | `composeOutcome` accumulates to `dropped`. |
| Invalid regex pattern | `regexp.Compile` fails, returns `unknown`. |
| Recording rules in input | Skipped — only `Type == "alert"` rules are evaluated. |
| Empty rules list | Returns empty results with zero-value summary. |
| Empty analyses list | Outcome map is empty. All metric lookups return `unknown`. |

## Testing Strategy

Tests are in `coverage_test.go` and cover all five alert statuses plus
composition and edge cases:

### Status tests
- **`TestAnalyze_Safe`** — metric kept, alert is safe. Summary safe=1.
- **`TestAnalyze_Broken`** — metric dropped, alert is broken. Summary broken=1.
- **`TestAnalyze_AtRisk`** — metric partial, alert is at risk. Summary atRisk=1.
- **`TestAnalyze_WouldActivate`** — metric dropped + `UsesAbsent=true`, alert
  would activate. Summary wouldActivate=1.
- **`TestAnalyze_Unknown`** — metric not in any analysis, alert is unknown.
  Summary unknown=1.

### Composition tests
- **`TestAnalyze_MultipleProcessors`** — metric kept by first processor,
  dropped by second. Result is broken.
- **`TestComposeOutcome_AllCombinations`** — table-driven test covering all
  pairwise combinations: kept+kept, kept+dropped, dropped+kept,
  kept+partial, partial+kept, kept+unknown, unknown+kept,
  partial+unknown, dropped+dropped.

### Multi-metric tests
- **`TestAnalyze_MultipleMetricsInAlert`** — alert references two metrics,
  one kept and one dropped. Result is broken.

### Regex tests
- **`TestAnalyze_RegexMetricName`** — regex `~http_.*` matches a dropped
  metric. Result is broken.
- **`TestAnalyze_RegexNoMatch`** — regex `~custom_.*` matches nothing in the
  outcome map. Result is unknown.

### Structural tests
- **`TestAnalyze_SkipsRecordingRules`** — recording rule in input is excluded
  from results.
- **`TestAnalyze_SummaryTotals`** — three alerts (safe, broken, unknown).
  Summary totals are correct.
- **`TestAnalyze_EmptyRules`** — nil input produces zero-total summary.
- **`TestDeriveStatus_EmptyMetrics`** — empty metrics slice returns unknown.

### Summary counter tests
- **`TestAnalyze_WouldActivateSummary`** — verifies `WouldActivate` counter.
- **`TestAnalyze_AtRiskSummary`** — verifies `AtRisk` counter.
