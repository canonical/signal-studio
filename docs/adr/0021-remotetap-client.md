# 0021 — Remote Tap Client

## Status

Accepted

## Context

Signal Studio currently supports a **passive OTLP tap**: the collector is configured to push
telemetry to Signal Studio's gRPC/HTTP ingest endpoint (`/v1/metrics`, `/v1/traces`, `/v1/logs`).
This works well but requires modifying the collector config.

Users running a collector on Kubernetes with the `remotetap processor` already configured want
to stream telemetry directly into Signal Studio without:

1. Reconfiguring the k8s collector, or
2. Running a local OTel Collector bridge process.

The `remotetap processor` (from `opentelemetry-collector-contrib`) exposes a **WebSocket server**
(default port `12001`) that streams OTLP JSON payloads for metrics, traces, and logs to any
connected client. Each WebSocket message is a full JSON-encoded OTLP export request — either
`{"resourceMetrics":[...]}`, `{"resourceSpans":[...]}`, or `{"resourceLogs":[...]}`.

## Decision

Add an active **remotetap client** to Signal Studio's backend that:

- Accepts a WebSocket endpoint address from the user via a new UI section in the tap popout.
- Connects outbound to `ws://<addr>` (normalising bare `host:port` inputs automatically).
- Reads the streaming OTLP JSON messages in a background goroutine.
- Detects signal type by inspecting the top-level JSON key (`resourceMetrics` / `resourceSpans`
  / `resourceLogs`) and unmarshals with the appropriate `pdata` JSON unmarshaler.
- Feeds the data into the **same metric, span, and log catalogs** as the existing passive tap, so
  no additional catalog display logic is needed.
- Shuts down cleanly on context cancellation (the WebSocket connection is closed, which causes
  the blocking `ReadMessage` to return).

Two new API endpoints are added:

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/tap/remotetap/connect` | Start the remotetap client; body: `{"addr":"host:port"}` |
| `POST` | `/api/tap/remotetap/disconnect` | Stop the remotetap client |

The existing `/api/tap/status` response is extended with a `remotetap` object:

```json
{
  "remotetap": {
    "status": "connected",
    "addr":   "localhost:12001",
    "error":  ""
  }
}
```

The `tap.Manager` gains a `remoteTap *remoteTapClient` field (initialised in `NewManager`) and
three new methods: `ConnectRemoteTap`, `DisconnectRemoteTap`, and `RemoteTapStatus`.

The passive OTLP tap and the remotetap client are **independent**: both can run simultaneously
and both populate the same catalogs. The prune goroutine for the catalogs is embedded in the
remotetap client's run loop (mirroring the existing `pruneLoop` approach), so catalog TTL
eviction works regardless of which source is active.

`github.com/gorilla/websocket` is added as a direct dependency for the WebSocket dial/read
implementation.

## Consequences

- Adds one direct dependency: `github.com/gorilla/websocket`.
- No changes to the collector-side configuration required.
- The catalog UI (`MetricCatalogPanel`) and catalog-based rules continue to work unchanged.
- The tap status API response gains a new `remotetap` field; existing clients that ignore
  unknown fields are unaffected.
- Reconnection on drop is left to the user (they click Connect again); no auto-reconnect.
