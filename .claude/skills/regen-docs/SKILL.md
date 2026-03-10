---
name: regen-docs
description: Regenerate docs from code (rules, API, README)
disable-model-invocation: true
---

## Your task

Regenerate all auto-generated documentation from the backend directory.

### Steps

1. Run `go generate ./...` from the `backend/` directory.

2. Report what was generated (rule counts, route counts, etc.) based on the output.

3. If the generation fails, report the error and suggest a fix.
