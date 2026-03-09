# Contributing to Signal Studio

Thank you for your interest in contributing to Signal Studio!

## Getting Started

### Prerequisites

- Go 1.24+
- Node.js 22+
- [just](https://github.com/casey/just) (optional, for convenience commands)

### Setup

```sh
# Backend
cd backend
go mod download

# Frontend
cd frontend
npm install
```

### Running locally

```sh
# Both backend and frontend (requires just)
just dev

# Or separately:
cd backend && go run ./cmd/server
cd frontend && npm run dev
```

The frontend dev server starts on `:5173` and proxies `/api` requests to the backend on `:8080`.

## Making Changes

### Code structure

- `backend/` — Go server, analysis engine, rules, API
- `frontend/` — React SPA (TypeScript, Vanilla Framework CSS)
- `docs/adr/` — Architecture decision records

### Tests

```sh
# Backend
cd backend && go test ./...

# Frontend
cd frontend && npm test
```

Backend code coverage must stay at or above 80% for all packages.

### Generated documentation

Parts of the documentation are generated from Go source code. After changing rules, API routes, or environment variables, regenerate with:

```sh
cd backend && go generate ./internal/rules/engine/
```

See [docs/generating-docs.md](docs/generating-docs.md) for details on what is generated and when.

### Architecture decisions

Significant changes should be documented in an ADR (Architecture Decision Record) under `docs/adr/`. Number them sequentially (e.g., `0020-your-change.md`).
Before new features are considered candidates for merge, they need to be preceeded by an accepted ADR.

## Submitting Changes

1. Fork the repository and create a branch from `master`.
2. Make your changes, add tests, and verify they pass.
3. Ensure generated docs are up to date (`go generate`).
4. Open a pull request with a clear description of the change.

## Reporting Issues

Use [GitHub Issues](https://github.com/canonical/signal-studio/issues) to report bugs or request features.
