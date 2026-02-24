# ADR-0002: Tech Stack and Initial Architecture

**Status:** Accepted
**Date:** 2026-02-23
**Decision Makers:** Project Founder
**Related:** [ADR-0001: Project Specification](0001-project-specification.md)

---

## 1. Context

ADR-0001 defines a read-only diagnostic tool for OpenTelemetry Collector with a single-container deployment consisting of a backend service and an SPA frontend. This ADR captures the technology choices and structural decisions made to implement that specification.

---

## 2. Decisions

### 2.1 Backend: Go

The backend is written in Go (1.23+).

**Rationale:**
- The OpenTelemetry Collector itself is written in Go, allowing future direct integration with its config structs and parsing packages.
- Single static binary output aligns with the single-container deployment model.
- Strong standard library for HTTP servers, JSON handling, and concurrency (relevant for the Prometheus scraper).

### 2.2 Frontend: React + TypeScript + Vanilla Framework + Vite

The frontend is a single-page application using:
- **React 19** for UI components
- **TypeScript 5.7** for type safety
- **Vanilla Framework** (Canonical's CSS framework) for styling, compiled via Sass
- **Vite 6** as the build tool and dev server
- **React Flow** (`@xyflow/react`) for pipeline graph visualization

**Rationale:**
- React has a mature ecosystem for graph/DAG visualization (needed for pipeline rendering).
- TypeScript provides type safety mirroring the backend models.
- Vanilla Framework provides a consistent, well-documented component library with built-in dark theme support (`is-dark`), application layout primitives (`l-application`, `p-panel`), and standard patterns for notifications, code snippets, and forms.
- Vite provides fast builds and a dev proxy to the backend, simplifying local development.

### 2.3 YAML Parsing: Lightweight Custom Parser

Rather than importing the full `go.opentelemetry.io/collector` config packages, we parse collector YAML using `gopkg.in/yaml.v3` into a lightweight custom model.

**Rationale:**
- The collector's config packages carry a large dependency tree and are tightly coupled to collector internals.
- Our model only needs to represent the structure relevant to diagnostic analysis: component names/types and pipeline wiring.
- This can be revisited if deeper config validation is needed in the future.

### 2.4 Project Structure

```
otel-signal-lens/
├── backend/
│   ├── cmd/server/         # HTTP server entrypoint
│   └── internal/
│       ├── config/         # YAML parser + data model
│       ├── rules/          # Rule engine + static rules
│       ├── metrics/        # Prometheus scraper (future)
│       └── api/            # HTTP handlers + routing
├── frontend/
│   └── src/
│       ├── components/     # React components
│       └── types/          # TypeScript types mirroring backend
├── Dockerfile              # Multi-stage build
└── docs/adr/
```

**Rationale:**
- `internal/` prevents external import of implementation packages.
- Separation of `config`, `rules`, `metrics`, and `api` keeps concerns isolated and independently testable.
- Frontend types in `types/api.ts` mirror the backend JSON contract.

### 2.5 API Design

Single analysis endpoint:
- `POST /api/config/analyze` — accepts raw YAML body, returns parsed config + findings
- `GET /api/health` — health check

**Rationale:**
- Minimal surface area for the MVP. Config parsing and rule evaluation happen in a single request.
- The response includes both the parsed config (for pipeline visualization) and findings (for recommendations), avoiding multiple round-trips.

### 2.6 Rule Engine Architecture

Rules implement a common interface:

```go
type Rule interface {
    ID() string
    Evaluate(cfg *config.CollectorConfig) []Finding
}
```

An `Engine` aggregates rules and runs them against a config. `NewDefaultEngine()` registers all built-in rules.

**Rationale:**
- Simple interface makes adding new rules straightforward.
- Engine can later support both static (config-only) and live (config + metrics) rules via the same interface.
- No rule ordering or dependencies needed for static analysis.

### 2.7 Deployment: Multi-Stage Docker Build

The Dockerfile uses three stages:
1. Node/Alpine — builds frontend static assets
2. Go/Alpine — compiles backend binary
3. Alpine — copies binary + static assets into a minimal runtime image

**Rationale:**
- Produces a small final image with no build tooling.
- Frontend assets are served by the Go binary (static file serving to be wired when needed).
- Matches ADR-0001's single-container requirement.

---

## 3. Static Rules Implemented

All six static rules from ADR-0001 are implemented and tested:

| Rule | ID | Severity | Signal |
|------|----|----------|--------|
| Missing `memory_limiter` | `missing-memory-limiter` | critical | all |
| Missing `batch` processor | `missing-batch` | warning | all |
| No trace sampling | `no-trace-sampling` | warning | traces |
| Unused components | `unused-components` | info | all |
| Multiple exporters without routing | `multiple-exporters-no-routing` | info | all |
| No log severity filtering | `no-log-severity-filter` | info | logs |

---

## 4. What Is Not Yet Implemented

- Prometheus metrics scraper (ADR-0001 Section 3.1B)
- Live rules 4, 5, 6, 7 (require metrics data)
- Static file serving from Go binary (frontend served by Vite proxy in dev)
- File upload for YAML (paste-only for now)

---

## 5. Consequences

### Positive
- Clean separation of concerns allows independent development of parser, rules, and UI.
- Lightweight YAML parsing keeps dependencies minimal and build times fast.
- All static rules are tested and produce the full recommendation format from ADR-0001 Section 7.

### Negative
- Custom YAML parser won't catch collector-specific config validation errors (e.g., invalid receiver options). This is acceptable for a diagnostic tool focused on pipeline structure.
- Frontend types must be kept in sync with backend JSON manually (no code generation).
