---
name: new-rule
description: Scaffold a new rule (static, live, or catalog) with tests and registration
disable-model-invocation: true
---

## Context

- Rule types and interfaces: !`cat backend/internal/rules/types.go`
- Current static rules: !`cat backend/internal/rules/static/all.go`
- Current live rules: !`cat backend/internal/rules/live/all.go`
- Current catalog rules: !`cat backend/internal/rules/catalog/all.go`

## Your task

Create a new rule based on the user's description: $ARGUMENTS

### Steps

1. **Determine the rule type** from the user's description:
   - `static` — inspects `config.CollectorConfig` only
   - `live` — also needs `metrics.Store` (implements `LiveRule`)
   - `catalog` — also needs tap catalog + filter analyses (implements `CatalogRule`)

2. **Create the rule file** at `backend/internal/rules/<type>/<rule_id>.go`:
   - Struct with no fields
   - `ID()` returning kebab-case identifier (prefixed with `live-` or `catalog-` for non-static rules)
   - `Description()` returning a concise one-liner
   - `DefaultSeverity()` returning the appropriate severity
   - `Evaluate()` (and `EvaluateWithMetrics`/`EvaluateWithCatalog` for live/catalog)
   - Follow existing patterns in the package for Finding construction (Evidence, Implication with `\nHowever,` pattern, Recommendation, Snippet, Scope)

3. **Register the rule** by adding `&StructName{}` to `all.go` in the appropriate package.

4. **Add tests** to the existing test file in the package:
   - Static: `backend/internal/rules/static/extended_test.go`
   - Live: `backend/internal/rules/live/live_test.go`
   - Catalog: `backend/internal/rules/catalog/catalog_test.go`
   - Use the test naming convention: `Test<RuleName>_<Condition>` (static/catalog) or `Test<RuleName><Condition>` (live)
   - Include at minimum: one fire case, one non-fire case, and relevant edge cases
   - Use existing test helpers (`mustParse`, `findByRule`, `makeSnapshot`, etc.)

5. **Update the `TestAllRules` count** in the test file to reflect the new total.

6. **Run tests**: `go test ./internal/rules/<type>/...` — fix any failures.

7. **Regenerate docs**: `go generate ./...` from the backend directory.

8. **Verify coverage**: `go test ./internal/rules/<type>/... -coverprofile=cover.out && go tool cover -func=cover.out | tail -1` — must stay above 80%.

Report the rule ID, test results, and coverage when done.
