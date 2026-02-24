# ADR-0003: Additional Static Linting Rules

**Status:** Proposed
**Date:** 2026-02-23

## Context

ADR-0001 defined 6 static rules for the MVP. All 6 are now implemented and tested. This ADR evaluates additional rules that can be detected through static analysis of the Collector YAML configuration, grouped by category with pros, cons, and tradeoffs for each.

The existing parser already resolves component configs into `map[string]any`, meaning deep inspection of component-level configuration values is possible without parser changes.

## Proposed Rules

### Category 1: Processor Ordering

#### R11 — memory_limiter not first processor

| | |
|---|---|
| **Severity** | Critical |
| **Detect** | `memory_limiter` exists in a pipeline but is not at index 0 of the processors list. |
| **Why** | The memory_limiter must be first so it can apply backpressure to receivers before any other processor buffers data. If a batch processor comes first, data accumulates before the limiter can refuse it, leading to OOM crashes. |
| **Pros** | Very common misconfiguration. Low implementation complexity. High impact — prevents OOM in production. |
| **Cons** | None significant. This is a well-documented hard requirement. |
| **Tradeoff** | None. Universally recommended by the official docs. |

#### R12 — batch processor before sampling

| | |
|---|---|
| **Severity** | Critical |
| **Detect** | A `batch` processor appears before `tail_sampling` or `probabilistic_sampler` in the processor list. |
| **Why** | The batch processor can split spans from the same trace into different batches. When this happens before tail_sampling, the sampler sees incomplete traces and makes incorrect decisions, producing fragmented or lost traces. |
| **Pros** | Prevents subtle data quality issues that are hard to debug. Low complexity (index comparison). |
| **Cons** | Only relevant to trace pipelines using sampling. |
| **Tradeoff** | Narrow applicability, but critical when it applies. Worth implementing because the failure mode is silent. |

#### R13 — batch processor not near end of pipeline

| | |
|---|---|
| **Severity** | Info |
| **Detect** | The `batch` processor is not the last (or second-to-last) processor in the pipeline. |
| **Why** | Batching should occur as late as possible to optimize compression and connection efficiency. Processors after batch operate on individual items, negating the batching benefit. |
| **Pros** | Encourages optimal pipeline design. |
| **Cons** | Legitimate exceptions exist (e.g., a resource detection processor after batch). May produce false positives. |
| **Tradeoff** | Info severity keeps it advisory. Consider allowing a configurable allowlist of "post-batch" processors. |

### Category 2: Security

#### R14 — Receiver endpoint bound to 0.0.0.0

| | |
|---|---|
| **Severity** | Warning |
| **Detect** | Any receiver's `endpoint` field contains `0.0.0.0`. |
| **Why** | Exposes the receiver to all network interfaces, making it accessible from untrusted networks. As of Collector v0.110.0 the default was changed to `localhost` for this reason. |
| **Pros** | Catches a common security oversight. Easy to implement (string match on config). |
| **Cons** | In containerized environments, binding to `0.0.0.0` is often intentional and necessary (e.g., Kubernetes pods). |
| **Tradeoff** | May produce false positives in container deployments. Warning severity lets users acknowledge and move on. Could add an ignore mechanism later. |

#### R15 — Debug exporter in pipeline

| | |
|---|---|
| **Severity** | Warning |
| **Detect** | A `debug` or `logging` exporter is referenced in any pipeline. |
| **Why** | Prints all telemetry to stdout. In production this causes performance overhead, disk bloat, and can expose sensitive data (PII, auth tokens) in log files. |
| **Pros** | Very easy to detect. Catches a common "forgot to remove after debugging" mistake. |
| **Cons** | Legitimate in development and staging environments. |
| **Tradeoff** | Warning severity is appropriate — flags it without blocking. Users in dev can safely ignore. |

#### R16 — Hardcoded credentials

| | |
|---|---|
| **Severity** | Critical |
| **Detect** | Config values for keys matching `password`, `api_key`, `token`, `secret`, `auth` that don't use `${env:...}` or `${...}` expansion syntax. |
| **Why** | Hardcoded credentials leak through version control, logs, and error messages. The Collector supports env var expansion specifically for this. |
| **Pros** | High security impact. Prevents credential leakage. |
| **Cons** | Heuristic-based — may produce false positives for keys that happen to match patterns but aren't secrets. The tool also receives raw YAML, so env vars may already be expanded by the user's deployment tooling. |
| **Tradeoff** | Risk of false positives from pattern matching. Could mitigate by checking the raw YAML string for `${env:` patterns rather than parsed values. Medium implementation complexity. |

#### R17 — Missing TLS on exporters

| | |
|---|---|
| **Severity** | Warning |
| **Detect** | An `otlp` or `otlphttp` exporter has no `tls` configuration and the endpoint is not localhost. |
| **Why** | Without TLS, telemetry data (potentially containing PII and auth tokens) is sent in plaintext. |
| **Pros** | Important security check for production deployments. |
| **Cons** | Requires parsing the endpoint to determine if it's local. Some environments use mTLS at the infrastructure level (e.g., service mesh), making this a false positive. |
| **Tradeoff** | Useful but prone to false positives in environments with infrastructure-level encryption. Warning severity is appropriate. |

#### R18 — pprof extension enabled

| | |
|---|---|
| **Severity** | Info |
| **Detect** | The `pprof` extension is present in the service extensions list. |
| **Why** | Exposes Go runtime profiling data. Can be used to gather operational intelligence or degrade performance. |
| **Pros** | Trivial to detect. Flags a production anti-pattern. |
| **Cons** | Useful and intentional during performance troubleshooting. |
| **Tradeoff** | Info severity keeps it as a gentle nudge. Low effort, low controversy. |

### Category 3: Reliability

#### R19 — memory_limiter without limit_mib or limit_percentage

| | |
|---|---|
| **Severity** | Critical |
| **Detect** | A `memory_limiter` processor config has neither `limit_mib` nor `limit_percentage` set. |
| **Why** | Without a limit, the processor does nothing. Creates a false sense of security — the pipeline appears protected but is not. |
| **Pros** | Catches a config that is effectively a no-op. Critical safety check. |
| **Cons** | None significant. |
| **Tradeoff** | None. This is always a misconfiguration. |

#### R20 — Exporter sending_queue not enabled

| | |
|---|---|
| **Severity** | Warning |
| **Detect** | An exporter has no `sending_queue` section or `sending_queue.enabled` is not `true`. |
| **Why** | Without a sending queue, transient backend unavailability causes immediate data loss. The queue buffers data during failures. |
| **Pros** | Important resilience check. Prevents data loss during transient failures. |
| **Cons** | Some exporters don't support sending queues. Queue consumes memory — in very constrained environments, disabling it is intentional. Not all exporter types support this configuration. |
| **Tradeoff** | Should only flag exporters known to support queues (otlp, otlphttp, etc). Warning severity lets users make an informed choice. |

#### R21 — Exporter retry_on_failure not enabled

| | |
|---|---|
| **Severity** | Warning |
| **Detect** | An exporter has no `retry_on_failure` section or `retry_on_failure.enabled` is not `true`. |
| **Why** | Without retry, any transient error causes permanent data loss for the affected batch. |
| **Pros** | Prevents silent data loss on network blips. |
| **Cons** | Same as R20 — not all exporters support this. Retries increase latency under sustained failure. |
| **Tradeoff** | Same scoping concern as R20. Should only apply to network-based exporters. |

### Category 4: Pipeline Wiring

#### R22 — Pipeline references undefined component

| | |
|---|---|
| **Severity** | Critical |
| **Detect** | A pipeline lists a receiver, processor, or exporter that does not exist in the corresponding top-level section. |
| **Why** | The collector will fail to start. Catching this in the linter gives faster feedback with clearer error messages than a startup crash. |
| **Pros** | Prevents broken configs from reaching deployment. Simple cross-reference check. |
| **Cons** | The collector itself already validates this on startup. |
| **Tradeoff** | Provides value as shift-left validation. Especially useful in CI pipelines or pre-commit checks where the collector binary may not be available. |

#### R23 — Empty pipeline

| | |
|---|---|
| **Severity** | Critical |
| **Detect** | A pipeline has zero receivers or zero exporters. |
| **Why** | No receivers = no data in. No exporters = data processed and silently dropped. Both are almost certainly mistakes. |
| **Pros** | Trivial to implement. Catches obvious misconfigurations. |
| **Cons** | None. A pipeline without receivers or exporters has no valid use case. |
| **Tradeoff** | None. Always worth flagging. |

### Category 5: Filter/Transform Safety

#### R24 — filter/transform processor with error_mode propagate

| | |
|---|---|
| **Severity** | Warning |
| **Detect** | A `filter` or `transform` processor does not set `error_mode` or sets it to `propagate`. |
| **Why** | With propagate (the default), any OTTL condition evaluation error drops the entire payload. A single typo in a filter condition can cause 100% data loss. Setting `error_mode: ignore` logs the error and continues processing. |
| **Pros** | Prevents silent total data loss from filter/transform typos. |
| **Cons** | `propagate` is the upstream default, so this flags the default behavior. Some users may intentionally want strict error handling. |
| **Tradeoff** | Flags a reasonable upstream default as potentially dangerous. Warning severity lets users opt in to the risk consciously. |

### Category 6: Observability Hygiene

#### R25 — No attribute redaction in pipeline

| | |
|---|---|
| **Severity** | Info |
| **Detect** | A pipeline has no processor capable of attribute redaction (`attributes`, `redaction`, `transform`, or `filter` processor). |
| **Why** | Telemetry frequently contains PII (HTTP headers, user emails, IPs). Without explicit redaction, sensitive data flows to all backends. |
| **Pros** | Raises awareness of data privacy requirements. |
| **Cons** | High false-positive rate — many pipelines legitimately don't need redaction (e.g., infrastructure metrics). Not all data contains PII. |
| **Tradeoff** | Very noisy for common configurations. Info severity mitigates this, but it may still annoy users. Consider making this opt-in or only flagging trace/log pipelines. |

#### R26 — OTLP receiver with only one protocol

| | |
|---|---|
| **Severity** | Info |
| **Detect** | An `otlp` receiver has only `grpc` or only `http` configured under `protocols`. |
| **Why** | Many SDKs default to HTTP (especially browser-based) while others use gRPC. Missing one can cause silent data loss from SDKs using the unconfigured protocol. |
| **Pros** | Catches a common "data not arriving" root cause. |
| **Cons** | Many deployments intentionally use only one protocol. |
| **Tradeoff** | Informational only. Useful as a "did you consider this?" prompt. |

## Recommended Implementation Priority

Based on impact-to-effort ratio:

**Tier 1 — High impact, low complexity (implement first):**
- R11 (memory_limiter ordering) — prevents OOM
- R22 (undefined component reference) — prevents startup failure
- R23 (empty pipeline) — prevents silent data loss
- R19 (memory_limiter without limits) — prevents false safety
- R12 (batch before sampling) — prevents fragmented traces

**Tier 2 — Moderate impact, low complexity:**
- R14 (0.0.0.0 endpoint) — security
- R15 (debug exporter) — production hygiene
- R20 (no sending_queue) — resilience
- R21 (no retry_on_failure) — resilience
- R24 (filter error_mode) — prevents silent data loss

**Tier 3 — Lower impact or higher false-positive risk:**
- R13 (batch not near end) — optimization
- R16 (hardcoded credentials) — medium complexity, heuristic-based
- R17 (missing TLS) — false positives in service mesh environments
- R18 (pprof enabled) — low impact
- R25 (no redaction) — high false-positive rate
- R26 (single OTLP protocol) — informational

## Decision

Pending team discussion. This ADR presents the full candidate set for review. The recommendation is to implement Tier 1 rules first as they address critical safety and correctness issues with minimal false-positive risk.

## Consequences

- Each new rule requires tests covering both positive (issue present) and negative (issue absent) cases.
- Rules that inspect component config values (R16, R17, R19, R20, R21, R24) require the parser to preserve nested config maps — this is already supported.
- False positives are a significant UX risk. Rules with known false-positive scenarios should clearly explain when the finding can be safely ignored.
- Backend code coverage must remain above 80% per CLAUDE.md.
