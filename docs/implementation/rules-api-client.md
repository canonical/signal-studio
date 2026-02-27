# Prometheus/Mimir Rules API Client

**Package:** `github.com/canonical/signal-studio/internal/alertcoverage`
**Source file:** `client.go`
**ADR:** [ADR-0018: Alert Rule Coverage Analysis](../adr/0018-alert-rule-coverage-analysis.md)

## Purpose

The rules API client fetches alert and recording rules from a Prometheus or
Mimir API endpoint (`GET /api/v1/rules`). This provides an alternative to
manual file upload/paste: users point Signal Studio at their Prometheus server
and rules are fetched automatically. The client also provides `MergeRules` to
deduplicate rules from multiple sources (API + file upload).

## Design

### Entry Point

```go
func FetchRules(opts ClientOptions) ([]AlertRule, error)
```

### ClientOptions

```go
type ClientOptions struct {
    URL     string        // Base URL, e.g. "http://prometheus:9090"
    Token   string        // Optional bearer token
    OrgID   string        // Optional Mimir X-Scope-OrgID header
    Timeout time.Duration // HTTP client timeout (default: 10s)
}
```

### Request Construction

1. The URL is normalized by trimming trailing slashes and appending
   `/api/v1/rules`.
2. A standard `GET` request is created via `http.NewRequest`.
3. Optional headers are set:
   - `Authorization: Bearer <token>` — if `Token` is non-empty.
   - `X-Scope-OrgID: <orgID>` — if `OrgID` is non-empty.
4. An `http.Client` is created with the configured timeout (default 10 seconds).

### Authentication

**Bearer token:** Many Prometheus/Mimir deployments sit behind a reverse proxy
that requires authentication. The client supports bearer token authentication
via the `Authorization` header. The token is passed as-is; the client does not
validate or refresh it.

**Mimir multi-tenancy:** Mimir (and Cortex) use the `X-Scope-OrgID` header to
identify the tenant whose rules should be returned. Without this header, Mimir
returns an error for multi-tenant deployments. The client sets this header when
`OrgID` is provided.

### Response Handling

The client validates the HTTP response at multiple levels:

1. **HTTP status code** — any non-200 response is treated as an error. The
   response body is read (up to 1024 bytes, via `io.LimitReader`) and included
   in the error message for debugging.

2. **JSON decoding** — the response body is decoded into a
   `prometheusRulesResponse` struct. Malformed JSON results in an error.

3. **API status field** — the Prometheus API wraps all responses in an envelope
   with a `status` field. The client checks that `status == "success"`. A
   non-success status (e.g., `"error"`) results in an error.

### Response Types

```go
type prometheusRulesResponse struct {
    Status string `json:"status"`
    Data   struct {
        Groups []prometheusAPIGroup `json:"groups"`
    } `json:"data"`
}

type prometheusAPIGroup struct {
    Name  string              `json:"name"`
    Rules []prometheusAPIRule `json:"rules"`
}

type prometheusAPIRule struct {
    Type   string `json:"type"`   // "alerting" or "recording"
    Name   string `json:"name"`
    Query  string `json:"query"`
    Alert  string `json:"alert,omitempty"`
    Record string `json:"record,omitempty"`
}
```

Note the field mapping differences from the YAML parser:
- The API returns `type` as `"alerting"` (not `"alert"`) and `"recording"`
  (not `"record"`).
- The PromQL expression is in `query` (not `expr`).

### Rule Conversion

`convertAPIRules` transforms the API response into `[]AlertRule`:

1. Iterates through all groups and rules.
2. Skips rules with empty `query` fields.
3. Maps `type`:
   - `"alerting"` becomes `Type = "alert"`.
   - `"recording"` becomes `Type = "record"`.
   - Unknown types are skipped.
4. Calls `ExtractMetrics(expr)` to parse the PromQL and extract metric names
   and the `usesAbsent` flag.
5. If `ExtractMetrics` fails, the rule is silently skipped (graceful
   degradation, same principle as the YAML parser).

### Rule Merging

```go
func MergeRules(sets ...[]AlertRule) []AlertRule
```

`MergeRules` combines multiple rule sets (e.g., API-sourced + file-based) into
a single deduplicated slice. The deduplication key is `Group + "/" + Name`.

Merge behavior:
- First occurrence wins. If the same rule appears in multiple sets, the version
  from the first set is kept.
- Rules with the same name but different groups are considered distinct. This
  matches Prometheus behavior where rule names are scoped to their group.
- The function accepts variadic `[]AlertRule` arguments, allowing any number
  of sets to be merged in a single call.

## Error Handling

| Error condition | Behavior |
|---|---|
| Invalid URL | `http.NewRequest` returns an error. |
| Network error (connection refused, DNS failure, timeout) | `client.Do` returns an error. |
| Non-200 HTTP status | Error includes status code and up to 1024 bytes of the response body. |
| Malformed JSON response | `json.Decoder` returns an error. |
| API status != "success" | Error includes the actual status string. |
| Individual rule with invalid PromQL | Rule is silently skipped. Other rules are still converted. |
| All rules have invalid PromQL | Returns empty slice and no error (differs from the YAML parser which errors when all rules fail). |

The body-read limit of 1024 bytes for error responses prevents unbounded memory
allocation from pathological responses while still capturing enough diagnostic
information.

## Edge Cases

| Scenario | Behavior |
|---|---|
| Trailing slash in URL | Trimmed before appending `/api/v1/rules`. `http://prom:9090/` becomes `http://prom:9090/api/v1/rules`. |
| Zero timeout | Defaults to 10 seconds. |
| No token or OrgID | Headers are not set. Works with unauthenticated Prometheus instances. |
| Empty query in API rule | Rule is skipped. |
| Unknown rule type in API | Rule is skipped. |
| Merge with duplicate rules | First occurrence wins, keyed by `group/name`. |
| Merge with same name, different groups | Both rules are kept (distinct keys). |
| Merge with empty sets | Empty sets contribute nothing. Non-empty sets are unaffected. |

## Testing Strategy

Tests are in `client_test.go` and use `net/http/httptest` to create mock HTTP
servers:

### Success path
- **`TestFetchRules_Success`** — mock server returns a valid response with one
  alerting and one recording rule. Verifies correct path (`/api/v1/rules`),
  rule count, names, and types.

### Authentication tests
- **`TestFetchRules_BearerToken`** — verifies `Authorization: Bearer test-token`
  header is sent when `Token` is configured.
- **`TestFetchRules_MimirOrgID`** — verifies `X-Scope-OrgID: tenant-1` header
  is sent when `OrgID` is configured.

### Error handling tests
- **`TestFetchRules_HTTPError`** — mock server returns 403 Forbidden. Verifies
  the function returns an error.
- **`TestFetchRules_MalformedJSON`** — mock server returns non-JSON body.
  Verifies JSON decode error.
- **`TestFetchRules_StatusError`** — mock server returns
  `{"status": "error"}`. Verifies API status check.

### Merge tests
- **`TestMergeRules_Dedup`** — two sets with an overlapping rule. Verifies
  deduplication (3 unique rules from 4 total).
- **`TestMergeRules_SameNameDifferentGroup`** — same rule name in different
  groups. Verifies both are kept (2 rules, not 1).

All mock servers use `httptest.NewServer` which provides real HTTP listeners on
localhost, testing the full HTTP client path including request construction,
header setting, response reading, and JSON decoding.
