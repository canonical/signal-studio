---
name: new-adr
description: Scaffold a new Architecture Decision Record
disable-model-invocation: true
---

## Context

- Existing ADRs: !`ls docs/adr/`

## Your task

Create a new ADR based on the user's description: $ARGUMENTS

### Steps

1. **Determine the next ADR number** from the existing files in `docs/adr/`.

2. **Create the ADR file** at `docs/adr/NNNN-<kebab-case-title>.md` using this template:

```markdown
# ADR-NNNN: <Title>

## Status

Proposed

## Date

<today's date in YYYY-MM-DD format>

## Related

- <links to related ADRs if applicable>

## Context

<Problem statement — what situation requires a decision?>

## Decision

<What was decided and why?>

## Scope

### Included

- <what's in scope for this change>

### Out of Scope

- <what's explicitly deferred>

## Consequences

### Positive

- <benefits>

### Negative

- <trade-offs>
```

3. Fill in the sections based on the user's description. The Context and Decision sections are the most important — make them specific and concrete.

4. Report the file path and ADR number when done.
