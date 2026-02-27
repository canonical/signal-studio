# Alert Rule YAML Parser

**Package:** `github.com/canonical/signal-studio/internal/alertcoverage`
**Source file:** `parser.go`
**ADR:** [ADR-0018: Alert Rule Coverage Analysis](../adr/0018-alert-rule-coverage-analysis.md)

## Purpose

The parser accepts YAML-encoded alert and recording rule files in two formats:
standard Prometheus rule files and Kubernetes `PrometheusRule` CRDs. It
auto-detects the format, extracts rule definitions, parses their PromQL
expressions to extract metric names, and returns a slice of `AlertRule` structs
ready for cross-referencing.

## Supported Formats

### Standard Prometheus Rule File

Used by Prometheus, Mimir, Cortex, and Thanos. Structure:

```yaml
groups:
  - name: service_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status="500"}[5m]) > 0.05
      - record: job:http_requests:rate5m
        expr: sum(rate(http_requests_total[5m])) by (job)
```

### Kubernetes PrometheusRule CRD

Used in Kubernetes environments with the Prometheus Operator. The standard rule
structure is wrapped inside `spec.groups`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: service-rules
spec:
  groups:
    - name: crd_alerts
      rules:
        - alert: PodCrashLoop
          expr: rate(kube_pod_container_status_restarts_total[15m]) > 0
```

## Design

### Entry Point

```go
func ParseRules(data []byte) ([]AlertRule, error)
```

The function is a two-phase pipeline:
1. `parseGroups(data)` — detects format and extracts `[]ruleGroup`.
2. `extractRules(groups)` — converts groups into `[]AlertRule` with PromQL
   extraction.

### Format Auto-Detection

`isCRD(data)` performs a lightweight string-based check:

```go
func isCRD(data []byte) bool {
    s := string(data)
    return strings.Contains(s, "apiVersion") && strings.Contains(s, "PrometheusRule")
}
```

This checks for the presence of both `apiVersion` and `PrometheusRule` anywhere
in the YAML text. This is a heuristic, not a full parse — it avoids
unmarshaling the YAML twice. The check is sufficient because:
- Standard Prometheus rule files never contain `apiVersion` or `PrometheusRule`.
- CRD files always contain both.
- False positives are unlikely in practice (a standard rule file would need a
  comment or string literal containing both terms).

### CRD Unwrapping

If the input is detected as a CRD, `parseCRD` unmarshals it into a `k8sCRD`
struct and extracts `crd.Spec.Groups`. The Kubernetes metadata (`apiVersion`,
`kind`, `metadata`) is discarded — only the `spec.groups` content is relevant.

### Standard Parsing

If the input is not a CRD, `parseStandard` unmarshals it into a
`prometheusRuleFile` struct and extracts `rf.Groups`.

### Rule Extraction

`extractRules` iterates through all groups and rules, performing the following
for each rule:

1. **Skip empty expressions** — rules with no `expr` field are silently skipped.

2. **Determine rule type:**
   - If `alert` field is set: `Type = "alert"`, `Name = alert`.
   - If `record` field is set: `Type = "record"`, `Name = record`.
   - If neither is set: the rule is skipped (not an alert or recording rule).

3. **Extract metric names** — calls `ExtractMetrics(r.Expr)` to parse the
   PromQL expression and extract referenced metric names plus the `usesAbsent`
   flag.

4. **Handle parse errors gracefully** — if `ExtractMetrics` fails for a
   specific rule, the error is recorded in `parseErrors` but processing
   continues with the remaining rules. A single malformed PromQL expression
   does not block analysis of valid rules.

5. **All-failed fallback** — if no rules were successfully parsed and there
   were parse errors, the function returns an error listing all failures. This
   prevents silently returning empty results when all rules are malformed.

### Internal Types

```go
type prometheusRuleFile struct {
    Groups []ruleGroup `yaml:"groups"`
}

type ruleGroup struct {
    Name  string `yaml:"name"`
    Rules []rule `yaml:"rules"`
}

type rule struct {
    Alert  string `yaml:"alert"`
    Record string `yaml:"record"`
    Expr   string `yaml:"expr"`
}

type k8sCRD struct {
    APIVersion string `yaml:"apiVersion"`
    Kind       string `yaml:"kind"`
    Spec       struct {
        Groups []ruleGroup `yaml:"groups"`
    } `yaml:"spec"`
}
```

The `rule` struct has both `Alert` and `Record` fields. In a valid Prometheus
rule file, exactly one of these is populated. The parser uses this to determine
the rule type.

## Edge Cases

| Scenario | Behavior |
|---|---|
| Standard Prometheus format | Parsed directly as `prometheusRuleFile`. |
| Kubernetes CRD format | Detected by `isCRD`, unwrapped from `spec.groups`. |
| Mixed alert and recording rules | Both types are extracted. Recording rules get `Type = "record"`. |
| Rule with empty `expr` | Skipped silently. |
| Rule with neither `alert` nor `record` | Skipped silently. |
| Malformed YAML | `yaml.Unmarshal` returns an error, propagated to caller. |
| Valid YAML but invalid PromQL in one rule | That rule is skipped with a parse error. Other rules are still processed. |
| All rules have invalid PromQL | Returns an error combining all parse error messages. |
| Empty groups list | Returns empty `[]AlertRule` and no error. |
| `absent()` in expression | `ExtractMetrics` sets `UsesAbsent=true` on the resulting `AlertRule`. |

## Graceful Degradation

The parser is designed to maximize the number of usable results. Individual
failures at the PromQL parsing level do not block the overall parse. This is
important because rule files in production can contain hundreds of rules, and a
single typo in one expression should not prevent analysis of the rest.

The error-aggregation strategy:
- Individual parse errors are collected in `parseErrors`.
- If at least one rule was successfully parsed, the errors are silently
  discarded and the valid rules are returned.
- Only if **all** rules fail to parse does the function return an error.

## Testing Strategy

Tests are in `parser_test.go` and cover both formats, rule types, and edge
cases:

- **`TestParseRules_Standard`** — standard Prometheus format with two alert
  rules. Verifies name, type, group, and metric name extraction.
- **`TestParseRules_CRD`** — Kubernetes `PrometheusRule` CRD format. Verifies
  CRD detection, unwrapping, and correct rule extraction.
- **`TestParseRules_RecordingRule`** — recording rule with `record` field.
  Verifies `Type = "record"` and correct name.
- **`TestParseRules_MixedAlertAndRecording`** — file with both alert and
  recording rules. Verifies both are extracted.
- **`TestParseRules_EmptyExpr`** — rule with no `expr` field is skipped,
  results in zero rules.
- **`TestParseRules_MalformedYAML`** — invalid YAML syntax returns an error.
- **`TestParseRules_EmptyGroups`** — `groups: []` returns zero rules with no
  error.
- **`TestParseRules_AbsentDetection`** — rule with `absent(up{job="api"})` has
  `UsesAbsent=true` after parsing. This is an end-to-end test through the
  parser into `ExtractMetrics`.
