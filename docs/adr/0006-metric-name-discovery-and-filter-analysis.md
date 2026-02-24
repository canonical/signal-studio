# ADR-0006: Metric Name Discovery and Filter Analysis

**Status:** Proposed
**Date:** 2026-02-24
**Related:** [ADR-0001: Project Specification](0001-project-specification.md), [ADR-0005: Live Metrics Implementation](0005-live-metrics-implementation.md)

---

## Context

Today Signal Lens understands two things about a Collector: its pipeline structure (from parsed YAML) and its aggregate throughput (from Prometheus self-observability metrics). What it does **not** know is *which* metric names are actually flowing through those pipelines.

This is a significant gap. When a user is considering a `filter` processor to reduce metric volume, the most natural questions are:

- What metric names exist in this pipeline right now?
- Which ones are high-cardinality or high-volume?
- If I add this filter expression, what percentage of my traffic would it match?

Without metric name visibility, our filter-related recommendations remain generic ("add severity filtering", "consider attribute pruning") rather than specific ("metric `http.server.duration` accounts for 40% of your metric points — consider dropping histogram buckets").

ADR-0001 Section 3.2 explicitly excludes OTLP ingestion, telemetry storage, and precise filter simulation from the MVP. Any approach must work within or carefully extend those boundaries.

---

## Problem Breakdown

There are two distinct subproblems:

1. **Metric name discovery** — learning what metric names (and ideally their types and rough volumes) are flowing through a Collector.
2. **Filter analysis** — given a set of known metric names and a proposed filter configuration, predicting what would be kept or dropped.

These can be solved independently. Discovery requires a data source; filter analysis is primarily a parsing and matching problem.

---

## Approaches for Metric Name Discovery

### A. Scrape the Collector's Internal `otelcol_processor_*` Metrics

The Collector's self-observability metrics include processor-level counters with a `processor` label, but **not** per-metric-name breakdowns. These tell us "processor X accepted 5,000 metric points" but not which metric names contributed to that count.

**Verdict:** Insufficient for metric name discovery. Already used by ADR-0005 for throughput analysis.

### B. Scrape the Downstream Backend

If the Collector exports metrics to a Prometheus-compatible backend, we could query that backend's `/api/v1/label/__name__/values` endpoint to discover metric names. For OTLP backends, similar metadata APIs may exist.

**Pros:**
- Read-only — we're querying an existing backend, not intercepting traffic
- Gives the full metric name catalog as the backend sees it
- Can correlate volume per metric name via `count` queries
- Works without any changes to the Collector itself

**Cons:**
- Requires the user to provide backend connection details (separate from the Collector endpoint)
- Only works if the backend exposes a query API (Prometheus, Mimir, Thanos, etc.)
- Shows what arrived at the backend, not what entered the Collector — post-processor view, not pre-processor
- **No attribution**: a backend typically receives metrics from many sources — other Collector instances, direct SDK instrumentation, other monitoring agents, sidecar proxies, etc. There is no reliable way to determine which metric names were contributed by the specific Collector instance being analyzed. This means the discovered metric set is polluted with names that may never pass through the pipeline in question, rendering filter analysis results misleading.
- Cannot attribute metric names to specific Collector pipelines even within the same Collector
- Adds another external dependency to the tool

**Verdict:** The attribution problem is fundamental. Filter analysis based on backend-queried metric names would produce predictions against a superset of metrics, many of which are irrelevant to the Collector under analysis. This makes Option B unsuitable as a primary discovery mechanism for filter analysis, though it could serve as a rough "what exists in my environment" overview.

### C. OTLP Mirror Receiver

Add a lightweight OTLP receiver to the Signal Lens backend. The user configures their Collector to fan out a copy of traffic to Signal Lens via an additional `otlp` exporter in the pipeline. Signal Lens receives the OTLP data, extracts metric metadata (names, types, attribute keys), and discards the actual data points.

```
Collector pipeline:
  receivers: [otlp]
  processors: [batch]
  exporters: [otlp/backend, otlp/signal-lens]   ← fan-out
```

**Pros:**
- Sees actual metric names at wire level — the ground truth
- Can be placed at any point in the pipeline (before or after processors) by changing where the fan-out exporter sits
- Metric names, types, and attribute keys are available from OTLP metadata without storing data points
- The Collector's existing fan-out mechanism handles the copy — no custom Collector components needed
- Can compute per-metric-name volume rates from the stream

**Cons:**
- Requires modifying the Collector config to add the fan-out exporter — no longer fully non-intrusive
- Crosses the ADR-0001 boundary of "will not ingest OTLP data" — this is a deliberate scope extension
- Increases Collector resource usage (CPU, memory, network) proportional to the pipeline's volume
- Signal Lens backend becomes stateful (holding metric name catalog in memory)
- Must handle backpressure — if Signal Lens is slow, the Collector's sending queue for this exporter backs up
- OTLP parsing adds a non-trivial dependency (`go.opentelemetry.io/collector/pdata` or raw protobuf)

### D. Debug Exporter Log Parsing

The Collector's `debug` exporter (formerly `logging`) outputs telemetry details to stdout. Signal Lens could read the Collector's log output (via a log file, Docker logs API, or Kubernetes log stream) and parse metric names from it.

**Pros:**
- No Collector config change beyond enabling/adjusting the debug exporter
- The debug exporter is already present in many development configs

**Cons:**
- Fragile — parsing unstructured or semi-structured log output
- The debug exporter format has changed across Collector versions
- `verbosity: detailed` produces enormous log volume and impacts Collector performance
- Not a realistic approach for production environments
- Would require a log-tailing integration (file, Docker API, Kubernetes API) — significant scope

**Verdict:** Too fragile and impractical for anything beyond local debugging.

### E. Collector Feature Gates / Telemetry Improvements

The OpenTelemetry Collector community has discussed adding per-metric-name telemetry (see `otelcol_exporter_sent_metric_points` with a `metric_name` label). This is not currently available in stable releases due to cardinality concerns, but future versions or feature gates may expose it.

**Pros:**
- Would fit naturally into the existing Prometheus scraping approach (ADR-0005)
- No Collector config changes required
- Read-only

**Cons:**
- Does not exist today in stable Collector releases
- High-cardinality labels on self-observability metrics are explicitly avoided by the Collector project
- Depends on upstream decisions outside our control

**Verdict:** Worth monitoring but cannot be relied upon. If a feature gate appears, the existing scraper in ADR-0005 could consume it with minimal changes.

### F. Periodic OTLP Sampling Tap

A variant of Option C that reduces the resource impact: instead of receiving all traffic, Signal Lens instructs the Collector to send only a periodic sample. This could be achieved by placing a `probabilistic_sampler` processor before the fan-out exporter, or by having Signal Lens accept traffic for short windows (e.g., 10 seconds every 5 minutes) and disconnect in between.

**Pros:**
- Dramatically lower resource overhead than full mirroring
- A short sample window is often sufficient to discover metric names (names change slowly; volume is what fluctuates)
- Signal Lens controls when it listens, limiting its own resource use

**Cons:**
- Same Collector config modification requirement as Option C
- May miss low-frequency metric names that don't appear in the sample window
- Windowed sampling adds protocol complexity (connect/disconnect cycling, or a custom processor)

---

## Approach for Filter Analysis

Independent of how metric names are discovered, filter analysis is a matching problem: given a set of known metric names (and optionally their attribute keys) and a filter processor config, determine which metrics match.

### Filter Processor Config Structure

The Collector's `filter` processor uses OTTL (OpenTelemetry Transformation Language) conditions or the older `include`/`exclude` block syntax:

```yaml
# OTTL style (modern)
filter:
  metrics:
    metric:
      - 'name == "http.server.duration"'
      - 'IsMatch(name, "system\\.cpu\\..*")'

# Include/exclude style (legacy but common)
filter:
  metrics:
    include:
      match_type: regexp
      metric_names:
        - "http\\.server\\..*"
```

### Analysis Levels

**Level 1 — Static name matching (no live data required)**

Parse the filter config's `metric_names` patterns. If metric names are known (from any discovery mechanism), test each name against the patterns and report the match set. This works for both literal and regexp `metric_names`.

This level is useful even without live metric discovery — if the user's Collector exports to a Prometheus backend they can query separately, they could paste a metric name list into Signal Lens.

**Level 2 — Volume-weighted matching (requires per-name volume data)**

If per-metric-name volume data is available (from Option C/F), weight the match results by volume. "This filter would drop 3 out of 20 metric names, representing ~60% of metric points" is far more actionable than "this filter matches 3 metric names".

**Level 3 — OTTL expression evaluation (requires OTTL parser)**

Full OTTL expressions can reference attribute values, resource attributes, and data point fields. Evaluating these requires either an OTTL parser/interpreter or access to actual data points.

Implementing an OTTL evaluator is a significant undertaking. The Collector's own OTTL package (`go.opentelemetry.io/collector/pdata`) is complex. A pragmatic alternative is to support only the `name`-based subset of OTTL expressions (which covers the majority of metric filtering use cases) and flag unsupported expressions as "cannot analyze statically."

---

## Recommendation

The attribution problem (see Option B verdict) rules out backend querying as a reliable foundation for filter analysis — we cannot confidently tell users "this filter would drop these metrics" if we don't know which metrics actually belong to the Collector under analysis. This shifts the recommendation toward approaches that observe traffic at the Collector itself.

### Phase 1: OTLP Sampling Tap + Static Filter Matching

Accept the ADR-0001 scope extension and implement the OTLP sampling tap (Option F) as the primary discovery mechanism. This is the only approach that provides attributed, per-Collector metric name visibility without depending on upstream Collector changes.

- Lightweight OTLP gRPC receiver in the Signal Lens backend
- Metadata extraction only: metric names, types, attribute keys, approximate point counts
- No data point storage — metadata is held in memory with a TTL
- Windowed approach: accept data for configurable intervals (default 30s every 5 minutes) to limit resource impact
- Generate a "Connect to Signal Lens" snippet that the user adds to their Collector config — Signal Lens already generates YAML snippets, so this fits the existing UX pattern
- Clearly document this as opt-in and requiring a Collector config change
- Implement Level 1 filter matching: parse `filter` processor configs from the Collector YAML, test discovered metric names against patterns, and show predicted keep/drop sets

The scope extension is justified because the alternative (backend querying) produces fundamentally unreliable results. A tool that shows "this filter would drop metric X" when metric X doesn't even pass through the Collector being analyzed is worse than no analysis at all.

### Phase 2: Volume-Weighted Filter Analysis

With per-metric-name volume data from the sampling tap, enhance filter analysis to show volume impact.

- "This filter would drop N metric names representing ~X% of traffic"
- Rank metric names by volume to help users identify the highest-impact filtering targets
- Visualize the before/after volume split on the pipeline cards

### Phase 3: Pre/Post Processor Comparison (Optional)

Allow users to place the fan-out exporter at different points in their pipeline to compare what enters vs. what exits a processor chain. This enables "before/after" analysis of existing filters and transforms.

This requires the user to add two fan-out exporters (one early, one late) — the UX should make this easy to set up but clearly explain the performance implications.

---

## Frontend Visualization

### Metric Name Catalog Panel

A new panel or drawer showing discovered metric names, sortable by:
- Name (alphabetical)
- Volume (highest first)
- Type (gauge, counter, histogram, summary)

Each entry shows: name, type, approximate rate (points/sec), and a match indicator if a filter processor is configured (green = kept, red = dropped).

### Filter Impact Overlay

When a pipeline contains a `filter` processor and metric names are available:
- The filter processor card on the pipeline graph shows a "N kept / M dropped" summary
- Clicking it opens a breakdown: which specific metric names are matched by each filter rule
- A "What if" mode allows editing filter patterns and seeing the predicted impact in real time against the known metric name set

---

## Open Questions

1. **Collector config friction** — The OTLP tap requires modifying the Collector config. How do we minimize the barrier? Auto-generating a ready-to-paste snippet helps, but some organizations have strict change control around Collector configs. Is there a way to make this zero-config for common deployment patterns (e.g., Helm chart annotations)?
2. **Attribute-level analysis** — Metric name filtering covers the most common case, but attribute-based filtering (`where attributes["service.name"] == "foo"`) requires attribute key discovery. The OTLP tap can extract attribute keys — should attribute-level filter matching be included in Phase 1 or deferred?
3. **OTTL support boundary** — What subset of OTTL expressions should filter matching support? `name ==` and `IsMatch(name, ...)` cover most cases, but users may expect broader support.
4. **Resource overhead transparency** — How should we communicate the performance cost of the sampling tap to users? Should Signal Lens estimate the additional load based on observed throughput?
5. **gRPC dependency** — Adding an OTLP gRPC receiver pulls in `google.golang.org/grpc` and related protobuf dependencies. What is the binary size and build time impact?

---

## Consequences

### Positive

- Users gain visibility into what's actually flowing through their pipelines, not just aggregate throughput
- Metric names are attributed to the specific Collector instance, so filter analysis predictions are reliable
- Filter analysis transforms recommendations from generic advice to specific, actionable predictions
- The sampling tap approach scales down gracefully — short observation windows keep overhead low
- Phased approach allows shipping value incrementally

### Negative

- Crosses the ADR-0001 scope boundary by ingesting OTLP data, even if only metadata — this is the most significant philosophical change since the project's inception
- Requires a Collector config change (adding a fan-out exporter), making the tool no longer fully non-intrusive
- Adds gRPC and protobuf dependencies, increasing binary size and build complexity
- Signal Lens backend becomes stateful (holding an in-memory metric name catalog), adding lifecycle concerns
- Must handle backpressure correctly — if Signal Lens is slow or unavailable, the Collector's sending queue for the tap exporter must not impact production pipelines
- OTTL expression evaluation (even partial) is a complex parsing problem that may produce incorrect predictions for edge cases
- Windowed sampling may miss low-frequency metric names that don't appear during the observation window
