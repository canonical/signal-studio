# ADR-0018: Alert Rule Coverage Analysis

**Status:** Approved
**Date:** 2026-02-26
**Related:** [ADR-0006: Metric Name Discovery and Filter Analysis](0006-metric-name-discovery-and-filter-analysis.md), [ADR-0009: Attribute Discovery for Filter Authoring](0009-attribute-discovery-for-filter-authoring.md), [ADR-0010: Partial Match Outcome and Filter-Aware Rules](0010-partial-match-outcome-and-filter-aware-rules.md)

---

## Context

Signal Studio can already predict what a filter processor would keep or drop based on observed telemetry (ADR-0006, ADR-0009). The missing piece is answering the question that matters most to operators: **"Will this filter break any of my alerts?"**

Today, teams must mentally cross-reference their filter expressions against their alerting rules. This is error-prone, especially as the number of rules grows. A filter that silently drops a metric used by a critical alert can go undetected until an incident occurs — precisely when the alert was needed.

No existing tool offers this capability. Cloudflare's `pint` validates that metrics referenced in alert rules currently exist in Prometheus, but it does not predict the effect of a proposed filter change. `mimirtool analyze` extracts metric names from rules but does not check them against any filter configuration. Signal Studio is uniquely positioned to do this because it already understands both the filter configuration and the metric catalog.

---

## Problem Breakdown

The feature has three subproblems:

1. **Alert rule parsing** — accept alerting (and optionally recording) rule files and extract the metric names referenced in each rule's PromQL expression.
2. **Cross-referencing** — match extracted metric names against the existing filter analysis results to determine which alerts are affected by which filters.
3. **Presentation** — surface the results in a way that makes it immediately clear which alerts are at risk.

---

## Alert Rule Format Landscape

### Format compatibility

| Source | Rule format | PromQL location | Parsing complexity |
|---|---|---|---|
| Prometheus file | Standard YAML | `groups[].rules[].expr` | Trivial |
| Mimir / Cortex API | Standard YAML | `groups[].rules[].expr` | Trivial |
| Thanos file | Standard YAML | `groups[].rules[].expr` | Trivial |
| Kubernetes `PrometheusRule` CRD | Standard YAML in `spec.groups` | `spec.groups[].rules[].expr` | Trivial (unwrap CRD) |
| Grafana unified alerting | Custom YAML | `data[].model.expr` | Moderate (multi-step pipeline) |

Prometheus, Mimir, Thanos, and the Kubernetes CRD all use the **identical YAML format**. A single parser covering the standard Prometheus rule file structure handles 90%+ of use cases. Grafana unified alerting is structurally different but the PromQL expression is still extractable from `data[].model.expr` when the datasource is Prometheus-compatible.

### Recording rules

Recording rules (`record` instead of `alert`) create derived metric names that form dependency chains:

```
http_requests_total (raw OTLP)
  → job:http_requests:rate5m (recording rule 1)
    → job:http_error_ratio (recording rule 2)
      → HighErrorRate alert
```

If the Collector filter drops `http_requests_total`, the entire chain breaks. Handling this requires parsing recording rules alongside alerting rules and building a dependency graph.

---

## PromQL Metric Name Extraction

### Library Options

#### A. `github.com/prometheus/prometheus/promql/parser` (Official)

The canonical PromQL parser. Provides `ParseExpr()` → AST, `ExtractSelectors()`, and `Inspect()` for traversal. `VectorSelector` nodes carry the metric name and label matchers.

**Pros:** Authoritative, covers the full PromQL grammar.
**Cons:** Pulls in the full `prometheus/prometheus` module. Known to import millions of lines of transitive dependencies. Would significantly increase Signal Studio's binary size beyond the current 21 MB.

#### B. `github.com/VictoriaMetrics/metricsql` (Lightweight)

Standalone PromQL/MetricsQL parser with minimal dependencies. Provides `Parse()` → AST and `VisitAll()` for traversal. `MetricExpr` nodes contain `LabelFilterss` with `__name__` filters.

**Pros:** Lightweight, minimal dependency footprint. Backwards-compatible with PromQL. Provides exactly what we need (AST traversal + metric name extraction) without the overhead.
**Cons:** MetricsQL extends PromQL with non-standard syntax — though this is harmless since all valid PromQL parses correctly. Not the "official" parser.

#### Decision: Option B (`metricsql`)

Binary size matters. The gRPC/pdata dependencies for the OTLP tap already added 8.4 MB. Adding the full Prometheus module for PromQL parsing is not justified when a lightweight alternative exists that covers the same use case. This is a pragmatic choice — `metricsql` is well-maintained, backwards-compatible with PromQL, and does not introduce any runtime coupling to VictoriaMetrics. If a lightweight, standalone PromQL parser emerges from the official Prometheus project or elsewhere, it would be worth re-evaluating.

### Edge Cases

| Case | Handling |
|---|---|
| Simple reference: `http_requests_total` | Direct extraction |
| With labels: `http_requests_total{job="api"}` | Extract metric name, note label matchers |
| Wrapped in function: `rate(http_requests_total[5m])` | Walk AST into function arguments |
| Binary ops: `metric_a / metric_b > 0.5` | Extract both sides |
| Aggregation: `sum(rate(foo[5m])) by (job)` | Walk into aggregate expression |
| `__name__` regex: `{__name__=~"http_.*"}` | Test regex against catalog, flag as pattern-based |
| `absent(up{job="api"})` | **Special case** — see below |
| Subqueries: `rate(foo[5m])[30m:1m]` | Walk into subquery inner expression |
| Recording rule output: `job:http_latency:p99` | Resolve via dependency graph if recording rules provided |

### The `absent()` Special Case

`absent()` and `absent_over_time()` fire when a metric is **missing**. If a filter drops the metric, these alerts would **start firing** rather than break — the semantically opposite outcome from normal alerts.

Signal Studio must detect `absent()` calls in the AST and report a distinct outcome: "This alert uses `absent()` — filtering this metric would **activate** the alert, not break it."

---

## Alternatives for Integration Pattern

### A. File Upload / Paste (MVP)

Same interaction as the existing Collector YAML input. The user pastes or uploads a Prometheus-format rule file. Signal Studio parses it and performs the analysis.

**Pros:** Works offline, no connectivity requirements, consistent with existing UX, handles all standard formats.
**Cons:** Requires manual copy-paste, no auto-discovery.

### B. Scrape Prometheus `/api/v1/rules` Endpoint

User provides the Prometheus server URL. Signal Studio calls `GET /api/v1/rules` and receives all configured rules.

**Pros:** Automatic, no copy-paste. Gets both alerting and recording rules.
**Cons:** Requires network access from Signal Studio to Prometheus. Authentication may be required. Mimir needs `X-Scope-OrgID` header.

### C. Scrape Grafana Alerting API

`GET /api/v1/provisioning/alert-rules` returns Grafana-managed rules.

**Pros:** Covers Grafana unified alerting format.
**Cons:** Different response format, requires API key, higher implementation complexity.

### D. Kubernetes CRD Files

User provides `PrometheusRule` YAML files. Signal Studio strips the Kubernetes wrapper.

**Pros:** Common in Kubernetes-native environments.
**Cons:** Just a variant of Option A — unwrap `spec.groups` and treat as standard rules.

### Recommendation

**Phase 1: Options A + B + D.** File upload/paste, Prometheus API scraping, and CRD auto-detection. Option B (Prometheus API) belongs in Phase 1 because users who already have a Prometheus/Mimir instance — the most likely initial audience — will expect to point Signal Studio at it rather than copy-paste rule files. Note that the rules endpoint (Prometheus/Mimir server) is a separate connection from the Collector metrics endpoint — these are different hosts. The UI needs a distinct connection input for the rules source (URL + optional bearer token + optional `X-Scope-OrgID` for Mimir tenants). Format detection for paste/upload: presence of `apiVersion` + `kind: PrometheusRule` = CRD, otherwise standard Prometheus format.

**Phase 2: Option C (Grafana API).** Only if there's demand. Grafana's multi-step evaluation pipeline adds parsing complexity, and the API requires authentication setup.

---

## Design

### Data Flow

```
Alert rule YAML → parse rules → extract metric names per rule
                                        ↓
Collector YAML → parse config → filter analysis → match results per metric
                                        ↓
                               Cross-reference: for each alert, look up
                               the filter outcome of each referenced metric
                                        ↓
                               Alert coverage report
```

### New Types

```go
// AlertRule represents a parsed alerting or recording rule.
type AlertRule struct {
    Name        string   // alert name or recording rule name
    Type        string   // "alert" or "record"
    Expr        string   // raw PromQL expression
    MetricNames []string // extracted metric names from expr
    Group       string   // rule group name
    UsesAbsent  bool     // true if expr contains absent() or absent_over_time()
}

// AlertCoverageResult represents the filter impact on a single alert.
type AlertCoverageResult struct {
    AlertName    string
    AlertGroup   string
    Expr         string
    Metrics      []AlertMetricResult
    Status       AlertStatus // safe, at_risk, broken, would_activate, unknown
}

type AlertMetricResult struct {
    MetricName    string
    FilterOutcome filter.MatchOutcome // kept, dropped, partial, unknown
    // ViaRecording is populated in Phase 2 when recording rule dependency
    // graph resolution is implemented. Empty string in MVP.
    ViaRecording  string
}

type AlertStatus string
const (
    AlertSafe          AlertStatus = "safe"           // all metrics kept
    AlertAtRisk        AlertStatus = "at_risk"        // some metrics partially filtered
    AlertBroken        AlertStatus = "broken"         // a required metric is fully dropped
    AlertWouldActivate AlertStatus = "would_activate" // absent() alert, metric would be dropped
    AlertUnknown       AlertStatus = "unknown"        // metric not in catalog
)
```

### Recording Rule Dependency Graph (Deferred — Phase 2)

Recording rules create derived metric names that form dependency chains. Resolving these chains requires building a DAG from recording rule `record` names to their `expr` source metrics, handling multi-level resolution, and detecting circular dependencies. This is non-trivial graph traversal code with a high complexity-to-usage ratio — most alert rules reference raw metrics directly, and recording rule chains are an advanced pattern used by a small subset of the target audience.

**MVP behavior:** If an alert references a metric name that is not found in the catalog or filter expressions, it is reported as `unknown`. This is honest and consistent with Signal Studio's approach elsewhere (ADR-0016). The `unknown` status clearly communicates "we don't have enough information" rather than guessing.

**Phase 2:** When users encounter `unknown` results caused by recording rule indirection and request resolution, add the dependency graph:

1. Parse all rules (alerting + recording).
2. For each recording rule, map its `record` name → set of source metric names from `expr`.
3. For each metric name referenced in an alert:
   a. If it exists in the tap catalog → use filter analysis result directly.
   b. If it matches a recording rule output → follow the chain to source metrics, use their filter outcomes.
   c. If neither → mark as `unknown`.

The graph is typically shallow (1-2 levels of recording rules). Circular dependencies in recording rules are a configuration error and should be flagged rather than followed.

### API

New endpoint:

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/alert-coverage` | Analyze alert rule coverage against current filter analysis |

Request body:

```json
{
  "rules": "... YAML string ...",
  "format": "prometheus"
}
```

Format auto-detection: if the YAML contains `apiVersion` and `kind: PrometheusRule`, treat as CRD. Otherwise, treat as standard Prometheus rules.

Response:

```json
{
  "results": [
    {
      "alertName": "HighErrorRate",
      "alertGroup": "service_alerts",
      "expr": "rate(http_requests_total{status=\"500\"}[5m]) > 0.05",
      "status": "broken",
      "metrics": [
        {
          "metricName": "http_requests_total",
          "filterOutcome": "dropped",
          "viaRecording": ""
        }
      ]
    }
  ],
  "summary": {
    "total": 15,
    "safe": 10,
    "atRisk": 2,
    "broken": 1,
    "wouldActivate": 0,
    "unknown": 2
  }
}
```

### Prerequisites

This feature depends on:
- **Filter analysis** (ADR-0006, ADR-0009) — already implemented. The `AnalyzeFilter` function produces per-metric `MatchResult` outcomes.
- **Metric catalog** (ADR-0006) — for the catalog-aware path. Without the tap running, the analysis falls back to name-only matching against filter expressions.

The feature **does not require** the tap to be running. Even without catalog data, filter expressions contain enough information to determine whether a metric name would be matched by an include/exclude pattern or OTTL expression. The catalog adds volume weighting and attribute-level analysis but is not strictly required for name-level coverage checking.

### Frontend

Add an "Alert Rules" section in the UI — either a tab in the existing config panel or a secondary text area. When rules are pasted and analyzed:

- Each alert gets a status badge: safe (green), at risk (yellow), broken (red), would activate (blue), unknown (gray).
- Expand an alert to see which metrics are referenced and their individual filter outcomes.
- Summary bar shows aggregate counts.
- Broken alerts are sorted to the top.

---

## Confidence and Accuracy

### What we can confidently determine

- Whether a metric name matches a filter's include/exclude patterns or OTTL `name ==` / `IsMatch(name, ...)` expressions
- Whether a metric is fully dropped, fully kept, or partially matched
- Whether the alert uses `absent()` semantics
- Recording rule dependency chains (when recording rules are provided)

### What we cannot determine

- **Metrics from outside this Collector** — alerts may reference metrics from other sources. These appear as `unknown`, not `broken`.
- **Label-level filter interactions** — if a filter drops a specific label subset (e.g., `route="/api/v1/users"`) and an alert queries that exact subset, we can flag this as `at_risk` with a partial match, but cannot be certain without full attribute intersection.
- **Recording rule outputs we don't have** — if an alert references a recording rule that wasn't included in the provided rules, we can only report `unknown`.
- **Grafana expression pipeline semantics** — Grafana's multi-step reduce/threshold pipeline adds complexity beyond PromQL. Defer to Phase 3.

### Honesty policy

Consistent with Signal Studio's approach elsewhere (ADR-0009, ADR-0016), the tool reports `unknown` rather than guessing. An `unknown` result means "we don't have enough information to determine the impact" — not "it's probably fine."

---

## Implementation Requirements

### Non-Trivial Components

The following components involve significant logic and must each be documented in a dedicated markdown file under `docs/implementation/`. The implementation is not considered complete until all documentation files exist and accurately describe the component's design, behavior, edge cases, and testing strategy.

| Component | Package | Documentation file | Description |
|---|---|---|---|
| PromQL metric name extraction | `internal/alertcoverage` | `docs/implementation/promql-metric-extraction.md` | AST traversal using `metricsql.Parse()` and `VisitAll()`. Must document handling of every edge case in the Edge Cases table: simple references, label matchers, function wrapping, binary ops, aggregations, `__name__` regex, subqueries. Must explain how `MetricExpr` nodes are walked and how `__name__` filters are extracted from `LabelFilterss`. |
| `absent()` / `absent_over_time()` detection | `internal/alertcoverage` | `docs/implementation/absent-detection.md` | How the AST walker identifies `absent()` and `absent_over_time()` function calls and propagates the `UsesAbsent` flag. Must document the semantic inversion (filtering activates rather than breaks the alert) and how `AlertWouldActivate` status is derived. |
| Cross-reference engine | `internal/alertcoverage` | `docs/implementation/cross-reference-engine.md` | How alert metric names are matched against filter analysis results. Must document: the composition of multiple filter processor outcomes, the status determination algorithm (safe/at_risk/broken/would_activate/unknown), and the catalog-present vs. catalog-absent code paths. |
| Alert rule YAML parser | `internal/alertcoverage` | `docs/implementation/alert-rule-parser.md` | Parsing of Prometheus-format rule files and Kubernetes `PrometheusRule` CRDs. Must document format auto-detection logic (`apiVersion` + `kind` check), CRD unwrapping, and graceful handling of malformed rules. |
| Prometheus/Mimir rules API client | `internal/alertcoverage` | `docs/implementation/rules-api-client.md` | HTTP client for `GET /api/v1/rules`. Must document authentication (bearer token), Mimir multi-tenancy (`X-Scope-OrgID` header), response parsing, error handling, and merging of API-sourced rules with file-based rules. |

**Deferred to Phase 2:**

| Component | Package | Documentation file | Description |
|---|---|---|---|
| Recording rule dependency graph | `internal/alertcoverage` | `docs/implementation/recording-rule-graph.md` | Graph construction from recording rule `record` → `expr` metric names. Chain resolution (multi-level recording rules), circular dependency detection and error reporting, and integration with catalog lookups. Deferred because most alert rules reference raw metrics directly — recording rule chains are an advanced pattern with a high complexity-to-usage ratio. |

### Test Coverage Requirements

All non-trivial components listed above must have test coverage of **90% or higher**. Specifically:

- **PromQL metric name extraction** — test every edge case in the Edge Cases table. Include tests for deeply nested expressions (e.g., `sum(rate(histogram_quantile(0.99, foo)[5m])) by (job)`), expressions with multiple metrics, and expressions with no metric selectors.
- **`absent()` detection** — test `absent()`, `absent_over_time()`, nested `absent(rate(foo[5m]))`, and expressions mixing `absent()` with normal selectors. Verify that `UsesAbsent` is set correctly and that `AlertWouldActivate` status is assigned.
- **Cross-reference engine** — test all five `AlertStatus` outcomes with controlled filter analysis inputs. Test composition of multiple filter processors (metric survives first but dropped by second = `broken`). Test the catalog-absent path separately.
- **Alert rule YAML parser** — test standard Prometheus format, CRD format, mixed alerting + recording rules, malformed YAML (graceful degradation), empty rule groups, and rules with missing `expr` fields.
- **Rules API client** — test with mock HTTP server: successful response, authentication headers, Mimir `X-Scope-OrgID`, HTTP errors, malformed JSON responses, and merging with file-based rules.

**Phase 2 (recording rule dependency graph):**
- Test single-level chains, multi-level chains (3+ deep), circular dependencies (must error, not infinite loop), recording rules whose source metrics are not in the catalog, and mixed alerting + recording rule files.

---

## Open Questions

1. ~~**Should this work without the tap?**~~ — **Resolved: yes.** Name-only matching against filter expressions covers the most common case without requiring catalog data. Volume weighting and attribute-level analysis are bonuses when the tap is running.

2. ~~**How to handle alerts referencing metrics from other Collectors?**~~ — **Resolved.** When the tap is running, report unobserved metrics as `unknown` with context: "This metric was not observed in the tap catalog. It may originate from another source." Do not flag as `broken`. When the tap is not running, there is no catalog to compare against — analysis is purely syntactic (pattern matching against filter expressions), so this distinction does not apply.

3. ~~**Should we validate PromQL syntax?**~~ — **Resolved: graceful degradation.** If `metricsql.Parse()` fails on an expression, report a parse error for that rule but continue analyzing the rest. One malformed expression should not block the entire analysis.

4. ~~**Multiple filter processors in a pipeline**~~ — **Resolved: compose results.** A metric must survive all filter processors in the pipeline to be considered `kept`. The existing filter analysis already returns per-processor results; this feature composes them sequentially.

5. ~~**CLI integration (ADR-0017)?**~~ — **Resolved: yes, file and API.** `signal-studio analyze config.yaml --alerts rules.yaml` accepts alert rules as a file path. `--rules-url` fetches rules from a Prometheus/Mimir API endpoint (with optional `--rules-token` and `--rules-org-id`). Both can be combined — file-based and API-sourced rules are merged. No stdin or paste support for alerts in CLI mode — the primary input remains the Collector YAML.

---

## Impact Assessment

### User Impact: Very High

This answers the #1 fear operators have when adding filters: "will this break something?" Reducing that fear directly translates to more aggressive (and warranted) telemetry reduction. Teams that currently avoid filtering because the risk is unknown would be able to filter confidently.

### Differentiation: Very High

**No existing tool offers this capability.** The closest analog is Cloudflare's `pint`, which validates metrics against a live Prometheus — but it checks the current state, not the predicted state after a filter change. Signal Studio would be the only tool that predicts downstream alerting impact of Collector filter changes before deployment.

This is a genuine competitive moat: it requires understanding both the Collector's filter configuration (from the YAML parser) AND the metric catalog (from the OTLP tap), plus a PromQL parser to extract metric references from alert rules. The combination of these three capabilities is unique to Signal Studio.

### Effort: Medium

- PromQL parser integration (`metricsql`) — ~0.5 day
- Alert rule YAML parser (Prometheus format + CRD detection) — ~1 day
- Metric name extraction from PromQL AST — ~1 day
- Cross-reference engine (connect alert metrics to filter outcomes) — ~1 day
- `absent()` detection and special handling — ~0.5 day
- API endpoint — ~0.5 day
- Prometheus/Mimir rules API client — ~0.5 day
- Frontend (alert rules input + coverage results panel) — ~2 days
- Tests — ~1.5 days

Estimated total: **8-9 days**.

Recording rule dependency graph (~1 day + tests) is deferred to Phase 2.

### Dependencies

- `github.com/VictoriaMetrics/metricsql` — new dependency, lightweight (~few hundred KB impact on binary)
- Existing filter analysis infrastructure (ADR-0006, ADR-0009) — already implemented
- Does NOT depend on CLI mode (ADR-0017) — works in both server and CLI modes

---

## Consequences

### Positive

- Directly addresses the #1 operator fear around filtering: "will this break my alerts?"
- Unique differentiator — no other tool in the market offers this
- Builds on existing infrastructure (filter analysis, metric catalog) with relatively modest new code
- `metricsql` dependency is lightweight and well-maintained
- Feature works with or without the OTLP tap — graceful degradation
- Naturally extends to CLI mode for CI/CD alert coverage checks on PR

### Negative

- Adds a PromQL parser dependency — albeit a lightweight one
- Alert rule format detection adds complexity (Prometheus vs. CRD vs. Grafana)
- `absent()` semantics require special-cased logic
- Recording rule dependency resolution is deferred — alerts referencing recording rule outputs will report `unknown` until Phase 2
- Cross-referencing alert label matchers against filter attribute conditions is complex and deferred — the MVP operates at metric name granularity only
- Users may over-trust the results — must clearly communicate the `unknown` status for metrics outside the observed Collector
- Grafana unified alerting format support is deferred, which may disappoint users in Grafana-heavy environments
