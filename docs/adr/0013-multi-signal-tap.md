# ADR-0013: Multi-Signal Tap — Logs and Traces Support

**Status:** Proposed
**Date:** 2026-02-25
**Related:** [ADR-0006: Metric Name Discovery and Filter Analysis](0006-metric-name-discovery-and-filter-analysis.md), [ADR-0009: Attribute Discovery for Filter Authoring](0009-attribute-discovery-for-filter-authoring.md), [ADR-0012: UI-Controlled Tap](0012-ui-controlled-tap.md)
**Deferred decisions:** [ADR-0014: Log Catalog Keying Strategy](0014-log-catalog-keying.md), [ADR-0015: Span Name Cardinality Management](0015-span-name-cardinality.md)

---

## Context

The OTLP sampling tap (ADR-0006) currently receives only metrics. The receiver registers `pmetricotlp.GRPCServer` and handles `POST /v1/metrics` over HTTP, ignoring logs and traces entirely. Yet the Collector pipelines Signal Studio analyzes routinely carry all three signal types, and users ask the same questions about logs and traces that they ask about metrics:

- What log sources are flowing through this pipeline?
- What trace service names and span names exist?
- If I add a filter processor, which logs/traces would it drop?

The `pdata v1.52.0` dependency already includes `plog`, `plogotlp`, `ptrace`, and `ptraceotlp` — no new Go dependencies are needed. The receiver, catalog, and extraction patterns established for metrics transfer naturally to the other signal types.

---

## Problem Breakdown

### What the Tap Currently Captures (Metrics)

For each incoming OTLP metric export request the receiver extracts:

- Metric name, type (gauge/sum/histogram/summary/exp\_histogram)
- Attribute keys and bounded sample values (resource, scope, datapoint levels)
- Point count and scrape count
- Timestamps (first seen, last seen)

This metadata feeds three downstream consumers:

1. **MetricCatalogPanel** — searchable table with attribute drill-down
2. **Filter analysis** — `AnalyzeFilterWithAttributes()` predicts keep/drop outcomes per metric
3. **Catalog rules** — 9 rules that detect issues like high cardinality, unfiltered internal metrics, volume outliers

### What Logs and Traces Would Need

Logs and traces have different structures and different questions to answer, but share the same OTLP hierarchy: Resource → Scope → Signal-specific records.

**Logs:**
- Log record body (often a message template or structured key)
- Severity level (trace, debug, info, warn, error, fatal)
- Resource/scope/log record attributes
- Volume: record count per source

**Traces:**
- Service name (from `service.name` resource attribute)
- Span name, span kind (client, server, internal, producer, consumer)
- Status (unset, ok, error)
- Resource/scope/span attributes
- Volume: span count per service/operation

---

## Approaches

### A. Single Unified Catalog

Merge all three signal types into one catalog keyed by a composite identifier (signal type + name). A single `CatalogEntry` struct would use a `Signal` discriminator field.

**Pros:**
- Single API endpoint, single frontend component, minimal new code paths
- Reuses the existing TTL, pruning, and rate-change detection logic unchanged

**Cons:**
- The entry model becomes awkward — metrics have "type" (gauge/sum/histogram) while logs have "severity" and traces have "span kind" and "status", so the struct needs many optional fields
- Filter analysis currently assumes metric entries; making it signal-generic complicates the matching logic
- Sorting and searching a mixed-signal table is harder for users to navigate

### B. Signal-Specific Catalogs in a Single Manager

Keep the existing `MetricEntry`-based catalog for metrics. Add parallel `LogEntry` and `SpanEntry` catalogs, each with their own data model, but coordinated by the same `Manager`. The receiver registers all three OTLP services and routes data to the appropriate catalog.

**Pros:**
- Each signal type gets a natural data model — no optional fields or discriminators
- Filter analysis can evolve independently per signal type
- Frontend can present signal-specific views (tabs or separate drawers) with appropriate columns
- Existing metric catalog and its 9 rules remain untouched
- The manager already coordinates lifecycle (start/stop/status) — adding two more catalogs is a small extension

**Cons:**
- Three catalog types means three extraction functions, three sets of tests, three API response shapes
- Slightly more code surface area than a unified approach

### C. Phased — Traces Only, Then Logs

Implement traces first (they have the most natural parallel to metrics: span names are analogous to metric names, span counts to point counts), then add logs in a follow-up.

**Pros:**
- Reduces the initial scope
- Traces are the most commonly filtered signal type after metrics

**Cons:**
- Two rounds of receiver/catalog/frontend changes instead of one
- Logs and traces extensions follow the same pattern, so there's little learning benefit in splitting them

---

## Recommendation: Option B — Signal-Specific Catalogs

Option B provides the cleanest data model and the most natural user experience. The implementation cost of three extraction functions is modest given the established pattern, and it avoids polluting the well-tested metrics path with signal-generic abstractions.

Logs and traces should be implemented together (rejecting Option C's phased split) because the receiver changes, catalog scaffolding, and frontend tab infrastructure are shared work that would be duplicated by splitting.

---

## Design

### Receiver Extensions

The `Receiver` struct embeds `pmetricotlp.UnimplementedGRPCServer` today. Extend it to also embed `plogotlp.UnimplementedGRPCServer` and `ptraceotlp.UnimplementedGRPCServer`:

```go
type Receiver struct {
    pmetricotlp.UnimplementedGRPCServer
    plogotlp.UnimplementedGRPCServer
    ptraceotlp.UnimplementedGRPCServer
    // ...
}
```

Register all three gRPC services in `Start()`:

```go
pmetricotlp.RegisterGRPCServer(r.grpcServer, r)
plogotlp.RegisterGRPCServer(r.grpcServer, r)
ptraceotlp.RegisterGRPCServer(r.grpcServer, r)
```

Add HTTP routes `POST /v1/logs` and `POST /v1/traces` alongside the existing `POST /v1/metrics`, following the same proto/JSON unmarshaling pattern.

### New Data Models

```go
// LogEntry represents a discovered log source in the catalog.
type LogEntry struct {
    SeverityText  string          // e.g. "ERROR", "INFO"
    SeverityRange SeverityRange   // grouped severity bucket
    ResourceName  string          // service.name resource attribute or scope name
    Attributes    []AttributeMeta // resource, scope, log-record levels
    RecordCount   int64
    ScrapeCount   int64
    LastSeenAt    time.Time
    FirstSeenAt   time.Time
}

// SpanEntry represents a discovered trace operation in the catalog.
type SpanEntry struct {
    ServiceName string          // from service.name resource attribute
    SpanName    string          // operation name
    SpanKind    string          // client, server, internal, producer, consumer
    StatusCode  string          // unset, ok, error
    Attributes  []AttributeMeta // resource, scope, span levels
    SpanCount   int64
    ScrapeCount int64
    LastSeenAt  time.Time
    FirstSeenAt time.Time
}
```

**Catalog keying:**
- Log entries — keying strategy deferred to [ADR-0014](0014-log-catalog-keying.md)
- Span entries — keyed by `(serviceName, spanName)`, with cardinality management deferred to [ADR-0015](0015-span-name-cardinality.md)

### Manager Changes

```go
type Manager struct {
    // existing
    catalog   *Catalog      // metrics

    // new
    logCatalog  *LogCatalog
    spanCatalog *SpanCatalog
}
```

`Stop()` and `Start()` manage all three catalogs. `Status()` is unchanged — it reflects the receiver lifecycle, not per-signal state.

### Extraction Functions

Follow the existing `extractAndRecord` pattern:

- `extractAndRecordLogs(logs plog.Logs, catalog *LogCatalog)` — walks ResourceLogs → ScopeLogs → LogRecord, extracts severity, resource name, attributes, and record count.
- `extractAndRecordSpans(traces ptrace.Traces, catalog *SpanCatalog)` — walks ResourceSpans → ScopeSpans → Span, extracts service name, span name, kind, status, attributes, and span count.

Both reuse the existing `pcommonMapKVs()` and `attrTracker` for bounded attribute sampling.

### API Endpoints

Extend the existing `/api/tap/catalog` or add signal-specific endpoints:

| Endpoint | Returns |
|----------|---------|
| `GET /api/tap/catalog` | `{ metrics: MetricEntry[], logs: LogEntry[], spans: SpanEntry[] }` |

A single endpoint simplifies the frontend polling (one fetch, one interval). The response includes all three arrays — empty arrays for signals with no data. The existing `count` and `rateChanged` fields become per-signal.

### Frontend

Add a signal selector (tabs or segmented control) to `MetricCatalogPanel`, renaming it to `CatalogPanel`:

- **Metrics tab** — existing table (name, type, points/scrape, total, attributes)
- **Logs tab** — table with columns: source, severity, record count, attributes
- **Traces tab** — table with columns: service, operation, kind, status, span count, attributes

The `useTap` hook returns all three entry arrays. Tab counts in the selector show how many entries each signal has.

### Collector Config Snippet

Update the generated snippet to include all three pipeline types:

```yaml
exporters:
  otlp/signal-studio:
    endpoint: "localhost:5317"
    tls:
      insecure: true

service:
  pipelines:
    metrics:
      exporters: [..., otlp/signal-studio]
    logs:
      exporters: [..., otlp/signal-studio]
    traces:
      exporters: [..., otlp/signal-studio]
```

---

### Basic Trace Filter Analysis

The existing metric filter analyzer matches `name == "..."` and `IsMatch(name, "...")` OTTL conditions against metric names. Trace filter processors use the same OTTL patterns against span names. Since the matching code already exists, extend `AnalyzeFilterWithAttributes` (or add a parallel `AnalyzeTraceFilter`) to evaluate trace pipeline filter processors against the span catalog.

This makes the traces tab immediately actionable — users see which spans a filter would keep or drop, weighted by span count, just like the metrics tab today. Attribute-level trace filter conditions (`status`, `kind`, `attributes[...]`) are out of scope for this ADR and can follow the same path as ADR-0009 did for metric attributes.

---

## Future Work (Out of Scope)

- **Log filter analysis** — extending filter analysis to evaluate log pipeline filter processors. Requires resolving log catalog keying (ADR-0014) first, and understanding OTTL conditions specific to logs (`severity_number`, `body`).
- **Attribute-level trace filter analysis** — evaluating OTTL conditions beyond span name matching (e.g. `status.code`, `kind`, `attributes["..."]`).
- **Log/trace catalog rules** — new rule implementations (e.g. "error log volume spike", "high-cardinality span names", "unfiltered debug logs") analogous to the 9 existing metric catalog rules.
- **Cross-signal correlation** — linking metrics to the traces or logs they originate from (e.g. same `service.name` resource attribute).

---

## Consequences

### Positive

- Users gain full visibility into all three signal types flowing through their Collector, not just metrics
- No new Go dependencies — `plog`, `plogotlp`, `ptrace`, `ptraceotlp` are already available in `pdata v1.52.0`
- The established extraction and catalog patterns transfer directly — the implementation is mechanical
- Signal-specific data models avoid awkward optional fields and keep each signal's catalog clean
- The receiver already listens on gRPC and HTTP — registering additional services is a small change
- The existing metric catalog, filter analysis, and 9 catalog rules remain completely untouched

### Negative

- Increased memory usage — three catalogs instead of one, each with bounded attribute tracking
- Larger API response payloads — a single `/api/tap/catalog` call returns all three signal arrays
- More test surface — three extraction functions and three catalog types each need coverage ≥80%
- Binary size may increase slightly from additional protobuf-generated code for log/trace types (expected to be small since the pdata module is already linked)
- The collector snippet becomes more complex (three pipeline sections) which may confuse new users
