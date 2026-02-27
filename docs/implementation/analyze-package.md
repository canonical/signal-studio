# internal/analyze — Shared Analysis Entrypoint

**Package:** `github.com/canonical/signal-studio/internal/analyze`
**ADR:** [ADR-0017: CLI Mode](../adr/0017-cli-mode.md)

## Purpose

The `analyze` package provides a shared analysis entrypoint used by both the CLI `analyze` subcommand and the HTTP API handler. It encapsulates the core analysis pipeline: config parsing, rule evaluation, filter analysis, and severity filtering.

## Key Types

### `Report`

The output of an analysis run. Contains:

- `Config` — the parsed `CollectorConfig`
- `Findings` — filtered list of `rules.Finding`
- `FilterAnalyses` — filter processor analysis results (populated when catalog data is available)
- `Summary` — counts of findings by severity

### `Options`

Controls analysis behavior:

- `MinSeverity` — minimum severity threshold for output filtering. Findings below this level are excluded from the report.

### `Summary`

Counts findings by severity: `Total`, `Critical`, `Warning`, `Info`.

## Analysis Pipeline

```
Run(yamlBytes, opts)
    │
    ├── config.Parse(yamlBytes)          → CollectorConfig
    │
    ├── engine.NewDefaultEngine()
    │   └── eng.Evaluate(cfg)            → []Finding (static rules only)
    │
    ├── filter.ExtractFilterConfigs(cfg)  → []FilterConfig
    │   └── (placeholder: no catalog in CLI mode)
    │
    ├── filterBySeverity(findings, opts.MinSeverity)
    │
    └── buildSummary(filtered)           → Summary
```

## Scope Boundary

`Run()` executes **static rules only**. Live rules (requiring a metrics connection) and catalog rules (requiring the OTLP tap) are not available in CLI mode. The HTTP handler in `api/handlers.go` continues to call `engine.EvaluateWithMetrics()` and `engine.EvaluateWithCatalog()` directly for the interactive path.

## Helper Functions

- `SeverityRank(s)` — returns numeric rank (critical=3, warning=2, info=1) for comparison
- `ExceedsThreshold(findings, threshold)` — returns true if any finding meets or exceeds the threshold severity. Used by the CLI to determine exit codes.
- `filterBySeverity(findings, minSeverity)` — filters findings below the minimum severity
- `buildSummary(findings)` — counts findings by severity

## Testing Strategy

Tests need to cover the main steps of the workflow and have a code coverage of at least 90%. At the time of writing, the tests cover:

- Valid config analysis (verifies report structure)
- Invalid YAML (returns error)
- Empty config (no panic)
- Severity filtering at all three levels
- `SeverityRank` for all severities
- `ExceedsThreshold` with various combinations including empty findings
- `filterBySeverity` with all threshold levels
