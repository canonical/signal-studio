# ADR-0008: Quality Improvements Roadmap

**Status:** Proposed
**Date:** 2026-02-24
**Related:** All prior ADRs

---

## Context

After a full-project review spanning the backend (Go), frontend (React/TypeScript), Docker build, documentation, and infrastructure, several improvement areas have been identified. The project has strong foundations — clear ADRs, good backend coverage (86-100% on core packages), clean architecture — but gaps exist in CI/CD, testing breadth, security hardening, and frontend quality.

This ADR catalogues improvements with honest trade-off analysis so the team can prioritize deliberately rather than reactively.

---

## Current State

### Strengths

- **Backend test coverage is strong**: config 100%, rules 96%, metrics 96.4%, filter 91.4%, tap 86.3%
- **ADR-driven development**: 7 ADRs with thorough alternatives analysis and trade-off discussion
- **Clean architecture**: Manager pattern, interface-based rule dispatch, clear package boundaries
- **Sound tech choices**: Go stdlib router, React 19, Vanilla Framework, multi-stage Docker, static binary
- **Read-only, stateless design**: Minimizes risk surface and operational complexity

### Weaknesses

- **No CI/CD pipeline**: No automated testing, linting, or security scanning
- **No frontend tests**: Zero test files exist in `frontend/src/`
- **API handler package at 0% coverage**: `internal/api/` is untested
- **Docker runs as root**: No non-root user configured
- **No linting enforcement**: No golangci-lint, no ESLint, no pre-commit hooks
- **Unused dependency**: `@xyflow/react` (4.4 MB) is installed but never imported

---

## Proposed Improvements

### 1. CI/CD Pipeline (GitHub Actions)

**What:** Add `.github/workflows/ci.yml` that runs on every push/PR:
- Backend: `go vet`, `go test ./... -race -cover`, coverage gate at 80%
- Frontend: `tsc --noEmit`, `npm run build`
- Docker: build image, scan with Trivy

**Pros:**
- Catches regressions before merge
- Enforces coverage requirement from CLAUDE.md automatically
- Prevents shipping broken builds

**Cons:**
- Adds CI minutes cost (mitigated by caching Go modules and node_modules)
- Requires maintaining workflow files as toolchain versions change

**Trade-offs:**
- Comprehensive CI slows PRs by ~2-3 min vs. fast feedback on breakage. Worth it given the project's rule count is growing.

---

### 2. Docker Security Hardening

**What:**
- Add non-root user: `RUN adduser -D -u 1001 appuser` + `USER appuser`
- Add `HEALTHCHECK CMD wget -qO- http://localhost:8080/api/health || exit 1`
- Pin Alpine package versions

**Pros:**
- Non-root eliminates container-escape privilege escalation
- Health check enables orchestrator liveness detection
- Package pinning prevents supply-chain drift

**Cons:**
- Non-root may complicate bind-mounting volumes if file ownership matters
- Health check adds a small recurring CPU cost (every 30s)

**Trade-offs:**
- Security hardening is essentially free for a stateless binary that doesn't write to disk. No downside worth skipping this for.

---

### 3. API Handler Tests

**What:** Add integration tests for all endpoints in `internal/api/` using `httptest.NewServer`.

Test matrix:
- `POST /api/analyze` — valid YAML, invalid YAML, oversized body, with/without tap data
- `POST /api/metrics/connect` — valid URL, empty URL, already connected
- `POST /api/metrics/disconnect`, `GET /api/metrics/status`, `GET /api/metrics/snapshot`
- `POST /api/tap/start`, `POST /api/tap/stop`, `GET /api/tap/status`, `GET /api/tap/catalog`
- `GET /api/health`

**Pros:**
- Brings API package from 0% to >80% coverage
- Catches serialization bugs, missing error paths, CORS issues
- Tests the integration seam between handlers and managers

**Cons:**
- Requires mocking or test doubles for `metrics.Manager` and `tap.Manager`
- Tests are slower than pure unit tests (~100ms each with HTTP round-trip)

**Trade-offs:**
- Could use interfaces for managers to enable mocking, but that adds abstraction. Alternatively, use real managers with in-memory backends. Real managers are simpler but couple tests to manager implementation.

---

### 4. Frontend Test Infrastructure

**What:** Add Vitest + React Testing Library. Target hooks and data-transformation logic first, then components.

Priority test targets:
1. `useDebounce` — pure logic, easy to test
2. `useMetrics` / `useTap` — mock fetch, verify polling lifecycle and cleanup
3. `FindingsPanel` — verify sort order, expansion, copy behavior
4. `MetricCatalogPanel` — verify search filtering, sort toggle, outcome display
5. `ConfigInput` — verify YAML paste reformatting

**Pros:**
- Catches regressions in sorting, filtering, polling logic
- Vitest integrates natively with Vite (near-zero config)
- React Testing Library encourages testing user-visible behavior

**Cons:**
- Test setup for Monaco Editor is non-trivial (needs jsdom mocking)
- Maintaining UI tests has a cost as components evolve

**Trade-offs:**
- Testing hooks and logic first gives the best coverage-per-effort ratio. Full component rendering tests can follow later. Skip snapshot testing — it generates noise and breaks on every CSS change.

---

### 5. Linting and Formatting

**What:**
- Backend: Add `.golangci.yml` with `errcheck`, `govet`, `staticcheck`, `unused`, `gosimple`
- Frontend: Add ESLint with `@typescript-eslint` and `eslint-plugin-react-hooks`
- Add `.pre-commit-config.yaml` or a `just lint` recipe to justfile

**Pros:**
- Catches dead code, unchecked errors, hook rule violations automatically
- Enforces consistent style without code review overhead
- `errcheck` would have caught the unused `matchedBy` variable in `filter/matcher.go:116`

**Cons:**
- Initial lint pass may surface many warnings requiring a cleanup commit
- Golangci-lint adds ~5s to CI

**Trade-offs:**
- Start with a small lint set and expand. Running `golangci-lint` with `--new-from-rev=main` limits noise to new code only.

---

### 6. Remove Unused `@xyflow/react` Dependency

**What:** `npm uninstall @xyflow/react` in frontend/.

**Pros:**
- Removes ~4.4 MB from node_modules
- Eliminates transitive dependency risk
- Reduces install time

**Cons:**
- None. It is unused.

**Trade-offs:**
- If pipeline visualization was planned, it can be re-added when needed. YAGNI applies.

---

### 7. Frontend Accessibility Fixes

**What:**
- Add `aria-label` to all icon-only buttons (MetricsConnection, TapConnection, PipelineGraph)
- Add `<caption>` to MetricCatalogPanel table
- Use `<details>/<summary>` or proper ARIA expanded state for FindingsPanel items
- Add visible focus outlines in CSS
- Verify color contrast meets WCAG AA (especially `#666`, `#999` grays)

**Pros:**
- Screen reader users can operate the tool
- Keyboard navigation works correctly
- Meets baseline accessibility compliance

**Cons:**
- Focus outlines and ARIA attributes add visual and code complexity
- Contrast fixes may require design adjustment

**Trade-offs:**
- Start with the mechanical fixes (aria-labels, focus outlines). Color contrast changes may need design review if they alter the visual aesthetic.

---

### 8. Structured Logging

**What:** Replace `log.Printf` calls in metrics scraper and tap receiver with `log/slog` (stdlib since Go 1.21). Use JSON output by default.

**Pros:**
- Structured logs are queryable in production
- `slog` is zero-dependency (stdlib)
- Level filtering lets users silence noisy messages

**Cons:**
- Requires touching several files
- JSON logs are harder to read during development (mitigate with text handler in dev)

**Trade-offs:**
- `log/slog` is the right choice over third-party loggers (zerolog, zap) because it's stdlib and the project doesn't need their extra performance. The project's log volume is low enough that slog's performance is more than adequate.

---

### 9. Graceful Shutdown

**What:** Add `signal.NotifyContext` for SIGINT/SIGTERM in `main.go`. Shut down HTTP server, metrics scraper, and tap receiver in order.

**Pros:**
- In-flight requests complete instead of being killed
- Tap gRPC/HTTP listeners close cleanly
- Standard behavior expected by orchestrators (Kubernetes, Docker)

**Cons:**
- Adds ~20 lines to main.go
- Requires propagating context through managers (minor refactor)

**Trade-offs:**
- For a stateless tool, ungraceful shutdown doesn't lose data. But clean shutdown avoids port-in-use errors on rapid restarts and is expected by container orchestrators. Worth the small effort.

---

### 10. Configuration Externalization

**What:** Move hardcoded values to environment variables or a config struct:

| Current location | Value | Suggested env var |
|---|---|---|
| `scraper.go:45` | HTTP timeout 5s | `SCRAPE_TIMEOUT_SECONDS` |
| `receiver.go:98` | OTLP body limit 10MB | `TAP_MAX_BODY_MB` |
| `manager.go:18` | Catalog TTL 5min | `TAP_CATALOG_TTL_SECONDS` |
| `store.go:8` | Window size 6 | `METRICS_WINDOW_SIZE` |

**Pros:**
- Operators can tune for their environment
- Avoids recompilation for deployment-specific tuning

**Cons:**
- More env vars to document and validate
- Each new knob is a support surface

**Trade-offs:**
- Externalize only what operators actually need to tune. The catalog TTL and body limit are good candidates. Internal window sizes and prune intervals are implementation details that should stay hardcoded unless a real need arises.

---

### 11. ADR Status Reconciliation

**What:** Update ADR statuses to reflect implementation reality:

| ADR | Current Status | Suggested Status |
|---|---|---|
| 0001 | Proposed | Accepted |
| 0003 | Proposed | Accepted (Tiers 1-2 implemented) |
| 0004 | Proposed | Deferred (hybrid approach not yet implemented) |
| 0005 | Proposed | Accepted (live metrics implemented) |
| 0006 | Proposed | Accepted (Phase 1 implemented) |

**Pros:**
- Reduces confusion about what's implemented vs. aspirational
- New contributors can trust ADR status

**Cons:**
- Minor maintenance effort

**Trade-offs:**
- None. This is pure documentation hygiene.

---

### 12. FindingsPanel Sort Memoization and Key Stability

**What:**
- Wrap `[...findings].sort()` in `useMemo` to avoid re-sorting on every render
- Replace `key={f.ruleId}-${f.pipeline}-${i}` with a stable key (drop the index)

**Pros:**
- Avoids unnecessary array allocation and sort on each render cycle
- Stable keys prevent React from re-mounting items when the list reorders

**Cons:**
- Negligible code change

**Trade-offs:**
- If two findings can share the same ruleId+pipeline (e.g. two `missing-memory-limiter` findings for different pipelines... but pipeline already differentiates), the key is already unique. Adding the evidence hash would guarantee uniqueness if needed.

---

### 13. `formatRate()` Bug Fix

**What:** In `PipelineGraph.tsx:112-115`, the function has two identical branches:
```typescript
if (rate >= 10000) return `${(rate / 1000).toFixed(1)}k`;
if (rate >= 1000) return `${(rate / 1000).toFixed(1)}k`;
```
The first branch is redundant (any value >= 10000 is also >= 1000). The intent was likely to use different decimal precision — e.g., `toFixed(0)` for large values and `toFixed(1)` for smaller thousands.

---

## Priority Matrix

| # | Improvement | Effort | Impact | Priority |
|---|---|---|---|---|
| 1 | CI/CD pipeline | Medium | High | P0 |
| 2 | Docker security | Small | High | P0 |
| 6 | Remove @xyflow/react | Trivial | Low | P0 |
| 11 | ADR status reconciliation | Small | Medium | P0 |
| 3 | API handler tests | Medium | High | P1 |
| 5 | Linting and formatting | Medium | Medium | P1 |
| 9 | Graceful shutdown | Small | Medium | P1 |
| 13 | formatRate() bug fix | Trivial | Low | P1 |
| 12 | FindingsPanel memoization | Trivial | Low | P1 |
| 4 | Frontend test infrastructure | Large | High | P2 |
| 7 | Accessibility fixes | Medium | Medium | P2 |
| 8 | Structured logging | Medium | Medium | P2 |
| 10 | Configuration externalization | Medium | Low | P3 |

---

## Decision

Propose all 13 improvements for team review. Implementation order follows the priority column. P0 items are low-effort, high-return and should be done immediately. P1 items should follow within the next development cycle. P2/P3 items are tracked but not blocking.

---

## Consequences

**Positive:**
- Project moves from "well-designed prototype" to "production-grade tool"
- CI/CD prevents regressions as the rule set grows
- Security hardening meets container deployment baseline
- Accessibility improvements expand user base

**Negative:**
- CI/CD and linting infrastructure require ongoing maintenance
- Frontend tests slow the feedback loop slightly
- More env vars increase operator cognitive load

**Risk:**
- Large lint cleanup commit may introduce churn. Mitigate by landing it as a single atomic commit with no behavior changes.
