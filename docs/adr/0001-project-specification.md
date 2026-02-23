# ADR-0001: Build a Read-Only OTel Collector Noise Analysis Tool

**Status:** Proposed  
**Date:** 2026-02-23  
**Decision Makers:** Project Founder  
**Related:** OpenTelemetry Collector-based environments

---

## 1. Context

Teams running OpenTelemetry Collector often experience:

- Increasing telemetry volume and cost
- Noisy traces, logs, and metrics
- Misconfigured pipelines
- Limited visibility into collector behavior
- Lack of safe guidance on what telemetry can be reduced

Existing tools either:

- Visualize configuration statically (limited scope), or
- Operate as full observability backends (high infrastructure overhead), or
- Provide ingestion controls tied to specific vendors

There is currently no lightweight, vendor-neutral, read-only tool focused on:

> Diagnosing telemetry waste and generating safe configuration improvements for OpenTelemetry Collector.

---

## 2. Decision

We will build a **read-only diagnostic assistant** for OpenTelemetry Collector.

Working description:

> A tool that visualizes collector pipelines, inspects live ingest/export metrics, and generates safe telemetry-reduction recommendations — without deploying or modifying infrastructure.

The MVP will:

1. Parse and visualize Collector YAML configuration.
2. Scrape and interpret Collector Prometheus metrics.
3. Generate rule-based recommendations with copy-paste YAML snippets.
4. Remain strictly read-only.

The MVP will **not** ingest telemetry, deploy changes, or function as a control plane.

---

## 3. Scope

### 3.1 Included in MVP

#### A. Configuration Analysis

- Accept raw OpenTelemetry Collector YAML (paste or upload)
- Parse:
  - receivers
  - processors
  - exporters
  - service pipelines (traces, metrics, logs)
- Render pipeline graph:
  - receivers → processors → exporters
- Provide component-level inspection
- Run static lint rules

#### B. Live Collector Metrics Analysis

- Connect to Prometheus metrics endpoint (read-only)
- Consume key Collector metrics:
  - `otelcol_receiver_accepted_*`
  - `otelcol_exporter_sent_*`
  - `otelcol_processor_dropped_*`
  - exporter queue metrics
- Display:
  - Ingest rate
  - Export rate
  - Drop rate %
  - Queue utilization %

No long-term storage required. In-memory only.

#### C. Noise Detection & Recommendations

A rule engine combining:

- Static config analysis
- Live throughput metrics

Output includes:

- Title
- Severity
- Explanation
- Why it matters
- Estimated impact (heuristic)
- Copy-paste YAML snippet

---

### 3.2 Explicitly Out of Scope

The MVP will NOT:

- Ingest OTLP data
- Store telemetry samples
- Simulate filters precisely
- Deploy configuration changes
- Provide GitOps integration
- Operate as SaaS multi-tenant platform
- Act as a Kubernetes operator
- Replace observability backends
- Implement cardinality estimation algorithms

This is a diagnostic tool only.

---

## 4. Architecture

### 4.1 High-Level Design

**Frontend (SPA)**

- Pipeline graph visualization
- Metrics dashboard
- Recommendation panel

**Backend (Single Service)**

Modules:

- YAML parser
- Config model builder
- Prometheus scraper
- Rule engine

Storage:

- In-memory only
- No persistent database required

Deployment:

- Single container

---

## 5. Data Model (Initial)

### Pipeline

```ts
Pipeline {
  signal: "traces" | "metrics" | "logs"
  receivers: string[]
  processors: string[]
  exporters: string[]
}
```

## 6. Initial Detection Rules (MVP)

Each rule produces:

- **Severity:** info | warning | critical
- **Evidence:** what was observed (config + metrics)
- **Recommendation:** what to change
- **Snippet:** copy-paste YAML
- **Impact estimate:** heuristic only (simple math based on current rates)

### Rule 1: Missing `memory_limiter` (Static)

**Condition:** No `memory_limiter` processor present in any pipeline.  
**Severity:** critical  
**Why it matters:** Prevents OOM and uncontrolled memory growth.  
**Recommendation:** Add `memory_limiter` early in each pipeline.

### Rule 2: Missing `batch` processor (Static)

**Condition:** No `batch` processor present in a pipeline.  
**Severity:** warning  
**Why it matters:** Reduces exporter overhead and improves throughput stability.  
**Recommendation:** Add `batch` near the end of each pipeline.

### Rule 3: No trace sampling configured (Static)

**Condition:** No sampling processor detected in trace pipeline (e.g., tail sampling, probabilistic).  
**Severity:** warning  
**Why it matters:** High trace volume drives cost and backend load.  
**Recommendation:** Add probabilistic or tail sampling with conservative defaults.

### Rule 4: High processor drop rate (Live)

**Condition:** `otelcol_processor_dropped_spans` / accepted spans > 10% (or equivalent for logs/metrics where available).  
**Severity:** warning  
**Why it matters:** Unexpected drops indicate backpressure, misconfiguration, or overload.  
**Recommendation:** Investigate memory limiter, batch, queue settings; validate filter processors.

### Rule 5: Logs dominating volume (Live)

**Condition:** log ingest rate > 3× trace ingest rate (or exceeds configurable threshold).  
**Severity:** info  
**Why it matters:** Log volume frequently dominates cost; many logs are low-value.  
**Recommendation:** Add severity filtering and/or attribute pruning.

### Rule 6: Exporter queue near capacity (Live)

**Condition:** `otelcol_exporter_queue_size` / `otelcol_exporter_queue_capacity` > 80% for sustained interval.  
**Severity:** warning  
**Why it matters:** Indicates exporter bottleneck and potential data loss.  
**Recommendation:** Increase queue capacity, add batch, reduce volume, or adjust retry/backoff.

### Rule 7: Receiver accepts but exporter sends far less (Live)

**Condition:** accepted rate >> sent rate for a signal, sustained (excluding expected sampling).  
**Severity:** warning  
**Why it matters:** Suggests drops, pipeline blockage, or exporter failure.  
**Recommendation:** Check processor dropped counters, exporter errors, and queue utilization.

### Rule 8: Unused components (Static)

**Condition:** receivers/processors/exporters defined but not referenced by any pipeline.  
**Severity:** info  
**Why it matters:** Adds confusion and configuration drift.  
**Recommendation:** Remove unused components or wire them correctly.

### Rule 9: Multiple exporters without routing clarity (Static)

**Condition:** multiple exporters configured per signal with unclear intent (e.g., no routing processor / no explicit separation).  
**Severity:** info  
**Why it matters:** Can duplicate telemetry or complicate troubleshooting.  
**Recommendation:** Add routing or separate pipelines explicitly.

### Rule 10: No log severity filtering (Heuristic Static)

**Condition:** logs pipeline exists but no filter/transform processor suggests severity control.  
**Severity:** info  
**Why it matters:** DEBUG/INFO floods are common cost drivers.  
**Recommendation:** Add severity filter or environment-based filtering.

---

## 7. Recommendation Output Format

Each recommendation MUST include:

- **Title**
- **Severity**
- **Evidence** (config elements and/or metric values)
- **Explanation**
- **Why it matters**
- **Estimated impact** (heuristic)
- **Copy-paste YAML snippet**
- **Placement hint** (where in the pipeline it should go)

### Impact Estimation Rules (Heuristic)

- Use current ingest/export rates from metrics (per signal).
- Provide simple “before/after” math (not simulation).
- Example:
  - Current trace export: 1200 spans/sec
  - Suggested probabilistic sampling at 20%
  - Estimated export: ~240 spans/sec

---

## 8. Metrics Requirements

### 8.1 Metrics Endpoint

The backend must scrape a Prometheus-format endpoint.

Input:

- URL
- Optional auth:
  - Bearer token (initial MVP)
  - (Future: basic auth / mTLS)

Update interval:

- Default 10 seconds (configurable 5–30s)

### 8.2 Minimum Metrics Set

Traces:

- `otelcol_receiver_accepted_spans`
- `otelcol_exporter_sent_spans`
- `otelcol_processor_dropped_spans`

Metrics:

- `otelcol_receiver_accepted_metric_points`
- `otelcol_exporter_sent_metric_points`

Logs:

- `otelcol_receiver_accepted_log_records`
- `otelcol_exporter_sent_log_records`

Internal/exporter health:

- `otelcol_exporter_queue_size`
- `otelcol_exporter_queue_capacity`

If some metrics are missing, UI must degrade gracefully and report what is unavailable.

---

## 9. UI Requirements

### 9.1 Main Screens

1. **Config Studio**
   - YAML input (paste/upload)
   - Pipeline graph (per signal tabs)
   - Component inspection drawer
   - Lint warnings list

2. **Live Metrics**
   - Endpoint config (URL + token)
   - Signal throughput cards (traces/metrics/logs)
   - Drop rate + queue utilization
   - Status indicator (connected / error)

3. **Recommendations**
   - Sorted by severity
   - Each item shows:
     - Evidence
     - Explanation
     - Snippet
     - Copy button
     - Placement guidance

### 9.2 UX Constraints

- The tool must clearly state **read-only**.
- No “Apply” or “Deploy” actions in MVP.
- Copy-paste is the primary workflow.

---

## 10. Security Considerations

- Credentials (token) stored in memory only.
- No telemetry payloads stored or transmitted.
- No writes to target environment.
- Avoid logging secrets (redaction in logs).
- Network egress limited to scraping metrics endpoint only.

---

## 11. Operational Requirements

- Single container deployment
- Stateless runtime (in-memory state only)
- Works on:
  - local Docker
  - Kubernetes (optional)
  - VM

Configuration options:

- `PORT`
- `SCRAPE_INTERVAL_SECONDS`
- `MAX_YAML_SIZE_KB`
- `ALLOW_REMOTE_METRICS` (default: true)
- `CORS_ORIGINS`

---

## 12. Alternatives Considered

### A) Full ingestion control plane

**Rejected** due to:

- complex deployment
- rollout risk
- simulation and state requirements
- startup-scale scope

### B) Build a telemetry backend

**Rejected** due to:

- heavy infra and storage
- competitive landscape
- scope mismatch

### C) Static config visualizer only

**Rejected** because:

- limited differentiation
- does not address “what is flowing” and “what is waste”

---

## 13. Consequences

### Positive

- Low infrastructure burden
- Safer adoption (read-only)
- Fast iteration
- Vendor-neutral
- Useful for cost/noise diagnosis quickly

### Negative

- No automated enforcement
- Impact estimates are approximate
- Limited historical analysis without persistence

---

## 14. Success Criteria

MVP is successful if:

- Users can visualize a pipeline from YAML within 2 minutes.
- Tool identifies at least 5 actionable warnings in typical configs.
- Live throughput appears within 10 seconds of connecting to metrics.
- At least 3 recommendations are generated that users report as useful.
- Users can apply changes by copying snippets confidently.

---

## 15. Future Work (Explicitly Deferred)

- Telemetry sampling/preview via OTLP mirror
- Cardinality estimation (HLL / Top-K)
- GitOps PR generation
- Auto-deploy / canary rollout
- Multi-tenant SaaS + RBAC
- Collector version compatibility database
- PII detection / redaction suggestions
