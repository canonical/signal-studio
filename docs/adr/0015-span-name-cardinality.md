# ADR-0015: Span Name Cardinality Management

**Status:** Pending
**Date:** 2026-02-25
**Related:** [ADR-0013: Multi-Signal Tap](0013-multi-signal-tap.md)

---

## Context

ADR-0013 introduces trace support in the OTLP sampling tap, with span catalog entries keyed by `(serviceName, spanName)`. In practice, span names can be high-cardinality — HTTP frameworks often produce span names like `GET /users/123`, `GET /users/456`, etc., where path parameters create unbounded distinct keys.

The metric catalog avoids this problem because metric names are stable by convention. The span catalog needs a strategy to prevent unbounded growth.

Alternatives to evaluate:

- **Cap on distinct span names per service** — once a service exceeds N distinct span names, stop tracking new ones and flag it as high-cardinality
- **URL/path normalization** — collapse path segments that look like IDs (UUIDs, numeric) into placeholders (e.g., `GET /users/{id}`)
- **Instrumentation-level grouping** — group by `(service, spanKind, instrumentationScope)` instead of individual span names, with span names as bounded samples (similar to attribute sample values)
- **No normalization, rely on TTL** — let TTL expiry naturally bound the catalog, accepting that it may be large during active sampling

This ADR should be resolved before implementing the span catalog portion of ADR-0013.

---

## Decision

_Pending._
