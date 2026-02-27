# Text Formatter

**Package:** `github.com/canonical/signal-studio/internal/report`
**File:** `text.go`
**ADR:** [ADR-0017: CLI Mode](../adr/0017-cli-mode.md)

## Purpose

Human-readable terminal output with optional ANSI color codes. Default format when stdout is a TTY.

## Output Format

```
CRITICAL  missing-memory-limiter
          No memory_limiter processor in pipeline metrics/default
          Scope: pipeline:metrics/default

WARNING   receiver-endpoint-wildcard
          Receiver otlp binds to 0.0.0.0
          Scope: receiver:otlp

3 findings (1 critical, 1 warning, 1 info)
```

## Color Behavior

Colors are enabled when **both** conditions are true:
1. `NoColor` is false (not overridden by `--no-color` flag)
2. The output writer is a TTY (detected via `os.File.Stat()` and `ModeCharDevice`)

Writing to a file, pipe, or `bytes.Buffer` never produces ANSI codes regardless of the `NoColor` setting.

### Severity Colors

| Severity | ANSI |
|---|---|
| `critical` | Bold red (`\033[1m\033[31m`) |
| `warning` | Yellow (`\033[33m`) |
| `info` | Cyan (`\033[36m`) |

## Structure

```
TextFormatter.Format(report, writer)
    │
    ├── Detect color: !NoColor && isTerminal(writer)
    │
    ├── For each finding:
    │   ├── Severity (uppercase, left-padded to 10 chars, optionally colored)
    │   ├── Rule ID
    │   ├── Explanation (indented)
    │   └── Scope (indented, if present)
    │
    └── Summary line: "N findings (X critical, Y warning, Z info)"
```

## TTY Detection

```go
func isTerminal(w io.Writer) bool {
    if f, ok := w.(*os.File); ok {
        fi, _ := f.Stat()
        return (fi.Mode() & os.ModeCharDevice) != 0
    }
    return false
}
```

Only `*os.File` writers can be terminals. Any other `io.Writer` (buffers, pipes, HTTP response writers) returns false.

## Testing Strategy

Tests verify:
- Correct output structure (severity labels, rule IDs, explanations, scopes)
- Empty report produces "0 findings"
- `NoColor: true` suppresses all ANSI escape codes
- Non-TTY writer (bytes.Buffer) never produces ANSI codes even with `NoColor: false`
- Color codes applied to each severity level
- Scope line omitted when scope is empty
- Summary line with colored parts

Coverage: 100% (text.go: Format, writeFinding, writeSummary, colorSeverity)
