---
name: coverage-check
description: Check backend code coverage and flag packages below 80%
disable-model-invocation: true
---

## Your task

Check backend code coverage across all packages and report any that fall below the 80% threshold required by CLAUDE.md.

### Steps

1. Run all tests with coverage from the `backend/` directory:
   ```
   go test ./... -coverprofile=cover.out
   ```

2. Extract per-package coverage and flag any package below 80%:
   ```
   go tool cover -func=cover.out | tail -1
   ```

3. Report a summary table showing each package and its coverage percentage. Highlight any package below 80% as a problem. Exclude packages with `[no test files]` or `0.0%` coverage for `cmd/` packages (these are acceptable).

4. If all packages are above 80%, confirm the threshold is met. If any are below, suggest which functions lack coverage and what tests could be added.
