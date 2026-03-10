# ADR-0020: Production Safety and Security Rules

## Status

Accepted

## Date

2026-03-10

## Related

- [ADR-0011: Rules Sub-Package Structure](0011-rules-sub-package-structure.md)
- [ADR-0019: Generated Rule Documentation](0019-generated-rule-documentation.md)

## Context

The current rule set covers common configuration mistakes (missing processors, misconfigured exporters, unused components) and runtime anomalies (drop rates, queue pressure). However, several high-value production and security scenarios are not yet detected:

1. **Exporter timeout too low** — The default OTLP exporter timeout is 5 seconds. In high-latency environments (cross-region exports, loaded backends), this causes unnecessary retries and amplified load. Operators frequently discover this only after investigating retry storms.

2. **Sending queue too small** — When `sending_queue` is enabled but `queue_size` is left at the default (1000 for OTLP), a brief backend outage at high throughput fills the queue within seconds. Operators expect the queue to provide meaningful buffer time but the default may not.

3. **No span-to-metrics connector** — When a collector has both traces and metrics pipelines but no `spanmetrics` connector, the operator may be missing an opportunity to derive RED metrics (Rate, Errors, Duration) from trace data without additional instrumentation.

4. **Extension endpoints exposed on wildcard** — Extensions like `zpages`, `pprof`, and `health_check` that bind to `0.0.0.0` are accessible from any network interface. While `pprof` is already flagged as an info-level finding when enabled, the wildcard binding issue applies to all debug/admin extensions and represents a more concrete security concern.

## Decision

Add four new static rules:

### `exporter-timeout-too-low`
- **Severity:** info
- **Fires when:** An OTLP exporter has `timeout` configured to less than 10 seconds, or uses the default (no explicit timeout).
- **Scope:** `exporter:<name>`
- **Rationale:** Low timeouts cause premature retries. The default 5s is often too short for production backends. This is informational — some environments genuinely need fast timeouts.
- **Skip when:** The exporter targets localhost (likely a local relay where 5s is fine).

### `sending-queue-too-small`
- **Severity:** info
- **Fires when:** An OTLP exporter has `sending_queue.enabled: true` but `queue_size` is set below 500, or is absent (default 1000 is acceptable, so this only flags explicitly small values).
- **Scope:** `exporter:<name>`
- **Rationale:** A queue of 100–200 entries provides only seconds of buffering at moderate throughput. Operators who enable the queue typically expect it to absorb backend outages lasting minutes.
- **Revised threshold:** Flag when `queue_size` is explicitly set to < 500. The default (1000) is reasonable and should not be flagged — the rule targets conscious-but-too-low choices.

### `no-span-metrics-connector`
- **Severity:** info
- **Fires when:** The config has at least one traces pipeline and at least one metrics pipeline, but no `spanmetrics` connector is defined.
- **Scope:** global (no specific component)
- **Rationale:** The `spanmetrics` connector is one of the most impactful collector features for deriving RED metrics from traces. Many operators are unaware of it. This is purely informational.
- **Skip when:** A `spanmetrics` connector is already defined (in `connectors` map), regardless of whether it is wired into a pipeline.

### `extension-endpoint-exposed`
- **Severity:** warning
- **Fires when:** An active extension (`zpages`, `pprof`, `health_check`) has an endpoint binding to `0.0.0.0` or `::`.
- **Scope:** `extension:<name>`
- **Rationale:** These endpoints expose operational data or profiling capabilities. Binding to wildcard makes them accessible from any network. The existing `pprof-extension-enabled` rule flags pprof being active at all; this rule specifically targets the binding exposure for all admin extensions.
- **Skip when:** The endpoint binds to `localhost`, `127.0.0.1`, or `[::1]`.

All four are static rules (config-only, no runtime data needed) placed in `rules/static/`.

Additionally, add three new live rules that detect runtime anomalies from scraped Prometheus metrics:

### `live-exporter-sustained-failures`
- **Severity:** critical
- **Fires when:** An exporter's `send_failed_*` rate is non-zero for 3+ consecutive intervals.
- **Scope:** `exporter:<name>`
- **Rationale:** This is the most actionable runtime signal — the exporter is actively losing data. The `send_failed` counters are already scraped but no rule evaluates them.

### `live-receiver-backpressure`
- **Severity:** warning
- **Fires when:** Receiver accepted rate drops >50% compared to the baseline (average of earlier intervals), sustained for 2 consecutive intervals. Requires baseline ≥ 10/s to avoid noise.
- **Scope:** `signal:<name>`
- **Rationale:** A sharp drop in accepted rate typically means memory_limiter is applying backpressure. This surfaces memory pressure before it leads to OOM kills.

### `live-zero-throughput`
- **Severity:** warning
- **Fires when:** All receiver accepted rates (spans, metric points, log records) are zero for 3+ consecutive intervals.
- **Scope:** global
- **Rationale:** Catches the "deployed but nothing flows" scenario — misconfigured SDK endpoints, network issues, or missing instrumentation.

## Consequences

### Positive
- Catches seven common production pitfalls that currently go undetected
- Static rules are informational or low-severity, avoiding false-positive fatigue
- Live rules surface critical runtime issues (sustained failures, backpressure, zero throughput)
- No new dependencies — static rules inspect `CollectorConfig`, live rules use already-scraped metrics
- Extends the rule count from 27 static + 4 live to 31 static + 7 live

### Negative
- `no-span-metrics-connector` may feel opinionated for operators who intentionally don't use spanmetrics — mitigated by info severity and the "However" clause in the implication
- `exporter-timeout-too-low` threshold (10s) is a judgement call — some operators may find it too conservative or too aggressive
- `live-receiver-backpressure` uses a 50% drop threshold which may trigger during legitimate traffic ramp-downs — mitigated by requiring baseline ≥ 10/s and medium confidence

## Implementation

1. Add four new files in `rules/static/`:
   - `exporter_timeout_too_low.go`
   - `sending_queue_too_small.go`
   - `no_span_metrics_connector.go`
   - `extension_endpoint_exposed.go`

2. Add three new files in `rules/live/`:
   - `exporter_sustained_failures.go`
   - `receiver_backpressure.go`
   - `zero_throughput.go`

3. Register them in `rules/static/all.go` and `rules/live/all.go`

4. Add tests for all rules

5. Regenerate `docs/rules.md` via `go generate`

6. Verify coverage stays ≥ 80%
