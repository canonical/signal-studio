# Generating Documentation

Parts of the project documentation are generated from Go source code. The generated files have a `DO NOT EDIT` comment at the top — edit the source instead.

## What is generated

| File | Source |
| ---- | ------ |
| `docs/rules.md` | `Description()` and `DefaultSeverity()` methods on each rule struct |
| `docs/api.md` | `api.Routes` slice in `internal/api/routes.go` |
| `README.md` (Configuration table) | `serverconfig.EnvVars` slice in `internal/serverconfig/env.go` |

## How to regenerate

From the `backend/` directory:

```sh
go generate ./internal/rules/engine/
```

This runs `cmd/docgen` which regenerates all three targets in one pass.

## When to regenerate

Run `go generate` after any of these changes:

- Adding, removing, or renaming a rule
- Changing a rule's `Description()` or `DefaultSeverity()`
- Adding or removing an API route in `api.Routes`
- Adding or changing an environment variable in `serverconfig.EnvVars`
