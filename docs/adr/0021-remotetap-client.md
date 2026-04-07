# ADR-0021: Remote Tap Client — Connect to remotetap processor

**Status:** Proposed
**Date:** 2026-04-07
**Related:** [ADR-0006: Metric Name Discovery and Filter Analysis](0006-metric-name-discovery-and-filter-analysis.md), [ADR-0012: UI-Controlled Tap](0012-ui-controlled-tap.md), [ADR-0013: Multi-Signal Tap](0013-multi-signal-tap.md)

---

## Context

Signal Studio's OTLP sampling tap (ADR-0006, ADR-0012) is a **passive receiver**: the Collector must be configured to push telemetry to Signal Studio's gRPC/HTTP ingest endpoint (`/v1/metrics`, `/v1/traces`, `/v1/logs`). This requires a Collector config change that is not always possible or desirable.

The `remotetap processor` from `opentelemetry-collector-contrib` exposes a **WebSocket server** (default port `12001`) that streams OTLP JSON payloads for all three signal types to any connected client. Each WebSocket message is a complete OTLP export request encoded as JSON — `{"resourceMetrics":[...]}`, `{"resourceSpans":[...]}`, or `{"resourceLogs":[...]}`.

If Signal Studio could act as a WebSocket client, it could consume the tap stream without any Collector reconfiguration.

---

## Problem Breakdown

### Passive tap requires Collector reconfiguration

The passive tap requires inbound connectivity from the Collector to Signal Studio, meaning the Collector config must be modified to add a fan-out OTLP exporter. Alternatively, a local Collector bridge process can be run to forward the remotetap processor stream to Signal Studio's ingest endpoint. Both approaches require extra steps beyond Signal Studio itself.

### Existing catalog infrastructure is reusable

Signal Studio already maintains three in-memory catalogs (`Catalog`, `SpanCatalog`, `LogCatalog`) that the passive tap populates. If a remotetap client can feed the same catalogs, the entire downstream stack — the catalog API, the `MetricCatalogPanel`, catalog-based rules, and filter analysis — works unchanged.

### Protocol is simple and already partially supported

The remotetap processor marshals OTLP data using the same `pdata` JSON marshalers already used in Signal Studio's HTTP tap handlers. The signal type is identifiable from the top-level JSON key. No new deserialization code is needed beyond detecting which unmarshaler to invoke.

---

## Approaches

### A. Run a local OTel Collector bridge

Run a minimal local `otelcol-contrib` instance with `remotetapreceiver` → `otlpexporter` pointing at Signal Studio's ingest port.

**Pros:**
- Zero changes to Signal Studio
- Leverages the existing passive tap endpoint

**Cons:**
- Requires installing and running a separate `otelcol-contrib` process
- Users need to understand OTel Collector config just to connect

### B. Add an active remotetap client to Signal Studio

Implement a WebSocket client inside the Signal Studio backend that connects outbound to a user-specified remotetap processor endpoint, reads the OTLP JSON stream, and feeds data into the existing catalogs. Expose connect/disconnect controls in the tap popout UI alongside the existing passive tap toggle.

**Pros:**
- No additional processes or tools required
- Reuses all existing catalog infrastructure unchanged
- Single UI surface for both tap modes

**Cons:**
- Requires the remotetap processor to be configured on the Collector
- Adds one new Go dependency (`gorilla/websocket`) for WebSocket dialing
- The tap popout UI becomes slightly more complex (two independent tap modes)

---

## Recommendation: Option B — Active remotetap client

Option B eliminates the multi-process friction while reusing virtually all existing infrastructure. The implementation cost is modest: a WebSocket dial/read loop, signal-type detection, and two new API endpoints. The dependency addition (`gorilla/websocket`) is minimal and well-established.

Option A remains valid as a fallback for users who prefer the bridge approach, but it should not be the primary supported workflow.

---

## Design

### WebSocket client

A new `remoteTapClient` struct in `internal/tap` dials the remotetap processor endpoint and streams messages in a background goroutine:

```go
type remoteTapClient struct {
    catalog     *Catalog
    spanCatalog *SpanCatalog
    logCatalog  *LogCatalog
    cancel      context.CancelFunc
    doneCh      chan struct{}
    // status, addr, mu
}
```

The run loop:

1. Dials `ws://<addr>` (bare `host:port` inputs are normalised automatically).
2. Closes the WebSocket connection when the context is cancelled, causing `ReadMessage` to return and the loop to exit cleanly.
3. For each message, peeks at the top-level JSON key to determine the signal type, then unmarshals with the appropriate `pdata` JSON unmarshaler and calls the matching `extractAndRecord*` function.
4. Runs a catalog prune goroutine for the duration of the connection so TTL eviction works even when the passive OTLP tap is not running.

Signal-type detection:

```go
type signalPeek struct {
    ResourceMetrics json.RawMessage `json:"resourceMetrics"`
    ResourceSpans   json.RawMessage `json:"resourceSpans"`
    ResourceLogs    json.RawMessage `json:"resourceLogs"`
}
```

### Manager extensions

`tap.Manager` gains a `remoteTap *remoteTapClient` field (initialised in `NewManager`) sharing the same three catalog instances. Three new methods are added:

```go
func (m *Manager) ConnectRemoteTap(addr string) error
func (m *Manager) DisconnectRemoteTap()
func (m *Manager) RemoteTapStatus() (RemoteTapStatus, string, string)
```

The passive OTLP receiver and the remotetap client are independent — both can run simultaneously and both populate the same catalogs.

### API endpoints

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

`RemoteTapStatus` follows the same state vocabulary as `TapStatus`: `idle`, `connecting`, `connected`, `error`.

### Frontend

The tap popout gains a "Remote tap" section below the existing OTLP tap section, separated by a divider. It uses the same toggle-button design (`tap-popout__toggle-btn` + `ToggleIcon`) as the OTLP tap row, with an endpoint input field (styled consistently with the Metrics endpoint popout) that is disabled while connected.

The Signal Catalog is enabled when **either** the passive OTLP tap or the remotetap client is active:

```tsx
tapActive={tap.status === "listening" || tap.remotetap.status === "connected"}
```

No reconnection on drop — the user reconnects manually from the UI.

---

## Future Work (Out of Scope)

- **TLS support** — allowing `wss://` endpoints for remotetap processors behind TLS termination.
- **Auto-reconnect** — reconnecting automatically after a connection drop with exponential backoff.
- **Address persistence** — storing the last-used remotetap address in `localStorage`, similar to the metrics endpoint URL.
- **Remotetap-based catalog rules** — rules that fire specifically when data is arriving via remotetap (e.g. detecting that the remotetap rate limit is dropping messages at high throughput).

---

## Consequences

### Positive

- Signal Studio can connect to a remotetap processor without any Collector config changes or additional processes
- The entire existing catalog stack — API, UI, filter analysis, catalog rules — works unchanged
- Single new dependency (`gorilla/websocket`) with no transitive dependencies

### Negative

- The tap popout becomes slightly more complex with two independent tap modes
- No auto-reconnect: a dropped WebSocket connection requires a manual reconnect from the UI
- The remotetap processor's built-in rate limiting (default: 1 message/second) may cause Signal Studio to miss telemetry at high throughput — this is a limitation of the remotetap processor itself, not this implementation
