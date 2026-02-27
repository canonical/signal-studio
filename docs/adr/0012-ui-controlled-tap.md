# ADR-0012: UI-Controlled Tap with TAP_DISABLED Opt-Out

**Status:** Accepted
**Date:** 2026-02-25
**Related:** [ADR-0006: Metric Name Discovery and Filter Analysis](0006-metric-name-discovery-and-filter-analysis.md)

---

## Context

The OTLP sampling tap (ADR-0006) required setting `TAP_ENABLED=true` at server startup. This created unnecessary friction: users had to restart the backend to try the feature. The tap is a core part of the Signal Studio experience, and making it opt-in by default reduced discoverability.

---

## Decision

Flip the tap from opt-in to opt-out:

1. **The tap is always available by default.** The UI provides start/stop controls so users can enable the tap on demand without restarting the server.
2. **Operators can disable the tap entirely** by setting `TAP_DISABLED=true`. When disabled, the manager rejects start requests and the frontend shows a disabled state with no start button.
3. **The `TAP_ENABLED` env var is removed.** The auto-start-on-boot behavior is removed; the UI is now the primary control surface.

---

## Changes

### Backend

- `tap.NewManager(disabled bool)` — accepts a disabled flag. When disabled, `Status()` returns `"disabled"` and `Start()` returns an error.
- `TapStatusDisabled` — new status constant.
- `main.go` — reads `TAP_DISABLED` env var instead of `TAP_ENABLED`, removes auto-start block.
- `tap_handler.go` — `handleStart` reads optional `grpcAddr`/`httpAddr` from JSON request body, falling back to `TAP_GRPC_ADDR`/`TAP_HTTP_ADDR` env vars, then defaults (`:5317`/`:5318`). `handleStatus` includes `"disabled": true` when the tap is disabled.

### Frontend

- `TapStatus` type extended with `"disabled"`.
- `useTap` hook exposes `start()` and `stop()` callbacks.
- `TapConnection` component: idle state shows a "Start tap" button, listening state shows a "Stop tap" button, disabled state shows an explanatory message with no controls.

---

## Consequences

- **Lower friction:** Users can try the tap immediately from the UI without any server configuration.
- **Operator control preserved:** `TAP_DISABLED=true` provides a hard opt-out for environments where the tap should not be available.
- **Breaking change:** `TAP_ENABLED=true` no longer works. Operators using it must remove it (tap is now on by default) or switch to `TAP_DISABLED=true` if they want to prevent usage.
- **`tap.NewManager` signature change:** All callers must pass the `disabled` boolean.
