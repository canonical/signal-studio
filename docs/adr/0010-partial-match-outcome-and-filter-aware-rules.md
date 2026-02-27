# ADR-0010: Partial Match Outcome and Filter-Aware Catalog Rules

**Status:** Proposed
**Date:** 2026-02-25
**Related:** [ADR-0009: Attribute Discovery for Filter Authoring](0009-attribute-discovery-for-filter-authoring.md), [ADR-0007: Catalog-Based Recommendation Rules](0007-catalog-based-recommendation-rules.md)

---

## Context

ADR-0009 introduced attribute-aware filter analysis, enabling the matcher to determine whether attribute-based filter expressions would match a given metric. However, the current implementation has a binary view: if any sample value matches a datapoint-level attribute filter, the entire metric is marked as `dropped`. This creates two problems:

1. **Volume projection overestimates.** A filter expression like `attributes["device"] == "loop0"` on `system.disk.io` causes the frontend to remove the entire metric's point count from projections, even though only a fraction of data points (those with `device=loop0`) would actually be dropped. For a metric with 10 device values where only 2 match, the real volume reduction is ~20%, not 100%.

2. **Catalog rules ignore existing filters.** The `LoopDeviceMetrics` rule fires whenever loop devices are detected in `system.disk.*` attributes, regardless of whether the user has already configured a filter processor to drop them. Other catalog rules (e.g. `InternalMetricsNotFiltered`) already check the `analyses` parameter to suppress when addressed — `LoopDeviceMetrics` should follow the same pattern.

---

## Decision

### Part A: Partial Match Outcome

Introduce `OutcomePartial` as a fourth `MatchOutcome` value. When a datapoint-level attribute filter matches some but not all observed sample values, the result is `partial` with a `droppedRatio` between 0.0 and 1.0.

**Semantic rules:**

| Condition | Outcome |
|---|---|
| All sample values match, not capped | `dropped` |
| Some match, some don't | `partial` (ratio = matched/total) |
| Some match, capped | `partial` (ratio = matched/total, estimate) |
| None match, capped | `unknown` |
| None match, not capped | `kept` |
| Resource-level: any match | `dropped` (unchanged) |
| `HasAttrKey`: key exists | `dropped` (unchanged) |

Resource-level attributes are binary because a matching resource attribute causes the entire metric (all data points) to be dropped. Datapoint-level attributes filter individual data points, so partial matches are meaningful.

The `droppedRatio` is an estimate based on the sample values observed. When the attribute is capped (more unique values exist than were sampled), the ratio represents a lower bound on the matched fraction.

### Part B: Filter-Aware LoopDeviceMetrics Rule

Update the `LoopDeviceMetrics` catalog rule to check the `analyses` parameter for any filter that already addresses loop device data points. If any `FilterAnalysis` contains results where `system.disk.*` metrics have loop-device-related outcomes of `dropped` or `partial`, suppress the rule.

This follows the same pattern already established by `InternalMetricsNotFiltered`.

---

## Changes

### Backend

- `internal/filter/matcher.go`: Add `OutcomePartial`, `DroppedRatio` field on `MatchResult`, `PartialCount` on `FilterAnalysis`. Refactor `matchAttrEquality` and `matchAttrRegex` to return match counts, producing partial outcomes for datapoint-level matchers.
- `internal/rules/catalog_rules.go`: Update `LoopDeviceMetrics.EvaluateWithCatalog` to check analyses before firing.

### Frontend

- `types/api.ts`: Add `"partial"` to `MatchOutcome`, `droppedRatio` to `MatchResult`, `partialCount` to `FilterAnalysis`.
- `PipelineGraph.tsx`: Update `filterVolumeChange` to proportionally reduce points for partial matches.
- `MetricCatalogPanel.tsx`: Add partial icon, update summary and sort order.

---

## Consequences

**Positive:**
- Volume projections become proportional rather than binary, giving users a realistic estimate of filter impact.
- The LoopDeviceMetrics rule no longer fires spuriously when the user has already configured appropriate filters.
- The `partial` outcome is a more honest representation of what datapoint-level filters actually do.

**Negative:**
- The `droppedRatio` is an estimate based on sampled values; the actual ratio depends on the real distribution of attribute values across data points, which may differ from the uniform distribution assumed.
- Adds a fourth outcome state to the UI, increasing visual complexity slightly.
