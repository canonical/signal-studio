# ADR-0016: Finding Confidence, Caveat, and Scope

## Status

Accepted

## Context

Rule findings previously contained severity, evidence, explanation, impact, and a
snippet, but lacked metadata to help users gauge how much to trust a finding and
what side effects acting on it might have. Specifically:

- **Confidence** -- a static config check is deterministic (high confidence),
  while a live metric threshold may be transient (medium confidence).
- **Caveat** -- acting on a recommendation can have side effects; users need to
  know about false-positive risk and trade-offs before making changes.
- **Scope** -- findings lacked an explicit label for what they apply to (which
  pipeline, component, signal, or metric).

Additionally, the frontend rendered an empty snippet block when the `snippet`
field was empty.

## Decision

### Backend

Add a `Confidence` type (`high`, `medium`, `low`) and three new fields to
`Finding`:

| Field        | Type         | JSON tag                    |
|--------------|--------------|-----------------------------|
| `Confidence` | `Confidence` | `"confidence"`              |
| `Caveat`     | `string`     | `"caveat,omitempty"`        |
| `Scope`      | `string`     | `"scope,omitempty"`         |

Assignment strategy:

- **Static rules**: `ConfidenceHigh` (config analysis is deterministic).
- **Live rules**: `ConfidenceMedium` (metric windows may be transient).
- **Catalog rules**: `ConfidenceHigh` for deterministic checks (filter drops
  everything), `ConfidenceMedium` for statistical outliers.

Each rule supplies a rule-specific caveat explaining false-positive scenarios
and a scope string using a `kind:name` convention (e.g. `pipeline:traces`,
`exporter:otlp/backend`, `metric:http.server.duration`, `all pipelines`).

### Frontend

- `Finding` type gains `confidence`, `caveat?`, and `scope?`.
- `FindingsPanel` shows a confidence badge next to the title, a scope tag below
  it, a caveat row in expanded details, and hides the snippet block when empty.

## Consequences

- All existing tests continue to pass; spot-check assertions verify confidence
  is set on key rules.
- Coverage remains above 80% across all backend packages.
- The API response grows slightly but all new fields are either required
  (`confidence`) or omitted when empty (`caveat`, `scope`).
