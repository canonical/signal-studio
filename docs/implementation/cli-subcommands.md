# CLI Subcommand Routing

**File:** `cmd/server/main.go`
**ADR:** [ADR-0017: CLI Mode](../adr/0017-cli-mode.md)

## Subcommands

| Command | Behavior |
|---|---|
| `signal-studio analyze <path> [flags]` | Static analysis, output to stdout |
| `signal-studio serve [flags]` | Start HTTP server (existing behavior) |
| `signal-studio` (no subcommand) | Default to `serve` for backward compatibility |
| `signal-studio <unknown>` | Treat as `serve` args for backward compatibility |

## Flag Parsing

Uses `flag.NewFlagSet` per subcommand. The standard library `flag` package stops at the first non-flag argument, so a `splitArgs` helper separates flags from positional arguments to allow `signal-studio analyze config.yaml -f json`.

### `splitArgs(args) → (flags, positional)`

Scans the argument list and classifies each as a flag or positional:
- Arguments starting with `-` are flags
- `=`-style values (`--format=json`) are self-contained
- Bool flags (`--no-color`) don't consume the next argument
- Other flags consume the next non-flag argument as their value

### Analyze Flags

| Flag | Default | Description |
|---|---|---|
| `--format`, `-f` | auto-detect | Output format: text, json, sarif, markdown |
| `--fail-on` | `warning` | Exit code threshold: info, warning, critical |
| `--min-severity` | `info` | Minimum severity to include in output |
| `--no-color` | `false` | Disable ANSI color output |
| `--alerts` | — | Alert rules file (placeholder for ADR-0018) |
| `--rules-url` | — | Prometheus/Mimir rules API URL (placeholder for ADR-0018) |
| `--rules-token` | — | Bearer token for rules endpoint |
| `--rules-org-id` | — | X-Scope-OrgID for Mimir multi-tenant setups |

### Serve Flags

| Flag | Default | Description |
|---|---|---|
| `--metrics-url` | — | Auto-connect to Collector metrics endpoint on startup |
| `--metrics-token` | — | Bearer token for metrics endpoint |
| `--rules-url` | — | Auto-connect to Prometheus/Mimir rules endpoint |
| `--rules-token` | — | Bearer token for rules endpoint |
| `--rules-org-id` | — | X-Scope-OrgID for Mimir multi-tenant setups |

## Format Auto-Detection

When `--format` is not specified:
- **stdout is a TTY** → `text`
- **stdout is piped** → `json`

Detection uses `os.Stdout.Stat()` and checks `os.ModeCharDevice`. This follows the `gh` CLI pattern.

## Input Handling

| Input | Behavior |
|---|---|
| `analyze config.yaml` | Read file from path |
| `analyze -` | Explicit stdin |
| `echo yaml \| analyze` | Auto-detect piped stdin (no path, `ModeCharDevice` unset) |
| `analyze` (no pipe, no path) | Error: "no input file specified and stdin is not piped" |

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | No findings above `--fail-on` threshold |
| 1 | Findings above threshold |
| 2 | Tool error (invalid YAML, bad flags, I/O failure) |

### `--fail-on` and `--min-severity` Interaction

These are independent:
- `--min-severity` filters what appears in output
- `--fail-on` controls the exit code

Example: `--min-severity=warning --fail-on=critical` shows warnings and criticals in the output but only exits 1 if there are criticals.

## Serve Auto-Connect

When `--metrics-url` is provided to `serve`, the server calls `mgr.Connect(ScrapeConfig{URL: url})` on startup. The UI arrives with the metrics connection already established. If the connection fails, a warning is logged but the server still starts.

## Testing Strategy

The subcommand routing and flag parsing live in `cmd/server/main.go` which has no test file (it's the `main` package). Testing is done through:
- Integration smoke tests: build the binary and run with various flag combinations
- The `internal/analyze` and `internal/report` packages cover the core logic with unit tests
- Exit code behavior verified via smoke tests

The `splitArgs` function should be moved to a testable package if the CLI surface grows.
