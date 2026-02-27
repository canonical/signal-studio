# ADR-0011: Rules Sub-Package Extraction

**Status:** Accepted
**Date:** 2026-02-25
**Related:** [ADR-0003: Additional Static Rules](0003-additional-static-rules.md), [ADR-0005: Live Metrics Implementation](0005-live-metrics-implementation.md), [ADR-0007: Catalog-Based Recommendation Rules](0007-catalog-based-recommendation-rules.md)

---

## Context

The `internal/rules/` package grew to ~6300 lines across 10 files with three distinct rule tiers (static, live, catalog) that have different dependency profiles:

- **Static rules** depend only on `config` — they analyze YAML configuration.
- **Live rules** additionally depend on `metrics` — they analyze runtime telemetry.
- **Catalog rules** additionally depend on `tap` and `filter` — they analyze discovered metric catalogs and filter analyses.

All three tiers plus the engine, type definitions, and shared helpers lived in a single flat package. This made dependency relationships implicit and created a growing file that was harder to navigate.

---

## Decision

Extract `internal/rules/` into sub-packages organized by rule tier, with the root package containing only shared types and contracts.

### Package structure

```
internal/rules/
    types.go            Finding, Severity, Rule, LiveRule, CatalogRule
    helpers.go          HasProcessorType (shared by static + catalog)
    engine/
        engine.go       Engine, NewEngine, NewDefaultEngine, Evaluate*
        engine_test.go
    static/
        static.go       6 base rules
        extended.go     21 extended rules
        all.go          AllRules() []rules.Rule
        static_test.go
        extended_test.go
    live/
        live.go         4 live rules
        all.go          AllRules() []rules.Rule
        live_test.go
    catalog/
        catalog.go      9 catalog rules
        all.go          AllRules() []rules.Rule
        catalog_test.go
```

### Import graph

```
rules/           types + contracts (imported by everyone, imports nothing internal)
rules/static/    -> rules/
rules/live/      -> rules/
rules/catalog/   -> rules/
rules/engine/    -> rules/, rules/static, rules/live, rules/catalog
api/             -> rules/, rules/engine
```

The engine is the only package that imports all sub-packages. No import cycles exist because nothing imports `rules/engine` back.

### Key decisions

1. **`rules/` root is pure types.** The `Rule`, `LiveRule`, and `CatalogRule` interfaces plus `Finding` and `Severity` live here. This package imports only `config`, `metrics`, `filter`, and `tap` — all stable leaf packages.

2. **`HasProcessorType` exported to `rules/`.** This is the only helper shared across tiers (used by both `static` and `catalog` rules). All other helpers remain package-local.

3. **Each sub-package exposes `AllRules() []rules.Rule`.** The engine assembles them via `static.AllRules()`, `live.AllRules()`, `catalog.AllRules()`.

4. **Integration tests live in `engine/`.** Tests that exercise `NewDefaultEngine()` across all rule types belong with the engine that assembles them.

---

## Consequences

- **Explicit dependencies.** Each rule tier's imports are visible in its `import` block.
- **Easier navigation.** Files are grouped by tier rather than mixed in a flat directory.
- **Scalable.** New rule tiers can be added as new sub-packages without touching existing ones.
- **API consumer change.** `api/handlers.go` now imports `rules/engine` for `engine.NewDefaultEngine()` and `rules` for the `Finding` type.
- **No behavioral changes.** All existing rules, thresholds, and evaluation logic are preserved exactly.
