# ADR-0007: Catalog-Based Recommendation Rules

**Status:** Accepted
**Date:** 2026-02-24
**Related:** [ADR-0006: Metric Name Discovery and Filter Analysis](0006-metric-name-discovery-and-filter-analysis.md)

---

## Context

ADR-0006 Phase 1 delivered the OTLP sampling tap (metric catalog) and filter analysis engine. The catalog collects metric names, types, attribute keys, and point counts from live OTLP data. Filter analysis predicts which metrics a `filter` processor would keep or drop.

With this data available, we can now surface actionable, data-driven recommendations — moving beyond generic config-only rules to findings backed by observed telemetry.

---

## Decision

Introduce a `CatalogRule` interface that extends the existing `Rule` interface with access to catalog entries and filter analyses. Implement 8 catalog-based recommendation rules.

### CatalogRule Interface

```go
type CatalogRule interface {
    Rule
    EvaluateWithCatalog(
        cfg *config.CollectorConfig,
        entries []tap.MetricEntry,
        analyses []filter.FilterAnalysis,
    ) []Finding
}
```

This follows the same extension pattern as `LiveRule` (ADR-0005). The engine dispatches to `EvaluateWithCatalog` for rules that implement the interface, falling back to `Evaluate` for plain rules.

### Rules

| # | ID | Severity | Trigger |
|---|---|---|---|
| 1 | `catalog-internal-metrics-not-filtered` | warning | `otelcol_*` metrics present and not dropped by any filter |
| 2 | `catalog-high-attribute-count` | warning | Metric with >10 attribute keys |
| 3 | `catalog-point-count-outlier` | warning | Point count >10x mean AND >1000 |
| 4 | `catalog-filter-keeps-everything` | info | Filter with 0 drops, 0 unknowns, >0 kept |
| 5 | `catalog-filter-drops-everything` | critical | Filter with 0 kept, >0 drops |
| 6 | `catalog-no-filter-high-volume` | info | >50 metrics, no filter processor |
| 7 | `catalog-many-histograms` | info | >5 histograms AND >30% of total |
| 8 | `catalog-short-scrape-interval` | info | Prometheus or hostmetrics receiver with <60s interval |

### Handler Wiring

Filter analyses are computed before catalog rule evaluation so they're available as input. Catalog findings are merged with existing static and live findings.

---

## Consequences

- **Positive:** Recommendations are now data-driven and specific to the user's actual metric stream.
- **Positive:** The CatalogRule interface is a clean extension of the existing rule pattern.
- **Positive:** All 8 rules have comprehensive tests; coverage stays above 80%.
- **Negative:** Rules 1-7 only fire when the tap is active and has collected data.
- **Negative:** Rule 8 fires based on config alone but is gated behind the CatalogRule interface for grouping consistency.

---

## Alternatives Considered

1. **Add catalog parameters to LiveRule** — rejected because LiveRule's contract is metrics store data, not catalog data. Mixing concerns would complicate the interface.
2. **Run catalog rules as a separate engine** — rejected as unnecessary; the single engine with interface dispatch handles mixed rule types cleanly.
