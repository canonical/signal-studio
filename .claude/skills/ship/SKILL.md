---
name: ship
description: Split working tree changes into logical git commits and push
disable-model-invocation: true
---

## Context

- Current git status: !`git status`
- Current git diff (staged and unstaged): !`git diff HEAD --stat`
- Current branch: !`git branch --show-current`
- Recent commit style: !`git log --oneline -10`

## Your task

Split the uncommitted changes into logical, well-scoped git commits and push them. Based on the arguments: $ARGUMENTS

### Guidelines

1. **Analyze the changes** and group them into logical units:
   - Rule implementation + its tests + registration in `all.go` = one commit
   - ADR documents = separate commit
   - Doc regeneration (`docs/rules.md`, `docs/api.md`) = separate commit or bundled with related rule commits
   - Refactors / file renames = separate commit
   - Test-only changes (coverage improvements) = separate commit
   - Frontend vs backend changes = separate commits unless tightly coupled

2. **Commit message style** — match the existing convention from recent commits:
   - Prefix: `feat:`, `fix:`, `refactor:`, `test:`, `chore:`, `docs:`
   - Concise, lowercase, no period
   - Focus on the "why" not the "what"

3. **Stage files specifically** — never use `git add -A` or `git add .`. Add files by name.

4. **Push** to the current branch when all commits are created.

5. Report a summary of all commits created (hash + message).
