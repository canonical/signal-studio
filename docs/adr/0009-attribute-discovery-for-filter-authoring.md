# ADR-0009: Attribute Discovery for Filter Authoring

**Status:** Proposed
**Date:** 2026-02-24
**Related:** [ADR-0006: Metric Name Discovery and Filter Analysis](0006-metric-name-discovery-and-filter-analysis.md), [ADR-0007: Catalog-Based Recommendation Rules](0007-catalog-based-recommendation-rules.md)

---

## Context

ADR-0006 delivered metric name discovery and name-based filter analysis. Users can see which metrics flow through their Collector and predict which ones a `filter` processor would keep or drop. However, this only covers half of what filter processors can do.

The OTel Collector filter processor supports two OTTL contexts:

- **Metric context** (`metrics.metric[]`) — drops entire metrics based on name, type, or resource attributes.
- **Datapoint context** (`metrics.datapoint[]`) — drops individual data points based on their attributes, enabling fine-grained filtering without losing the entire metric.

Common real-world filter expressions operate on attributes, not just names:

```yaml
# Drop all metrics from staging
- 'resource.attributes["deployment.environment"] == "staging"'

# Drop high-cardinality routes from a specific metric
- 'metric.name == "http.server.request.duration" and IsMatch(attributes["http.route"], "^/users/\\d+/.*")'

# Drop metrics carrying debug attributes
- 'HasAttrKeyOnDatapoint("debug.trace_id")'
```

Today, users have no visibility into what attribute keys and values exist in their metric stream. They must guess or consult external systems to author these expressions. The catalog tracks attribute keys (the merged union per metric) but not their values or cardinality.

---

## Problem Statement

To author attribute-based filters, a user needs to answer:

1. **What attributes exist?** — Which keys appear on which metrics, and at which level (resource, scope, datapoint)?
2. **What values do they take?** — What are the actual values for `service.name` or `http.method`?
3. **How many unique values?** — Is `http.route` low-cardinality (10 routes) or high-cardinality (100k user-specific paths)?
4. **What would a filter match?** — If I filter on `attributes["http.method"] == "GET"`, how many data points would that affect?

The catalog currently answers none of these.

---

## Decision

Extend the metric catalog to track attribute metadata at three levels — keys, sample values, and cardinality — with bounded memory usage. Extend the filter analysis engine to support attribute-based OTTL expressions. Surface this data in the catalog UI to guide filter authoring.

### 1. Attribute Metadata in the Catalog

OTLP metrics carry attributes at three levels with very different cardinality profiles:

| Level | Examples | Typical cardinality | Tracking strategy |
|---|---|---|---|
| Resource | `service.name`, `host.name`, `k8s.namespace.name` | Low (tens) | Store all unique values |
| Scope | `otelcol/hostmetricsreceiver` | Very low (single digits) | Store all unique values |
| Datapoint | `http.method`, `http.status_code`, `http.route` | Low to very high | Sample values + cardinality count |

Extend `MetricEntry` with per-attribute metadata:

```go
type AttributeMeta struct {
    Key           string   `json:"key"`
    Level         string   `json:"level"`          // "resource", "scope", "datapoint"
    SampleValues  []string `json:"sampleValues"`   // first N unique values observed
    UniqueCount   int64    `json:"uniqueCount"`     // total unique values seen
    Capped        bool     `json:"capped"`          // true if uniqueCount > sample cap
}
```

**Memory bounds:**

- Cap `SampleValues` at 25 per attribute key per metric. Once reached, stop storing new values but continue incrementing `UniqueCount`.
- Resource and scope attributes are expected to stay well within this cap. Datapoint attributes like `user.id` will hit it quickly, which is itself a useful signal (high cardinality = filter candidate).
- Use a `map[string]struct{}` for deduplication during recording, bounded to the same cap. When the cap is reached, switch to count-only mode (increment counter, skip map insertion).

**Estimated overhead per metric:** ~25 attribute keys x (25 sample values x ~40 bytes + map overhead) ≈ 25 KB worst case. For 200 metrics, that's ~5 MB — acceptable for an in-memory tool.

### 2. Recording Attribute Values from OTLP

The OTLP receiver's `extractAndRecord` function currently extracts only attribute keys from the first data point. Extend it to:

1. Distinguish resource, scope, and datapoint attributes.
2. For each data point, pass attribute key-value pairs to the catalog.
3. The catalog merges values into the bounded `AttributeMeta` structure.

To avoid processing every data point on every export (which could be expensive for high-cardinality metrics), sample at most 10 data points per metric per export batch. This is sufficient to discover attribute values over a few scrape cycles without adding significant CPU cost.

### 3. OTTL Attribute Expression Support

Extend the OTTL parser (`internal/filter/ottl.go`) to recognize these additional expression forms:

| Expression pattern | Match type |
|---|---|
| `resource.attributes["key"] == "value"` | `MatchTypeOTTLResourceAttr` |
| `attributes["key"] == "value"` | `MatchTypeOTTLDatapointAttr` |
| `IsMatch(resource.attributes["key"], "pattern")` | `MatchTypeOTTLResourceAttrMatch` |
| `IsMatch(attributes["key"], "pattern")` | `MatchTypeOTTLDatapointAttrMatch` |
| `HasAttrKeyOnDatapoint("key")` | `MatchTypeOTTLHasAttrKey` |
| `HasAttrOnDatapoint("key", "value")` | `MatchTypeOTTLHasAttr` |

For filter analysis, match these against the catalog's attribute metadata:

- Attribute equality checks: look up the attribute key in the metric's `AttributeMeta`, check if the value exists in `SampleValues`. If the cap was hit and the value isn't in the sample, return `unknown` instead of `kept`.
- `HasAttrKeyOnDatapoint`: check if the key exists in any datapoint-level `AttributeMeta` for the metric.
- Regex matches: test the pattern against all `SampleValues` for the attribute. If capped, mark as `unknown`.

This means filter analysis results for attribute-based expressions will be `kept`, `dropped`, or `unknown` — the same three outcomes as today. The `unknown` outcome is honest about the limits of sampling.

### 4. Frontend: Expandable Catalog Rows

When a user clicks a metric row in the catalog table, expand it to show:

- Attribute keys grouped by level (resource / scope / datapoint).
- For each key: sample values (as chips/tags), unique count, and a "capped" indicator if the sample is incomplete.
- High-cardinality attributes (unique count > sample cap) get a visual callout — these are prime candidates for filtering.

No new panels or pages. The existing table structure gains expandable rows.

### 5. Suggested Filter Expressions

Based on catalog data, generate copy-pasteable OTTL snippets for common scenarios:

- **High-cardinality attribute:** `'HasAttrKeyOnDatapoint("user.id")'` with explanation "This attribute has >25 unique values and may cause cardinality issues."
- **Environment scoping:** If `resource.attributes["deployment.environment"]` has multiple values, suggest per-environment filter expressions.
- **Internal metrics:** If `otelcol_*` metrics are present, suggest name-based filter (already covered by catalog rule, but now surfaced inline).

These are displayed as a helper section in the expanded row, not as recommendations/findings.

---

## Scope

### Included

- Attribute metadata tracking in the catalog (keys, sample values, cardinality counts)
- Resource/scope/datapoint level distinction
- Memory-bounded value sampling (25 values per key per metric)
- OTTL parser extensions for 6 attribute expression forms
- Filter analysis using attribute metadata (with `unknown` for capped attributes)
- Expandable catalog rows showing attribute details
- Inline filter expression suggestions

### Explicitly Out of Scope

- Attribute-based *aggregation* or *transformation* suggestions (e.g., reducing histogram buckets)
- Attribute value correlation across metrics (e.g., "these 5 metrics all have `service.name=foo`")
- Persistent attribute storage or export
- Datapoint context filter analysis (matching at the individual series level vs. whole-metric level)
- `instrumentation_scope.attributes` support (very low usage in practice; can be added later)

---

## Alternatives Considered

### A. Track Only Cardinality Counts (No Sample Values)

Store the number of unique values per attribute key, but not the values themselves.

**Pros:**
- Minimal memory overhead (one counter per key per metric)
- No risk of storing sensitive attribute values

**Cons:**
- Users still can't see *what* the values are, so they can't write filter expressions
- Defeats the primary purpose of helping with filter authoring
- Cardinality count alone is useful for recommendations but not for interactive exploration

**Verdict:** Insufficient. The whole point is showing users what they can filter on.

### B. HyperLogLog for Cardinality Estimation

Use HyperLogLog sketches (~12 KB each) for exact-ish cardinality counting without storing values.

**Pros:**
- Fixed memory per sketch regardless of actual cardinality
- Accurate within ~2% for large cardinalities

**Cons:**
- Still doesn't give sample values (same problem as A)
- 12 KB per attribute key is actually more expensive than 25 capped string samples for low-cardinality attributes
- Added complexity for a marginal accuracy improvement over a simple bounded counter

**Verdict:** Over-engineered for this use case. A bounded set with a counter is simpler and gives us sample values for free.

### C. Full Value Tracking with LRU Eviction

Store all unique values using an LRU cache per attribute key, evicting least-recently-seen values.

**Pros:**
- Always shows the most recent values, adapting to changes over time
- No hard cap on discovery — all values are seen eventually

**Cons:**
- Unbounded memory if the eviction window is too large
- LRU semantics are confusing for users ("why did this value disappear?")
- More complex than a simple append-and-cap approach
- No clear benefit over showing the first 25 values — users need examples, not exhaustive lists

**Verdict:** Unnecessary complexity. First-seen sampling is predictable and sufficient for authoring guidance.

### D. Query a Downstream Backend for Attribute Values

Instead of tracking values in the catalog, query the metrics backend (Prometheus, Mimir, etc.) for label values.

**Pros:**
- Complete and accurate — the backend has all values
- No catalog memory overhead

**Cons:**
- Same attribution problem as ADR-0006 Option B: the backend receives metrics from many sources, so values are polluted with data from other Collectors, SDKs, and agents
- Requires additional connectivity configuration
- Defeats the purpose of the self-contained tap approach

**Verdict:** Rejected for the same reasons as in ADR-0006.

---

## Consequences

**Positive:**
- Users can see exactly what attributes and values exist in their metric stream, directly enabling attribute-based filter authoring.
- Cardinality indicators help users identify problematic attributes before they cause cost or performance issues.
- Filter analysis becomes significantly more useful — moving from name-only matching to attribute-aware predictions.
- The bounded sampling approach keeps memory usage predictable regardless of actual cardinality.

**Negative:**
- Increased CPU cost per OTLP export for attribute extraction (mitigated by sampling at most 10 data points per metric per batch).
- The `unknown` outcome for capped attributes means filter analysis is less precise for high-cardinality attributes — but these are exactly the cases where static analysis has inherent limits.
- API response size increases with attribute metadata; may need pagination or lazy loading for metrics with many attribute keys.

---

## Open Questions

1. **Should attribute values be redacted or filtered?** Some attribute values may contain PII (e.g., user IDs, email addresses). Should we provide an opt-out or redaction mechanism, or is the 25-value sample cap sufficient protection?
2. **Should we track attribute value co-occurrence?** Knowing that `http.method=GET` always appears with `http.status_code=200` could help users write more precise filters, but adds significant complexity.
3. **Should suggested filter expressions be editable inline?** Users could modify a suggestion and immediately see the updated filter analysis, creating a tighter feedback loop.
