# Signal Studio

A read-only diagnostic tool for OpenTelemetry Collector. Paste your Collector YAML config, connect to a live Prometheus metrics endpoint, and get actionable recommendations to reduce telemetry noise and improve pipeline health.

## Features

**Config Studio** -- Paste or edit Collector YAML and instantly see:
- Pipeline visualization (receivers, processors, exporters per pipeline)
- 18 static lint rules covering missing processors, ordering issues, security concerns, and misconfigurations
- Copy-paste YAML snippets to fix each finding

**Live Metrics** -- Connect to a Collector's Prometheus endpoint to see:
- Per-pipeline throughput (in/out rates) overlaid on pipeline cards
- Per-card component throughput (receiver accepted rates, exporter sent rates + queue utilization)
- 4 live rules that detect drop rates, log volume dominance, queue saturation, and receiver-exporter mismatches

**OTLP Sampling Tap** -- Enable with `TAP_ENABLED=true` to discover metric names flowing through your Collector:
- Lightweight OTLP gRPC + HTTP receiver extracts metric metadata (names, types, attribute keys)
- Add a fan-out exporter to your Collector config — the UI shows the exact YAML snippet to add
- Metric catalog with TTL-based expiry, accumulates across the session
- Filter analysis: predicts which metrics would be kept/dropped by `filter` processors (legacy + OTTL syntax)

**Recommendations** -- All findings (static + live) appear in a unified panel sorted by severity, with evidence, explanations, impact estimates, and remediation snippets.

## Static Rules

| ID | Severity | Description |
|---|---|---|
| `missing-memory-limiter` | critical | Pipeline has no `memory_limiter` processor |
| `memory-limiter-not-first` | critical | `memory_limiter` is not the first processor |
| `memory-limiter-without-limits` | warning | `memory_limiter` has no limit configured |
| `missing-batch` | warning | Pipeline has no `batch` processor |
| `batch-before-sampling` | warning | Batch processor runs before sampling |
| `batch-not-near-end` | warning | Batch processor is not near the end of the chain |
| `no-trace-sampling` | warning | Trace pipeline has no sampling configured |
| `no-log-severity-filter` | info | Log pipeline has no severity filtering |
| `filter-error-mode-propagate` | warning | Filter/transform uses `error_mode: propagate` |
| `receiver-endpoint-wildcard` | warning | Receiver binds to `0.0.0.0` |
| `debug-exporter-in-pipeline` | info | Debug/logging exporter present |
| `pprof-extension-enabled` | info | pprof extension is enabled |
| `exporter-no-sending-queue` | warning | Network exporter has no sending queue |
| `exporter-no-retry` | warning | Network exporter has no retry on failure |
| `undefined-component-ref` | critical | Pipeline references undefined component |
| `empty-pipeline` | critical | Pipeline has no receivers or exporters |
| `unused-components` | info | Component defined but not used in any pipeline |
| `multiple-exporters-no-routing` | info | Multiple exporters without a routing processor |

## Live Rules

| ID | Severity | Condition |
|---|---|---|
| `live-high-drop-rate` | warning | >10% drops sustained over 2+ intervals |
| `live-log-volume-dominance` | info | Log ingest rate exceeds 3x trace rate |
| `live-queue-near-capacity` | warning | Exporter queue >80% utilized, sustained |
| `live-receiver-exporter-mismatch` | warning | Accepted rate >2x sent rate, sustained over 3+ intervals |

## Quick Start

### Docker

```sh
docker build -t signal-studio .
docker run -p 8080:8080 signal-studio
```

Open http://localhost:8080.

### Local Development

**Backend** (Go 1.24+):

```sh
cd backend
go run ./cmd/server
```

The API server starts on `:8080`.

**Frontend** (Node 22+):

```sh
cd frontend
npm install
npm run dev
```

Vite starts on `:5173` and proxies `/api` requests to the backend.

## Configuration

Environment variables for the backend:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `SCRAPE_INTERVAL_SECONDS` | `10` | Metrics polling interval (5-30) |
| `MAX_YAML_SIZE_KB` | `256` | Maximum YAML body size |
| `CORS_ORIGINS` | `*` | Allowed CORS origins (comma-separated) |
| `TAP_ENABLED` | `false` | Enable the OTLP sampling tap on startup |
| `TAP_GRPC_ADDR` | `:4317` | gRPC listen address for the OTLP tap |
| `TAP_HTTP_ADDR` | `:4318` | HTTP listen address for the OTLP tap |

## API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/config/analyze` | Parse YAML and return config + findings |
| `POST` | `/api/metrics/connect` | Start scraping a Prometheus endpoint |
| `POST` | `/api/metrics/disconnect` | Stop scraping |
| `GET` | `/api/metrics/snapshot` | Latest computed rates and queue data |
| `GET` | `/api/metrics/status` | Connection status |
| `POST` | `/api/tap/start` | Start OTLP sampling tap |
| `POST` | `/api/tap/stop` | Stop tap |
| `GET` | `/api/tap/status` | Tap status + window timing |
| `GET` | `/api/tap/catalog` | Discovered metric names |
| `GET` | `/api/health` | Health check |

## Project Structure

```
signal-studio/
├── backend/
│   ├── cmd/server/          # HTTP server entrypoint
│   └── internal/
│       ├── api/             # HTTP handlers + routing
│       ├── config/          # YAML parser + data model
│       ├── filter/          # Filter config parser + matcher
│       ├── metrics/         # Prometheus scraper + store
│       ├── rules/           # Static and live rule engine
│       └── tap/             # OTLP sampling tap + metric catalog
├── frontend/
│   └── src/
│       ├── components/      # React components
│       ├── hooks/           # Custom hooks
│       └── types/           # TypeScript types mirroring backend
├── docs/adr/                # Architecture decision records
└── Dockerfile               # Multi-stage production build
```

## Tech Stack

- **Backend**: Go, `gopkg.in/yaml.v3`, `github.com/prometheus/common/expfmt`
- **Frontend**: React 19, TypeScript, Vanilla Framework (Canonical CSS), Monaco Editor, Vite
- **Deployment**: Single container, stateless, in-memory only

## License

MIT
