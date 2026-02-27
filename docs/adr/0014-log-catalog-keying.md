# ADR-0014: Log Catalog Keying Strategy

**Status:** Accepted
**Date:** 2026-02-25
**Related:** [ADR-0013: Multi-Signal Tap](0013-multi-signal-tap.md)

---

## Context

ADR-0013 introduces log support in the OTLP sampling tap. Unlike metrics (keyed by metric name) and traces (keyed by service + span name), logs lack a natural stable identifier to group catalog entries by.

The initial implementation uses `(resourceName, severityRange)` as the key, producing very coarse entries — roughly one row per service per severity bucket. This is an inventory, not a useful catalog.

### Research Findings

Every major observability platform keys logs primarily on **service/source identity**, not on content or severity:

- **Grafana Loki** — label sets anchored on `service_name` + infrastructure attributes
- **Datadog** — `service` + `source` + `status` (status is a filterable dimension, not the primary key)
- **Elastic** — data stream naming: `logs-{dataset}-{namespace}` where dataset encodes service+type

The OTLP protocol itself nests logs as Resource → InstrumentationScope → LogRecord[], hinting that Resource + Scope is the natural grouping boundary.

**InstrumentationScope.Name** is the logger name for bridged logs (the vast majority of real-world OTel log traffic). In Java this is the fully-qualified class name, in Python the module name, in Go whatever string is passed to the logger factory. It is stable per code path but may be empty for collector-sourced logs (filelog, syslog receivers).

**event.name** is a top-level LogRecord field designed as a stable identifier for structured log events. However, adoption is limited — only OTel-native instrumentation currently uses it. Most real-world logs are bridged from existing frameworks and do not set event.name.

**Body template extraction** (e.g. the Drain algorithm) is too expensive and fragile for a primary key — better suited as an analytical feature layered on top.

---

## Decision

### Composite Key with Graceful Degradation

Key each log catalog entry by: **`(service.name, scope.name, event.name)`** with tier-based fallback:

| Tier | Key | When | Classification |
|------|-----|------|----------------|
| 1 | `(service, scope, event.name)` | `event.name` is present | **Event** |
| 2 | `(service, scope, "")` | Scope is present, no event name | **Log** |
| 3 | `(service, "unscoped", "")` | Neither scope nor event name set | **Log** |

### Severity as a Distribution, Not a Key Dimension

Severity moves out of the key and becomes a **per-entry distribution**:

```go
type LogEntry struct {
    ServiceName    string
    ScopeName      string
    EventName      string
    LogKind        LogKind           // "event" or "log"
    SeverityCounts map[SeverityRange]int64
    // ... attributes, timestamps, etc.
}
```

A single catalog row for `(my-service, com.example.UserController)` aggregates logs across all severity levels. The severity distribution is displayed as color-coded counter pills in the UI (e.g. `INFO 1204` `WARN 38` `ERROR 7`).

### Log Kind Classification

Each entry is classified as either **event** or **log** based on how the composite key resolved:

- **Event**: `event.name` was present — structured, named log event following OTel conventions
- **Log**: plain log record from a bridged logging framework or collector receiver

This classification is displayed in the UI and enables future live rules such as:

- Detecting when a high proportion of logs are unstructured (no event name, no scope)
- Flagging services that could benefit from adopting structured events
- Identifying unscoped log sources that lack instrumentation metadata

---

## Consequences

### Positive

- Aligns with how every major observability platform groups logs — by source identity
- Produces meaningful catalog rows (per-logger or per-event-type) instead of coarse per-severity buckets
- Severity distribution per entry is more informative than separate rows — a spike in error ratio is immediately visible
- Graceful degradation: works with rich OTel events, bridged logs, and bare collector-sourced logs
- Log kind classification is a free byproduct of the keying logic
- Opens the door for live rules that analyze log structure quality

### Negative

- More complex key logic than the current simple two-field key
- Scope name granularity varies by language ecosystem (hundreds of loggers in Java, few in Go)
- Entries may accumulate many severity buckets over time, increasing memory per entry slightly
- The "unscoped" fallback tier may still produce coarse entries for collector-sourced logs
