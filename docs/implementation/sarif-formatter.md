# SARIF Formatter

**Package:** `github.com/canonical/signal-studio/internal/report`
**File:** `sarif.go`
**ADR:** [ADR-0017: CLI Mode](../adr/0017-cli-mode.md)

## Purpose

Generates SARIF v2.1.0 output for integration with GitHub Code Scanning, VS Code, and other SARIF-consuming tools. Uses the `owenrumney/go-sarif/v2` library.

## Field Mapping

| Finding field | SARIF field |
|---|---|
| `RuleID` | `result.ruleId` + `tool.driver.rules[].id` |
| `Title` | `tool.driver.rules[].shortDescription.text` |
| `Severity` | `result.level` |
| `Explanation` | `result.message.text` |
| `WhyItMatters` | `tool.driver.rules[].fullDescription.text` |
| `Scope` / `Pipeline` | `result.locations[].logicalLocations[].name` |
| `Confidence`, `Evidence`, `Impact`, `Caveat` | `result.properties` (property bag) |
| `Snippet` | `result.fixes[].description.text` |

## Severity Mapping

| Signal Studio | SARIF level |
|---|---|
| `critical` | `error` |
| `warning` | `warning` |
| `info` | `note` |

## Structure

```
SARIFFormatter.Format(report, writer)
    │
    ├── sarif.New(Version210)
    │
    ├── sarif.NewRunWithInformationURI("signal-studio", url)
    │
    ├── Register rules (deduplicated by RuleID)
    │   └── run.AddRule(id).WithShortDescription().WithFullDescription()
    │
    ├── For each finding:
    │   ├── NewRuleResult(ruleID).WithLevel().WithMessage()
    │   ├── Add logical location (scope or pipeline name)
    │   ├── Add properties bag (confidence, evidence, impact, caveat)
    │   └── Add fix (snippet as fix description)
    │
    └── sarifReport.Write(writer)
```

## Design Decisions

**Logical locations only.** Physical line numbers (file:line) require enhancing the YAML parser to use `yaml.Node` for position tracking. This is planned as a follow-up (ADR-0017, open question 4). Without line numbers, GitHub Code Scanning shows findings at the repository level rather than inline.

**Rule deduplication.** The same `RuleID` may appear in multiple findings (e.g., `missing-memory-limiter` fires once per pipeline). The SARIF `tool.driver.rules[]` array registers each rule only once, while `results[]` contains every finding instance.

**Property bag.** Fields that don't have a direct SARIF mapping (`confidence`, `evidence`, `impact`, `caveat`) are stored in the SARIF property bag (`result.properties`). These are accessible to downstream tools but not displayed by default in GitHub Code Scanning.

**Fixes.** The `Snippet` field maps to `result.fixes[].description.text`. This is a suggested fix description, not a code change — SARIF fixes with actual code changes require `artifactChanges` which need physical locations.

## GitHub Code Scanning Integration

```yaml
- name: Analyze Collector Config
  run: signal-studio analyze config.yaml -f sarif > results.sarif

- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif
```

## Testing Strategy

Tests verify:
- Valid SARIF JSON structure with version and $schema fields
- Tool driver name and info URI
- Rule registration and deduplication for duplicate RuleIDs
- Severity mapping (critical→error, warning→warning, info→note)
- Logical locations populated from scope/pipeline
- Property bag contents (confidence, evidence, impact, caveat)
- Fix descriptions from snippets
- Findings without scope/pipeline have no locations
- Empty report produces valid SARIF

Coverage: 95.6%
