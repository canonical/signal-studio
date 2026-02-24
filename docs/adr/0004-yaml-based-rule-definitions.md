# ADR-0004: YAML-Based Rule Definitions

**Status:** Proposed
**Date:** 2026-02-23
**Related:** [ADR-0003: Additional Static Rules](0003-additional-static-rules.md)

---

## Context

All 18 linting rules are currently compiled Go structs implementing the `Rule` interface (`ID()` + `Evaluate(*CollectorConfig)`). This means adding, modifying, or disabling a rule requires recompiling the binary.

This ADR evaluates whether rules could instead be defined as YAML files loaded at runtime, enabling users to customize the rule set without rebuilding.

---

## Rule Complexity Breakdown

Categorizing the existing 18 rules by the kind of logic they require:

| Category | Count | Examples |
|---|---|---|
| Presence / absence checks | 7 | missing-memory-limiter, debug-exporter-in-pipeline, empty-pipeline |
| Processor ordering | 3 | memory-limiter-not-first, batch-before-sampling, batch-not-near-end |
| Deep config inspection | 5 | memory-limiter-without-limits, exporter-no-sending-queue, receiver-endpoint-wildcard |
| Cross-referencing | 1 | undefined-component-ref |
| Combination (count + absence) | 2 | unused-components, multiple-exporters-no-routing |

All 18 rules are expressible declaratively given a sufficiently capable expression language. A pure attribute-check YAML schema (like Checkov's) would only cover the 7 simplest rules.

---

## Approaches Evaluated

### A. Template + Parameters (KubeLinter-style)

Abstract existing rules into reusable templates (e.g., `processor-must-exist`, `processor-ordering`, `config-field-check`). YAML rules select a template and fill in parameters.

```yaml
- id: custom-missing-memory-limiter
  template: processor-must-exist
  severity: critical
  params:
    type: memory_limiter
  message: "memory_limiter is required in every pipeline"
```

**Pros:**
- Very simple for users — no expression syntax to learn
- Easy to validate — finite set of templates with known parameter schemas
- Safe — impossible to write a rule that crashes the engine

**Cons:**
- Expressiveness capped at what templates expose
- Ordering checks, deep config inspection, and cross-references each need a dedicated template
- Adding a new rule *category* still requires Go code
- Estimated 8–10 templates needed to cover existing rules

### B. Expression Language in YAML (expr-lang/expr)

Rules defined in YAML with conditions written in [expr-lang](https://github.com/expr-lang/expr), a Go-centric expression language with compile-time type checking.

```yaml
- id: memory-limiter-not-first
  severity: critical
  scope: pipeline
  condition: >
    any(Processors, {ComponentType(#) == "memory_limiter"})
    && ComponentType(Processors[0]) != "memory_limiter"
  message: "memory_limiter must be the first processor"
```

**Pros:**
- All 18 rules are expressible — no Go fallback needed
- Expressions are type-checked against the `CollectorConfig` Go struct at load time
- Performance is excellent (~70 ns/op per expression evaluation)
- Mature library used by Google Cloud, Uber, GoDaddy
- Users can write arbitrary conditions for organization-specific policies

**Cons:**
- Users must learn the expression syntax (moderate learning curve)
- Expression strings can become hard to read for complex rules
- Error messages from expression failures are less friendly than template violations
- Adds a runtime dependency (~3 MB to binary)

### C. CEL (Common Expression Language)

Similar to Option B but using Google's [CEL](https://cel.dev/) instead of expr-lang.

**Pros:**
- Standardized — used by Kubernetes ValidatingAdmissionPolicy, Istio, Firebase
- Non-Turing complete — guaranteed termination
- Has a built-in YAML policy format

**Cons:**
- Proto-first design adds type conversion overhead for a pure Go project
- Heavier dependency than expr-lang
- Slightly slower (~91 ns/op)
- Less idiomatic Go field access

### D. OPA/Rego

Rules written in the Rego language, evaluated via the OPA engine.

**Pros:**
- Most powerful — full policy language with aggregation, set operations, partial evaluation
- Ecosystem (conftest, Kubernetes Gatekeeper) is familiar to platform engineers

**Cons:**
- Rego is a specialized language with a steep learning curve
- Millisecond-scale evaluation (100x slower than expr/CEL — still acceptable for this use case)
- Large dependency
- Overkill for configuration linting

### E. Hybrid — Compiled Core + YAML Custom Rules

Keep the 18 built-in rules as compiled Go. Add an optional YAML rule file using expr-lang for user-defined custom rules.

```yaml
customRules:
  - id: require-mtls-on-exporters
    severity: warning
    scope: exporter
    filter: 'Type in ["otlp", "otlphttp"]'
    condition: '"tls" in Config'
    message: "Exporters should use TLS"
```

**Pros:**
- Zero regression — existing rules unchanged and fully tested
- Users can extend without rebuilding
- Incremental — built-in rules can be migrated to YAML over time if desired
- Compiled rules remain the fast path for the default rule set

**Cons:**
- Two rule formats to maintain (Go + YAML)
- Users may expect all rules to be overridable/disablable via YAML
- Slightly more complex engine (must merge built-in + custom rules)

---

## Feasibility Assessment

| Dimension | Template + Params | expr-lang | CEL | OPA/Rego | Hybrid |
|---|---|---|---|---|---|
| Coverage of existing rules | ~60% without new templates | 100% | 100% | 100% | 100% (Go + expr) |
| User learning curve | Very low | Moderate | Moderate | High | Moderate |
| Implementation effort | Medium (8–10 templates) | High (rewrite all rules) | High | High | Low–medium |
| Binary size impact | None | ~3 MB | ~5 MB | ~10 MB | ~3 MB |
| Performance impact | None | Negligible | Negligible | Measurable but acceptable | Negligible |
| Type safety | Schema validation | Compile-time checking | Parse-time checking | Runtime only | Compile-time for custom |
| Can users add rules? | Within template bounds | Yes, arbitrary | Yes, arbitrary | Yes, arbitrary | Yes, arbitrary |
| Can users disable rules? | Yes | Yes | Yes | Yes | Needs explicit support |

---

## Recommendation

**Option E (Hybrid)** is the most pragmatic path:

1. **Low risk** — the 18 compiled rules are well-tested and performant; no need to rewrite them.
2. **Incremental** — user-defined rules via YAML + expr-lang can be added as a feature without disrupting existing functionality.
3. **Extensible** — organizations can add rules specific to their environment (e.g., "all exporters must use mTLS", "pprof must be disabled in production namespaces") without forking.
4. **Migration path** — if YAML rules prove popular, built-in rules can be migrated to YAML one-by-one over time, eventually reaching Option B if desired.

The recommended expression engine is **expr-lang/expr** due to its Go-native type checking, minimal footprint, and active maintenance.

Implementation would involve:
- A `CustomRule` struct that loads YAML, compiles expr expressions at startup, and implements the existing `Rule` interface
- A `--rules` CLI flag or `rules_dir` config option pointing to YAML rule files
- A rule loader that merges built-in + custom rules into the engine

Estimated effort: ~2–3 days for the loader, expression environment, and tests.

---

## Decision

Pending team discussion.

---

## Consequences

- Adding expr-lang as a dependency increases binary size by ~3 MB.
- A YAML rule schema must be documented for users writing custom rules.
- Custom rule expressions that reference `CollectorConfig` fields create a coupling — field renames would break user rules (mitigated by expr's load-time type checking, which surfaces errors at startup).
- The engine must handle expression compilation errors gracefully (log + skip the rule, not crash).
- Tests should cover both the YAML loading path and expression evaluation against sample configs.
