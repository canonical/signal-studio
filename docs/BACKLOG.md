# Backlog

Open research questions, feature ideas, and deferred work. When an item here matures into a concrete design choice, it graduates to an ADR.

## Research

- **Cardinality estimation** — HyperLogLog / Top-K for attribute cardinality without storing all values. Would improve confidence in high-cardinality findings. (ADR-0001)
- **Span name cardinality** — unbounded span names can blow up the catalog. Evaluate capping, normalization, grouping, or stricter TTL. (ADR-0015)
- **Attribute value co-occurrence** — tracking which attribute values appear together could enable smarter filter suggestions. (ADR-0009)
- **Cross-signal correlation** — linking metrics, traces, and logs via shared resource attributes. Useful but unclear ROI for the diagnostic use case. (ADR-0013)
- **expr-lang for user-defined rules** — hybrid approach where users can write custom rules in a safe expression language. (ADR-0004)
- **SSE for live metrics** — Server-Sent Events instead of polling. Simpler client code, but polling works fine for current scale. (ADR-0005)

## Features

Ordered by estimated value — combination of user impact, differentiation, and unlock potential.

1. **Inline filter expression editor** — let users edit filter expressions and see predicted impact in real-time. Turns the tool from diagnostic to interactive. (ADR-0009)
2. **Log filter analysis** — blocked on resolving log catalog keying strategy. Completes multi-signal coverage. (ADR-0013, ADR-0014)
3. **Attribute-level trace filter analysis** — beyond span name matching, evaluate attribute-based OTTL expressions against discovered span attributes. (ADR-0013)
4. **Shareable diagnostic reports** — export findings as a standalone HTML or PDF for team review. Low effort now that CLI mode exists.
5. **Juju Doctor integration** — expose diagnostic capabilities as a provider for `juju doctor`. Requires CLI mode (done).
6. **Historical trend view** — requires optional persistence (SQLite or similar). High value but changes the deployment story. (ADR-0005)
7. **GitOps PR generation** — generate a PR with recommended config changes. Significant scope, but natural extension of CLI mode. (ADR-0001)
8. **Distribution-aware validation** — validate component names and config keys against specific collector distributions (core, contrib, AWS) and versions. Catches typos and invalid references that static rules can't.
9. **PII detection / redaction suggestions** — flag attributes that look like PII and suggest redaction processors. Niche but valuable for regulated environments. (ADR-0001)
10. **Metrics endpoint to collect analysis results** - to be explored.
11. **Signal catalog separated by pipeline** - Instead of separating the catalog by signal, we should separate it by pipeline, allowing users to forward multiple pipelines to the tap. Somehow we need to understand what comes in from which pipeline, which will require some research.

## Deferred from ADRs

Items explicitly scoped out of MVP implementations. Add when there's demand.

- **JUnit XML output formatter** — deferred from CLI mode (ADR-0017). The "rules as test cases" metaphor is a poor fit for a linter. Exit codes handle CI pass/fail, SARIF handles inline annotations. Add if Jenkins/GitLab users request it.
- **Multiple input files for CLI** — deferred from CLI mode (ADR-0017). Useful for fleet audits but adds complexity around per-file vs. aggregate results and output grouping.
- **Recording rule dependency graph** — deferred from alert rule coverage Phase 1 (ADR-0018). Resolves recording rule chains to their source metrics for transitive filter impact analysis. High complexity (DAG construction, cycle detection, multi-level resolution) relative to usage — most alerts reference raw metrics. Until implemented, alerts referencing recording rule outputs report `unknown`.
- **Grafana unified alerting API** — deferred from alert rule coverage Phase 1 (ADR-0018). `GET /api/v1/provisioning/alert-rules` returns Grafana-managed rules. Different response format, requires API key, multi-step evaluation pipeline adds parsing complexity. Add if Grafana-heavy environments request it.

## Housekeeping

> **True single binary** - embed the frontend into the go binary and make it serve it.

- **CI/CD pipeline** — automated test runs, linting, Docker builds. (ADR-0008)
- **Docker security hardening** — non-root user, minimal base image, read-only filesystem. (ADR-0008)
  > **Remove unused `@xyflow/react` dependency** (ADR-0008)
  > **API handler tests** — backend HTTP handler coverage is thin. (ADR-0008)
  > **Frontend unit tests** — no frontend tests exist yet. Component and hook coverage. (ADR-0008)
- **Browser / E2E tests** — automated browser testing for critical user flows (paste YAML, connect metrics, tap setup, findings interaction).
  > **Graceful shutdown** — clean up tap listener and metrics poller on SIGTERM. (ADR-0008)
- **Structured logging** — replace fmt prints with slog. (ADR-0008)
- **Load generation tool** — standalone utility that sends synthetic OTLP telemetry (metrics, traces, logs) to any OTLP receiver — a live collector or Signal Studio's own tap endpoint. Useful for exercising pipelines during development, demos, and testing filter/tap/catalog behavior without real instrumented services.
- **Security audit** — review attack surface of OTLP tap endpoints, input validation on YAML parsing, and dependency supply chain.
- **Accessibility audit** — keyboard navigation, screen reader support. (ADR-0008)
