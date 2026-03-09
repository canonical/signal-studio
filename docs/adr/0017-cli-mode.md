# ADR-0017: CLI Mode for Headless Analysis

**Status:** Approved
**Date:** 2026-02-26
**Related:** [ADR-0001: Project Specification](0001-project-specification.md), [ADR-0002: Tech Stack and Initial Architecture](0002-tech-stack-and-initial-architecture.md), [ADR-0011: Rules Sub-Package Extraction](0011-rules-sub-package-extraction.md)

---

## Context

Signal Studio currently runs exclusively as a web server. Users paste or upload a Collector YAML, and the backend parses it, runs rules, and returns findings via the HTTP API. This works well for interactive exploration but blocks several high-value use cases:

- **CI/CD integration** — fail a pipeline if a Collector config introduces critical issues
- **Scripted audits** — batch-analyze configs across a fleet of Collectors
- **Juju Doctor integration** — expose diagnostics as a provider for `juju doctor`
- **Shareable reports** — generate a report file without running a web server

CLI mode is the prerequisite for all of these. It is the #1 item on the feature backlog.

---

## Alternatives

### A. Single Binary with Subcommands

Add an `analyze` subcommand to the existing binary. The current server mode becomes `signal-studio serve` (or remains the default when no subcommand is given).

```
signal-studio analyze config.yaml          # static analysis, text output
signal-studio analyze config.yaml -f json  # JSON output
signal-studio analyze -                    # read from stdin
signal-studio serve                        # start HTTP server (current behavior)
```

**Pros:**

- Single build artifact — simpler CI, Docker images, distribution
- Follows established patterns (Docker, Terraform, OPA, Cobra-based tools)
- Shared binary means shared dependencies — no version skew between CLI and server

**Cons:**

- CLI users pay for server dependencies (gRPC, pdata) in binary size even if they only run static analysis
- Subcommand routing adds a small amount of complexity to `main.go`

### B. Separate Binaries

Two distinct `cmd/` entry points: `cmd/server/main.go` (existing) and `cmd/cli/main.go` (new). Produces `signal-studio-server` and `signal-studio`.

**Pros:**

- CLI binary can be much smaller (no gRPC, no pdata, no HTTP server dependencies)
- Clearer separation of concerns

**Cons:**

- Two build artifacts to manage, version, and distribute
- Docker images need to include both or choose one
- Risk of version skew if packages diverge

### C. Separate Binary, Static-Only (Lightweight)

A minimal CLI binary that only imports `config`, `rules/engine`, and output formatters. No tap, no metrics, no HTTP server.

**Pros:**

- Smallest possible binary (~5-8 MB estimated vs. ~21 MB for current server)
- Fastest startup — no gRPC initialization
- Could be distributed as a standalone download for CI

**Cons:**

- Same two-artifact management issues as Option B
- Cannot leverage live rules or catalog rules even if data were piped in

---

## Recommendation

**Option A: Single binary with subcommands.**

The binary size difference (~21 MB vs ~8 MB) is not significant enough to justify managing two artifacts. The single-binary approach is the established Go convention and simplifies deployment. If binary size becomes a concern later, a separate lightweight CLI binary can be split out without breaking the existing command interface.

Use `cobra` or standard library `flag` for subcommand routing. Given the small number of subcommands, standard library is sufficient.

---

## Design

### Command Interface

```
signal-studio analyze <path>    [flags]
signal-studio serve             [flags]
```

If invoked with no subcommand, default to `serve` for backward compatibility.

### Serve Flags

The `serve` subcommand accepts optional flags for auto-connecting to external endpoints on launch, removing the need for manual UI setup:

```
--metrics-url=<url>          # Auto-connect to Collector metrics endpoint on startup
--metrics-token=<token>      # Bearer token for metrics endpoint
--rules-url=<url>            # Auto-connect to Prometheus/Mimir rules endpoint on startup
--rules-token=<token>        # Bearer token for rules endpoint
--rules-org-id=<id>          # X-Scope-OrgID for Mimir multi-tenant setups
```

These complement the existing environment variables (`PORT`, `TAP_ENABLED`, etc.) and enable fully headless deployments:

```
signal-studio serve \
  --metrics-url http://collector:8888/metrics \
  --rules-url http://mimir:9090
```

The UI still shows the connection controls but they arrive pre-filled and connected. Users can disconnect or change endpoints at any time.

### Analyze Flags

The `analyze` subcommand accepts optional flags for alert rule coverage analysis (ADR-0018):

```
--alerts=<path>              # Alert rules file (Prometheus/CRD format)
--rules-url=<url>            # Fetch alert rules from Prometheus/Mimir API
--rules-token=<token>        # Bearer token for rules endpoint
--rules-org-id=<id>          # X-Scope-OrgID for Mimir multi-tenant setups
```

`--alerts` and `--rules-url` can be combined — file-based and API-sourced rules are merged. This supports the CI use case of checking a Collector config change against production alert rules:

```
signal-studio analyze config.yaml --rules-url http://mimir:9090
```

### Input Handling

| Input                                      | Behavior                                      |
| ------------------------------------------ | --------------------------------------------- |
| `signal-studio analyze config.yaml`        | Read file from path                           |
| `signal-studio analyze -`                  | Read from stdin                               |
| `cat config.yaml \| signal-studio analyze` | Auto-detect piped stdin when no path argument |

Auto-detection in Go:

```go
stat, _ := os.Stdin.Stat()
isPipe := (stat.Mode() & os.ModeCharDevice) == 0
```

### Output Formats

Four formats, selected via `--format` / `-f`:

| Format   | Flag value | Default when    | Use case                       |
| -------- | ---------- | --------------- | ------------------------------ |
| Text     | `text`     | stdout is a TTY | Human reading in terminal      |
| JSON     | `json`     | stdout is piped | Machine consumption, scripting |
| SARIF    | `sarif`    | never           | GitHub Code Scanning, VS Code  |
| Markdown | `markdown` | never           | PR comment bots                |

JUnit XML was considered but deferred. The "rules as test cases" metaphor is a poor fit for a linter — a rule that didn't fire is not a "passing test." Exit codes handle CI pass/fail, SARIF handles inline annotations, and the remaining audience for JUnit rendering (Jenkins, GitLab) can consume JSON output instead. Add JUnit if there's explicit demand.

Auto-detection: if `--format` is not specified, use `text` when stdout is a TTY, `json` when piped. This follows the `gh` CLI pattern.

#### Text Output

```
CRITICAL  missing-memory-limiter
          No memory_limiter processor in pipeline metrics/default
          Scope: pipeline:metrics/default

WARNING   receiver-endpoint-wildcard
          Receiver otlp binds to 0.0.0.0
          Scope: receiver:otlp

3 findings (1 critical, 1 warning, 1 info)
```

Color via ANSI codes when TTY detected. `--no-color` flag to override.

#### JSON Output

Reuse the existing `analyzeResponse` structure with a version field:

```json
{
  "formatVersion": "1",
  "findings": [...],
  "filterAnalyses": [...],
  "summary": {
    "total": 3,
    "critical": 1,
    "warning": 1,
    "info": 1
  }
}
```

#### SARIF Output

SARIF v2.1.0 mapping:

| Finding field            | SARIF field                                                             |
| ------------------------ | ----------------------------------------------------------------------- |
| `RuleID`                 | `result.ruleId` + `tool.driver.rules[].id`                              |
| `Title`                  | `tool.driver.rules[].shortDescription.text`                             |
| `Severity`               | `result.level` — `critical`→`error`, `warning`→`warning`, `info`→`note` |
| `Implication`            | `result.message.text` + `tool.driver.rules[].fullDescription.text`      |
| `Scope`                  | `result.locations[].logicalLocations[].name`                            |
| `Confidence`, `Evidence` | `result.properties` (property bag)                                      |
| `Snippet`                | `result.fixes[].description.text` (suggested fix)                       |

Use `owenrumney/go-sarif` (v3) for generation. Logical locations only for MVP — physical line numbers require enhancing the YAML parser to track node positions (future work).

GitHub Code Scanning integration: users upload the SARIF file via `codeql/sarif-upload-action` in a GitHub Actions workflow:

```yaml
- name: Analyze Collector Config
  run: signal-studio analyze config.yaml -f sarif > results.sarif

- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif
```

#### Markdown Output

Table format for PR comments:

```markdown
## Signal Studio Analysis

| Severity | Rule                       | Finding                        | Scope                    |
| -------- | -------------------------- | ------------------------------ | ------------------------ |
| critical | missing-memory-limiter     | No memory_limiter processor    | pipeline:metrics/default |
| warning  | receiver-endpoint-wildcard | Receiver otlp binds to 0.0.0.0 | receiver:otlp            |

**Summary:** 1 critical, 1 warning, 1 info
```

### Exit Codes

Following the ESLint/yamllint/Ruff convention:

| Code | Meaning                                           |
| ---- | ------------------------------------------------- |
| 0    | No findings above threshold                       |
| 1    | Findings above threshold                          |
| 2    | Tool error (invalid YAML, bad flags, I/O failure) |

Controlled by `--fail-on`:

```
--fail-on=critical    # exit 1 only on critical findings
--fail-on=warning     # exit 1 on warning or critical (default)
--fail-on=info        # exit 1 on any finding
```

### Severity Filtering

Two independent controls:

```
--min-severity=<info|warning|critical>   # filter output (default: info — show all)
--fail-on=<info|warning|critical>        # exit code threshold (default: warning)
```

Example: `--min-severity=warning --fail-on=critical` shows warnings and criticals in output but only fails the CI job on criticals.

### Architecture

The analysis logic is already well-factored. The current flow:

```
HTTP handler → config.Parse() → engine.Evaluate() → JSON response
```

Extract a shared `internal/analyze` package:

```
internal/analyze/
    analyze.go    # AnalyzeConfig(yaml, Options) → Report
```

```go
type Report struct {
    Config         *config.CollectorConfig
    Findings       []rules.Finding
    FilterAnalyses []filter.FilterAnalysis
}

type Options struct {
    MinSeverity rules.Severity
}

func Run(yamlBytes []byte, opts Options) (*Report, error) {
    cfg, err := config.Parse(yamlBytes)
    if err != nil {
        return nil, err
    }
    eng := engine.NewDefaultEngine()
    findings := eng.Evaluate(cfg)
    // apply severity filter
    // run filter analysis
    return &Report{...}, nil
}
```

Both `cmd/server/` and the new CLI subcommand call `analyze.Run()`. The HTTP handler in `api/handlers.go` is refactored to use it, reducing duplication.

Output formatting lives in `internal/report/`:

```
internal/report/
    text.go
    json.go
    sarif.go
    markdown.go
```

Each formatter implements:

```go
type Formatter interface {
    Format(report *analyze.Report, w io.Writer) error
}
```

### Scope Boundary

CLI mode runs **static rules only**. Live rules require a metrics connection, and catalog rules require an OTLP tap — both need a running Collector. The CLI is for offline, headless analysis of YAML files.

If future use cases warrant it, the CLI could accept supplementary data files (metric snapshots, catalog exports) to enable live and catalog rules, but this is explicitly out of scope.

---

## Implementation Requirements

### Non-Trivial Components

The following components involve significant logic and must each be documented in a dedicated markdown file under `docs/implementation/`. The implementation is not considered complete until all documentation files exist and accurately describe the component's design, behavior, edge cases, and testing strategy.

| Component                                | Package            | Documentation file                       | Description                                                                                                                                                                                      |
| ---------------------------------------- | ------------------ | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| SARIF formatter                          | `internal/report`  | `docs/implementation/sarif-formatter.md` | SARIF v2.1.0 generation including field mapping, logical locations, property bags, and `go-sarif` library usage. Must cover the full mapping table and how findings translate to SARIF results.  |
| Subcommand routing and flag parsing      | `cmd/server`       | `docs/implementation/cli-subcommands.md` | `flag.NewFlagSet` per subcommand, stdin auto-detection, TTY-based format auto-detection, exit code logic, and the `--fail-on` / `--min-severity` interaction.                                    |
| Analysis extraction (`internal/analyze`) | `internal/analyze` | `docs/implementation/analyze-package.md` | The shared analysis entrypoint used by both CLI and HTTP handler. Must document how it composes config parsing, rule evaluation, filter analysis, and severity filtering into a single `Report`. |
| Text formatter with ANSI color           | `internal/report`  | `docs/implementation/text-formatter.md`  | TTY detection, ANSI color codes per severity, `--no-color` override, and summary line formatting.                                                                                                |

### Test Coverage Requirements

All non-trivial components listed above must have test coverage of **90% or higher**. Specifically:

- **`internal/analyze`** — test `Run()` with valid configs, invalid YAML, severity filtering, and empty configs. Verify that findings and filter analyses are correctly composed.
- **`internal/report` (all formatters)** — each formatter must have tests verifying correct output structure. SARIF tests must validate against the SARIF v2.1.0 schema. Markdown tests must verify table structure.
- **Subcommand routing** — test flag parsing for both `analyze` and `serve` subcommands, default behavior when no subcommand is given, invalid flag combinations, and stdin pipe detection.
- **Exit code logic** — test all combinations of `--fail-on` threshold vs. finding severities to verify correct exit codes (0, 1, 2).

---

## Open Questions

1. ~~**Cobra vs standard library?**~~ — **Resolved.** Standard library. With two subcommands (`analyze`, `serve`), Cobra's dependency cost is not justified. Routing is a `switch` on `os.Args[1]`, flag parsing is `flag.NewFlagSet()` per subcommand. If the CLI surface grows to three or more subcommands, migrate to Cobra — the flag definitions and handler functions transfer directly.

2. ~~**Config file for CLI defaults?**~~ — **Resolved.** Not for MVP. The flag surface is small (4 flags) and fits in a CI step's `args` line. A config file becomes worthwhile when per-repo rule suppression is needed — that's a bigger design decision. Revisit alongside rule suppression.

3. ~~**Multiple input files?**~~ — **Deferred.** Useful for fleet audits but adds complexity (per-file vs. aggregate results, output grouping). Revisit post-MVP if there's demand.

4. ~~**YAML line number resolution?**~~ — **Resolved: yes.** Enhance `config.Parse()` to use `yaml.Node` instead of direct unmarshaling, propagating source line numbers through to findings. Benefits both SARIF output (physical locations for GitHub Code Scanning) and the web UI (clicking a finding could highlight the relevant line in the Monaco editor). This should be implemented as a prerequisite or early follow-up to CLI mode.

5. ~~**Output to file vs stdout?**~~ — **Resolved: not needed.** Shell redirection (`> results.sarif`) is sufficient and more composable. No `--output` flag.

---

## Impact Assessment

### User Impact: High

CLI mode unlocks CI/CD integration, which is the most requested capability for linting tools. Every team that manages Collector configs in Git benefits from automated analysis on PR.

### Differentiation: Moderate

- OTelBin does not have a CLI.
- `otel-config-validator` (AWS) has a CLI but only validates syntax, not semantics.

The combination of static rules + filter analysis + SARIF output for GitHub Code Scanning is unique.

### Effort: Low-Medium

The analysis engine is already factored for reuse. The main work is:

- Subcommand routing (~0.5 day)
- `internal/analyze` extraction + refactor of `api/handlers.go` (~0.5 day)
- Text formatter (~0.5 day)
- JSON formatter (~0.25 day, mostly exists)
- SARIF formatter (~1 day)
- Markdown formatter (~0.25 day)
- Exit code + severity filtering (~0.25 day)
- Tests (~1 day)

Estimated total: **4 days**.

### Unlock Potential: Very High

CLI mode is the prerequisite for:

- Juju Doctor integration (backlog #7)
- GitOps PR generation (backlog #9)
- Shareable diagnostic reports (backlog #6)
- CI/CD pipeline quality gates
- Alert rule coverage analysis (ADR-0018) in headless mode

---

## Consequences

### Positive

- Enables CI/CD integration — the most common deployment model for linters
- SARIF output integrates directly with GitHub Code Scanning for inline PR annotations
- Unblocks Juju Doctor integration and GitOps workflows
- Forces a clean separation between analysis logic and HTTP serving, improving code quality
- Single binary distribution simplifies deployment and versioning
- Auto-detect format (text for TTY, JSON for pipe) provides good defaults with no configuration

### Negative

- CLI users carry the full server binary weight (~21 MB) even though they only use static analysis
- Must maintain four output formatters going forward — each format is a maintenance surface
- YAML line number resolution (`yaml.Node` migration) is needed for full SARIF value — without it, findings appear as repo-level rather than line-level annotations in GitHub Code Scanning
- Standard library `flag` package is less ergonomic than Cobra for help text and shell completion — revisit if a third subcommand is needed
