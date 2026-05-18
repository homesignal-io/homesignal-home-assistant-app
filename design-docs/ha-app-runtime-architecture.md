# Home Assistant App Runtime Architecture

Status: implementation guide for the local HomeSignal app.

This document unifies the local app runtime, mounted UI, local logging, and
cloud-authorized log retrieval contract. It should be read before refactoring
the current app skeleton or moving the HomeSignal Manager mock into the real
app.

## Target Shape

The app is one local product surface with four internal layers:

```text
Home Assistant ingress
  -> static HomeSignal Manager UI
  -> local UI API adapter
  -> Go app runtime services
  -> local state, Supervisor/Core APIs, HomeSignal cloud runtime APIs
```

The Go runtime owns local authority. The React UI renders local state and calls
local routes. The cloud can request bounded actions only through the claimed
device command/runtime contract, and the app remains the final local gate.

## Go Runtime Package Boundaries

Refactor toward these boundaries as the app grows:

| Boundary | Responsibility |
| --- | --- |
| `cmd/agent` | Process wiring only: config load, logger setup, service construction, HTTP server start/shutdown. |
| `internal/enrollment` | Local identity, claim state, pairing verification/finalization, credential persistence. |
| `internal/uiapi` | Page-ready local view models for Status, Pairing, Permissions, Advanced, and static UI serving. |
| `internal/supervisor` | Home Assistant Supervisor/Core clients and capability probes. |
| `internal/policy` | Local Management Permission Catalog and locally persisted policy revisions. |
| `internal/logging` | Structured logger, local ring store, redaction, level controls, bundle generation, suppression counters. |
| `internal/commands` | Claimed-device command notice handling, allowlist, ACK/result reporting. |
| `internal/artifacts` | Generated artifact registry, brokered upload validation, upload completion. |
| `internal/telemetry` | Health snapshots, runtime log summaries, policy hash/revision reporting. |

Do not put business behavior in `cmd/agent/main.go` or frontend components.
Frontend components should not know whether facts came from `/status`,
`/readyz`, Supervisor API, local storage, telemetry cache, or cloud metadata.

## UI Package Boundaries

The mounted UI should live under an app-owned frontend package, for example:

```text
homesignal/ui/
  src/
    app/
    adapters/
    components/ha/
    features/status/
    features/pairing/
    features/permissions/
    features/advanced/
    contracts/
    assets/
```

Rules:

- `components/ha` holds generic Home Assistant-style controls only.
- `features/*` owns page composition and page-specific copy.
- `adapters/mockAppAdapter` is for design review and tests.
- `adapters/httpAppAdapter` is for the real app.
- `contracts` holds TypeScript types that mirror local UI API responses.
- `assets` holds only app UI assets; cloud portal assets stay outside the app.
- The public cloud pairing page remains cloud/web owned. The app may load it
  in a hidden iframe bridge, but it should not duplicate that page.

## Cloud Environment Profile

The app must not accept arbitrary HomeSignal cloud base URLs from a visible
field, URL parameter, or browser storage. Runtime cloud targets come from an
allowlisted environment profile shipped with the build or enabled by local
development-only config.

Allowed launch profiles:

- `production`
- `staging`, only in staging/dev builds or explicit local dev config

Changing the cloud profile after pairing requires unpairing, clearing durable
device credentials, and claiming again because credentials are
environment-scoped.

Generated AWS endpoints are valid only for Stage 0 deployment smoke tests. Real
staging pairing, public pairing pages, hidden iframe bridge behavior,
localStorage/postMessage origin state, and email links require a stable owned
HTTPS HomeSignal domain.

Home Assistant app environment/profile selection is also a repository/release
track concern. Production stable, production candidate, and staging/non-prod
packages are defined in `ha-app-repository-release-strategy.md`; the runtime
should report its installed slug, profile, track, and version rather than expose
an end-user cloud URL selector.

## Local UI API

The UI API returns page-ready view models. It should avoid forcing the frontend
to derive authority, policy, or health state.

Minimum local routes:

| Route | Purpose |
| --- | --- |
| `GET /api/ui/status` | Status page view model, including managed-by metadata, health summary, update notice, and saved permissions summary. |
| `GET /api/ui/readiness` | Local Supervisor/Core/API reachability and degraded reasons. |
| `GET /api/ui/permissions` | Current local management policy and catalog. |
| `PUT /api/ui/permissions` | Save explicit local permission values and increment local policy revision. |
| `POST /api/ui/pairing/verify` | Verify pairing code through local runtime and HomeSignal claim verification API. |
| `POST /api/ui/pairing/confirm` | Confirm pairing with verification token and explicit local permissions. |
| `POST /api/ui/unpair` | Local cloud-independent unpair/reset. |
| `GET /api/ui/logging` | Current local logging configuration and storage status. |
| `GET /api/ui/logging/tail` | Read-only bounded recent log entries for the local tail view. |
| `PUT /api/ui/logging` | Local-only logging setting changes allowed by policy. |

`GET /status`, `GET /readyz`, and `GET /version` remain machine-friendly
runtime endpoints. `/api/ui/*` is allowed to compose those facts into UI-ready
responses.

`GET /api/ui/logging/tail` should support bounded query parameters such as
`limit`, `min_level`, `component`, `reason_code`, and `since`. Default limit:
200 entries. Maximum limit: 1000 entries. It reads only from the managed
diagnostic ring and never accepts local file paths.

## Local Logging Model

The app needs two separate logging paths:

1. **Operational stdout logs:** structured process logs emitted to container
   stdout for Home Assistant/Supervisor visibility.
2. **Local diagnostic log store:** bounded on-disk ring store used for local UI
   status, runtime summaries, and cloud-authorized diagnostic bundles.

The local diagnostic log store is not a raw unlimited file. It is a managed
ring with a fixed budget.

Default local storage:

```text
/config/logs/
  active/
    segment-000.ndjson
    segment-001.ndjson
    ...
  index.json
  registry.json
```

Defaults:

- total local diagnostic log budget: `32 MiB`
- segment size: `4 MiB`
- segment count: `8`
- startup behavior: ensure directory exists and enforce budget before writing
- overflow behavior: overwrite/delete oldest segment first
- high-priority error metadata: retained in `registry.json` with bounded counts
- disk pressure behavior: stop writing diagnostic logs, keep stdout logging, set
  degraded logging status, and report one collapsed warning

The implementation may use segmented NDJSON files first. A preallocated circular
file is allowed later if it materially improves disk behavior, but the external
contract remains the same: fixed local budget, oldest data discarded first, and
no unbounded growth.

## App-Only Logging Boundary

The diagnostic log store is for **HomeSignal Manager app runtime logs only**.

It must not mirror, import, or tail:

- Home Assistant Core logs
- Home Assistant Supervisor logs
- host/syslog logs
- logs from other apps
- arbitrary files from mapped directories

The app can log observations it makes while operating. For example, "local
readiness probe failed" is valid because HomeSignal Manager observed the
failure. A copied Home Assistant stack trace is not valid unless it is captured
as a bounded, redacted diagnostic artifact through an explicit future flow.

Use this distinction:

- **Source:** HomeSignal Manager app runtime.
- **Subject:** cloud connection, pairing, permissions, commands, telemetry,
  update posture, local readiness probes, or logging itself.

Recommended component values:

- `startup`
- `config`
- `enrollment`
- `cloud_connection`
- `local_probe`
- `permissions`
- `commands`
- `telemetry`
- `updates`
- `logging`
- `artifacts`

## Log Levels

Use three distinct controls so local diagnostics, routine cloud telemetry, and
temporary support capture do not collapse into one unsafe "debug" switch.

| Control | Owner | Values | Purpose |
| --- | --- | --- | --- |
| Local capture level | app local config | `error`, `warning`, `info`, `debug` | What the local diagnostic ring may store. Default `info`. |
| Routine cloud publish verbosity | cloud publish policy | `quiet`, `normal`, `verbose` | What summaries may leave the device routinely. Default from publish policy. |
| Temporary debug capture | cloud-authorized command | session with TTL | Time-boxed richer capture for support/debug flows. Not standing config. |

Rules:

- Local `debug` capture can be enabled locally, but cloud upload still requires
  an authorized artifact/debug flow.
- Routine cloud publishing never becomes a raw log stream.
- The app must report the active local capture level, routine publish policy
  version, and any active debug session in health telemetry.
- Secrets, tokens, private keys, claim/pairing codes, signed URLs, cookies, raw
  Home Assistant config, and arbitrary file contents must be redacted before
  writing to the local diagnostic store.

Runtime event level semantics:

| Level | Meaning | Examples |
| --- | --- | --- |
| `error` | An app runtime feature failed or cannot complete without intervention/retry. | Credential read failure, pairing confirm failure, command execution failure, artifact bundle generation failure. |
| `warning` | The app detected degraded behavior, unusual state, or self-recovered trouble. | Cloud reconnect backoff, command rejected by local policy, stale update posture, log suppression, storage pressure. |
| `info` | Meaningful app lifecycle or operational progress. | Startup complete, credential loaded, pairing verified, permissions saved, health snapshot sent, cloud connected. |
| `debug` | Early-days troubleshooting detail that should normally stay local and time-bounded. | State transitions, retry counters, dedupe decisions, local probe summaries, log GC decisions. |

## Logging Hooks

Every runtime boundary should log structured events through the local logging
service, not ad hoc `fmt.Println` or direct file writes.

Required hook points:

- process startup/shutdown
- options/config load and validation
- local state migration
- enrollment state changes
- pairing verify/confirm attempts and failures
- Supervisor/Core API probe failures
- cloud connectivity changes
- command received/accepted/rejected/result
- publish-policy apply success/failure
- artifact upload negotiation/upload/result
- local policy save/change
- local unpair/reset
- logging GC/drop/suppression events

Each log event should include:

- timestamp
- level
- component
- reason code
- message
- correlation IDs when available
- safe context fields

Preferred stored shape:

```json
{
  "timestamp": "2026-05-14T12:00:00Z",
  "level": "info",
  "component": "telemetry",
  "reason_code": "health_snapshot_sent",
  "message": "Health snapshot sent.",
  "request_id": "req_01J00000000000000000000000",
  "device_id": "dev_123",
  "details": {}
}
```

The logging service should also maintain collapsed counters by component,
reason code, and time window for `runtime_log_summary[]` telemetry.

## Cloud Log Request Contract

Cloud-requested internal app runtime logs use the existing artifact split pattern.
The cloud does not directly fetch local files, and the app does not upload
logs unsolicited.

Flow:

```text
1. App emits routine health/runtime summary with optional local_artifact_ref.
2. Authorized cloud user/service requests logs or diagnostics.
3. Command Service creates an allowlisted command.
4. AWS IoT sends tiny command notice: { command_id, type: "upload_artifact" }.
5. App fetches command details over Agent HTTPS mTLS.
6. App validates purpose, local ref/query, size, TTL, redaction profile.
7. App generates a redacted bundle from the local diagnostic store.
8. App requests artifact upload session over Agent HTTPS.
9. App uploads to object storage and reports completion/result.
```

Allowed initial purpose:

```text
error_log_bundle
```

Allowed command detail selectors:

- `local_artifact_ref`
- bounded time window
- minimum level
- component allowlist
- reason-code allowlist
- max bytes
- redaction profile

Disallowed selectors:

- arbitrary local file path
- shell command output
- broad Home Assistant config dump
- unbounded "all logs"
- secrets or credential material

Upload failures follow the recursion guard in
`app-runtime-error-and-artifact-contract.md`.

## Local Logging UI

Add logging as a sub-section of Advanced first. Promote to a separate page only
if the surface grows.

Advanced should show:

- local capture level
- local log storage budget and used bytes
- oldest/newest retained log time
- dropped/suppressed count
- active debug session, if any
- last cloud log request/result
- read-only recent log tail
- button to clear local diagnostic logs

The local log tail is for inspection, not support upload. It should be
designed like a readable `tail -f` view:

- label the section `HomeSignal Manager logs`
- state clearly that these are app runtime logs, not Home Assistant system
  logs
- default to the last 200 retained entries
- show timestamp, level, component, reason code, and message
- color by level without turning the whole view into an alert panel
- support search/filter, pause/resume, line wrapping, and copy visible or
  selected lines
- apply the same redaction rules used for generated log bundles
- never include a "send logs" or "upload logs" button

Cloud log requests should be visible locally as recent activity, but the local
admin does not need to approve every bounded v0 diagnostic/error-log bundle if
the local policy allows it and cloud authorization succeeds.

Cloud log retrieval is intentionally separate from the local tail. The cloud
pulls logs through the authorized `error_log_bundle` artifact path; the UI may
show the last request and result, but it should not initiate the upload.

## Implementation Defaults

- Keep local diagnostic logs under `/config/logs`, not `/data`, so they survive
  app restarts with other app-owned state.
- Use JSON/NDJSON internally so bundles can be filtered and redacted
  deterministically.
- Keep stdout logging even if the diagnostic ring is degraded.
- Treat storage budget failure as degraded observability, not app startup
  failure.
- Log bundle generation must never block the main agent loop indefinitely; use
  context deadlines.
- Log bundle max size should follow artifact defaults: `5 MiB` for
  `error_log_bundle`.

## Acceptance Criteria

- App UI is componentized by local feature and can be mounted without the
  broader cloud mock.
- Local UI reads through adapters and does not own runtime authority.
- Local diagnostic logs have a fixed budget and deterministic overwrite/GC.
- Logging levels are explicit and separated by local capture, routine publish
  verbosity, and temporary debug capture.
- Cloud can request internal app runtime logs only through an allowlisted command and
  brokered artifact upload flow.
- The app can generate a redacted log bundle without exposing arbitrary local
  files or secrets.
- Routine telemetry includes bounded log summaries and suppression counters,
  not raw log streams.
