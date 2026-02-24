# ADR-0005: Live Metrics Implementation

**Status:** Proposed
**Date:** 2026-02-23
**Related:** [ADR-0001: Project Specification](0001-project-specification.md), [ADR-0002: Tech Stack and Initial Architecture](0002-tech-stack-and-initial-architecture.md)

---

## Context

ADR-0001 defines live Collector metrics analysis as a core MVP capability (Section 3.1B). The tool should connect to a Collector's Prometheus metrics endpoint, display throughput/drop/queue data, and power live rules 4–7. This ADR designs the backend scraper, API surface, frontend integration, and live rule implementation.

The user has expressed a preference for integrating live metrics into the existing Config Studio UI rather than building a separate screen.

---

## OTel Collector Prometheus Metrics

The Collector exposes self-observability metrics at `http://localhost:8888/metrics` by default (configurable via `service.telemetry.metrics.address`). All metrics use Prometheus text exposition format.

### Key Metric Families

| Metric                                    | Type    | Labels                  | Description                 |
| ----------------------------------------- | ------- | ----------------------- | --------------------------- |
| `otelcol_receiver_accepted_spans`         | Counter | `receiver`, `transport` | Spans accepted by receivers |
| `otelcol_receiver_refused_spans`          | Counter | `receiver`, `transport` | Spans refused by receivers  |
| `otelcol_receiver_accepted_metric_points` | Counter | `receiver`, `transport` | Metric points accepted      |
| `otelcol_receiver_accepted_log_records`   | Counter | `receiver`, `transport` | Log records accepted        |
| `otelcol_exporter_sent_spans`             | Counter | `exporter`              | Spans successfully sent     |
| `otelcol_exporter_send_failed_spans`      | Counter | `exporter`              | Spans that failed to send   |
| `otelcol_exporter_sent_metric_points`     | Counter | `exporter`              | Metric points sent          |
| `otelcol_exporter_sent_log_records`       | Counter | `exporter`              | Log records sent            |
| `otelcol_exporter_queue_size`             | Gauge   | `exporter`              | Current queue size          |
| `otelcol_exporter_queue_capacity`         | Gauge   | `exporter`              | Queue capacity              |
| `otelcol_processor_accepted_spans`        | Counter | `processor`             | Spans accepted by processor |
| `otelcol_processor_dropped_spans`         | Counter | `processor`             | Spans dropped by processor  |

### Important Notes

1. **`_total` suffix**: Collector versions ≥ v0.119.0 append `_total` to counter names (e.g., `otelcol_receiver_accepted_spans_total`). The scraper must handle both forms.
2. **Processor drop metrics are sparse**: Not all processors emit `dropped_*` counters. Drop rates should be computed as `receiver_accepted - exporter_sent` at the pipeline level when per-processor metrics are unavailable.
3. **Labels map to config names**: The `receiver`, `exporter`, and `processor` label values correspond directly to the component names in the Collector YAML config, enabling correlation between static config analysis and live metrics.

---

## Design

### Backend

#### Package: `internal/metrics`

```
internal/metrics/
├── scraper.go       # Prometheus endpoint scraper + parser
├── store.go         # In-memory time-windowed metric store
├── rates.go         # Rate computation from counter deltas
└── scraper_test.go
```

**Scraper** (`scraper.go`):

- Fetches `/metrics` from a configurable endpoint URL
- Parses Prometheus text exposition format (use `github.com/prometheus/common/expfmt`)
- Normalizes metric names by stripping `_total` suffix for consistent lookup
- Accepts optional Bearer token for authenticated endpoints
- Returns a `MetricSnapshot` — a flat map of `{metricName+labels → float64}`

**Store** (`store.go`):

- Holds a sliding window of the last N snapshots (default: 6 snapshots at 10s intervals = 1 minute)
- Thread-safe (mutex-protected) for concurrent read/write from HTTP handlers and the scrape ticker
- Provides accessor methods: `Latest()`, `RatePerSecond(metricName, labels)`, `Window()`
- Automatically evicts snapshots older than the window

**Rate computation** (`rates.go`):

- For counters: `rate = (latest_value - previous_value) / elapsed_seconds`
- For gauges (queue size/capacity): use latest value directly
- Handles counter resets (value decreases → skip that interval)
- Computes derived metrics:
  - `drop_rate_pct = (accepted - sent) / accepted × 100` per signal per pipeline
  - `queue_utilization_pct = queue_size / queue_capacity × 100` per exporter

#### Scrape Lifecycle

1. Frontend calls `POST /api/metrics/connect` with the endpoint URL + optional token
2. Backend validates connectivity with a single test scrape
3. On success, starts a background goroutine with a `time.Ticker` (default 10s, configurable 5–30s via `SCRAPE_INTERVAL_SECONDS`)
4. Each tick fetches, parses, and stores a snapshot
5. Frontend polls `GET /api/metrics/snapshot` to retrieve computed rates
6. `POST /api/metrics/disconnect` stops the ticker and clears the store

Only one active metrics connection is supported at a time (single-user tool).

#### New API Endpoints

| Method | Path                      | Description                                                          |
| ------ | ------------------------- | -------------------------------------------------------------------- |
| `POST` | `/api/metrics/connect`    | Start scraping. Body: `{"url": "...", "token": "..."}`               |
| `POST` | `/api/metrics/disconnect` | Stop scraping, clear store                                           |
| `GET`  | `/api/metrics/snapshot`   | Latest computed rates + raw gauges                                   |
| `GET`  | `/api/metrics/status`     | Connection state: `disconnected`, `connecting`, `connected`, `error` |

**Snapshot response shape:**

```json
{
  "status": "connected",
  "collectedAt": "2026-02-23T14:30:00Z",
  "signals": {
    "traces": {
      "receiverAcceptedRate": 1250.3,
      "exporterSentRate": 1100.8,
      "exporterFailedRate": 2.1,
      "dropRatePct": 11.9
    },
    "metrics": { ... },
    "logs": { ... }
  },
  "exporters": {
    "otlp/backend": {
      "queueSize": 145,
      "queueCapacity": 1000,
      "queueUtilizationPct": 14.5,
      "sentRate": 800.2,
      "failedRate": 0.0
    }
  },
  "receivers": {
    "otlp": {
      "acceptedSpansRate": 1250.3,
      "acceptedMetricPointsRate": 5400.0,
      "acceptedLogRecordsRate": 3200.1
    }
  }
}
```

This pre-computed shape keeps the frontend simple — no rate math in the browser.

#### Why Polling Over SSE/WebSocket

- Polling at 10s intervals is trivially simple and matches the scrape cadence
- No connection lifecycle management on the frontend
- Consistent with the read-only, stateless design philosophy
- SSE can be added later if sub-second updates become desirable

### Frontend Integration

Rather than a separate "Live Metrics" screen, metrics are integrated directly into the existing Config Studio UI. This creates a unified view where the user sees both structural analysis and live behavior in one place.

#### Integration Points

**1. Metrics connection bar (top of main panel)**

A compact connection bar appears at the top of the Pipelines panel:

- Disconnected state: URL input field + "Connect" button (inline, single row)
- Connecting state: spinner
- Connected state: green dot + endpoint URL + "Disconnect" button
- Error state: red dot + error message + "Retry" button

This does not require a new panel or screen — it is a thin bar above the pipeline cards.

**2. Throughput overlay on pipeline cards**

When metrics are connected, each pipeline card gains a small metrics row at the bottom:

```
┌─────────────────────────┐
│ RECEIVERS             ● │  ← existing card header + status
│ otlp                    │  ← existing component list
│   ↳ grpc 0.0.0.0:4317   │
│                         │
│ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │  ← faint separator
│ 1,250 spans/s           │  ← NEW: live rate overlay
└─────────────────────────┘
```

For receivers: show accepted rate per signal.
For exporters: show sent rate + queue utilization percentage.
For processors: show accepted rate (if available).

If metrics are disconnected or unavailable for a component, this row simply doesn't render (graceful degradation).

**3. Signal summary bar (above pipeline cards)**

A horizontal bar showing aggregate throughput per signal type:

```
Traces: 1,250/s in → 1,100/s out (12% drop)  |  Logs: 3,200/s in → 3,180/s out  |  Metrics: 5,400/s in → 5,400/s out
```

This gives an at-a-glance view of overall pipeline health.

**4. Findings from live rules**

Live rules 4–7 produce `Finding` objects with the same structure as static rules. They appear in the existing Recommendations panel alongside static findings. The `ruleId` prefix `live-` distinguishes them (e.g., `live-high-drop-rate`).

### Live Rules

Live rules implement the same `Rule` interface but additionally receive metric data. This requires a small extension:

```go
type LiveRule interface {
    Rule
    EvaluateWithMetrics(cfg *config.CollectorConfig, metrics *metrics.Snapshot) []Finding
}
```

The engine calls `EvaluateWithMetrics` when a snapshot is available, falling back to `Evaluate` (which returns nil for live rules).

#### Rule 4: High processor drop rate (`live-high-drop-rate`)

- **Condition:** `(accepted - sent) / accepted > 10%` for any signal in any pipeline, sustained over 2+ snapshots
- **Severity:** warning
- **Evidence:** includes actual drop rate percentage and affected signal/pipeline
- **Note:** Computed from receiver accepted vs exporter sent since per-processor drop counters are not universally available

#### Rule 5: Logs dominating volume (`live-log-volume-dominance`)

- **Condition:** Log ingest rate > 3× trace ingest rate
- **Severity:** info
- **Evidence:** shows actual rates for both signals
- **Recommendation:** severity filtering, attribute pruning

#### Rule 6: Exporter queue near capacity (`live-queue-near-capacity`)

- **Condition:** `queue_size / queue_capacity > 80%` sustained over 2+ snapshots
- **Severity:** warning
- **Evidence:** shows queue utilization percentage and exporter name
- **Recommendation:** increase queue capacity, add batch processor, reduce volume

#### Rule 7: Receiver-exporter mismatch (`live-receiver-exporter-mismatch`)

- **Condition:** `receiver_accepted_rate > 2 × exporter_sent_rate` for a signal, sustained over 3+ snapshots (to exclude transient bursts)
- **Severity:** warning
- **Evidence:** shows both rates and the gap
- **Recommendation:** check processor drops, exporter errors, queue utilization

"Sustained" checks prevent false positives from transient spikes during startup or brief load changes.

---

## Implementation Plan

### Phase 1: Backend scraper + API (estimated 2–3 days)

1. Add `github.com/prometheus/common` dependency for metric parsing
2. Implement `internal/metrics/scraper.go` — fetch + parse + normalize `_total`
3. Implement `internal/metrics/store.go` — sliding window store
4. Implement `internal/metrics/rates.go` — rate computation + derived metrics
5. Add API endpoints in `internal/api/` — connect, disconnect, snapshot, status
6. Tests: mock HTTP server returning sample Prometheus output, verify rate computation, test `_total` normalization, test counter reset handling

### Phase 2: Frontend integration (estimated 2 days)

1. Add `MetricsConnection` component — URL input, connect/disconnect, status indicator
2. Add metrics polling hook (`useMetricsPolling`) — fetches `/api/metrics/snapshot` on interval when connected
3. Add throughput overlay to pipeline cards (conditional on metrics availability)
4. Add signal summary bar above pipeline cards
5. TypeScript types for snapshot response

### Phase 3: Live rules (estimated 1–2 days)

1. Extend rule engine with `LiveRule` interface
2. Implement rules 4–7 with sustained-check logic
3. Wire live rule evaluation into snapshot handler (evaluate on each new snapshot, merge findings into response)
4. Tests for each live rule with mock metric data

### Phase 4: Combined analysis endpoint (estimated 0.5 day)

1. Update `/api/metrics/snapshot` to include live rule findings alongside metric data
2. Frontend merges static + live findings in the Recommendations panel
3. Update `CardFilter` to handle live rule IDs

---

## Alternatives Considered

### A. Separate Live Metrics screen

A dedicated tab/screen showing only metrics data, disconnected from the config view.

**Rejected** because the user explicitly wants metrics integrated into the existing Config Studio. Separating them loses the correlation between config structure and live behavior.

### B. Server-Sent Events for real-time streaming

Push metric updates to the frontend via SSE instead of polling.

**Deferred** — polling at 10s intervals is simpler and sufficient for the MVP. SSE adds connection management complexity (reconnection, buffering) for minimal UX benefit at this cadence.

### C. Embed a full Prometheus client library

Use the full Prometheus Go client to parse metrics.

**Partially adopted** — we use only `github.com/prometheus/common/expfmt` for text format parsing, not the full client. This keeps the dependency lightweight.

### D. Store metrics in SQLite for historical analysis

Persist metric snapshots for trend analysis beyond the sliding window.

**Deferred** — ADR-0001 explicitly scopes the MVP as in-memory only. Historical analysis is listed as future work.

---

## Consequences

### Positive

- Users see live throughput data directly on the pipeline cards they already understand
- Live rules produce findings in the same format as static rules — no new UI concepts
- Polling keeps the frontend simple and stateless
- Graceful degradation: everything works without metrics connected, metrics overlay simply doesn't render

### Negative

- Adds `github.com/prometheus/common/expfmt` as a dependency
- Background goroutine for scraping adds lifecycle management (must clean up on disconnect, handle endpoint unavailability)
- `_total` suffix normalization adds complexity to metric name matching
- Rate computation from counter deltas can produce inaccurate results during the first scrape interval (no previous value to diff against) — mitigated by skipping the first interval
- Sustained-check logic for live rules requires storing state across snapshots, making rules slightly more complex than stateless static rules
