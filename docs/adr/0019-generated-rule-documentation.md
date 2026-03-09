# ADR-0019: Generated Rule Documentation

## Status

Accepted

## Context

The rules reference documentation in `docs/rules.md` was manually maintained, duplicating information already present in rule implementations. This creates a maintenance burden — every new rule requires updating both the code and the markdown file, and the two can drift out of sync.

## Decision

Add `Description() string` and `DefaultSeverity() Severity` methods to the `Rule` interface so that every rule exposes its metadata programmatically. Write a `cmd/docgen` generator that imports the default engine, iterates all rules, classifies them by type (static, live, catalog), and outputs `docs/rules.md`.

The generator runs via `go generate` from the engine package.

## Consequences

- Rule documentation is always in sync with the code — a new rule that implements the interface automatically appears in the generated docs.
- The `Rule` interface grows by two methods, requiring all implementations to add them. This is a one-time cost.
- `docs/rules.md` becomes a generated file and should not be edited by hand.
