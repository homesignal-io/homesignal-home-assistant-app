# Home Assistant App UI Mount Plan

Status: implementation handoff prep.

Scope: prepare the HomeSignal Manager mock in `design-mock/src/App.jsx` to
become the real Home Assistant app ingress UI served by the app under
`homesignal/`.

This document translates the settled app UX and claim contract into a mount
plan for implementation. The unified local runtime, UI, logging, and cloud log
request architecture lives in `design-docs/ha-app-runtime-architecture.md`.

## Target

The real app should serve the HomeSignal Manager UI as a static React bundle
inside Home Assistant ingress. The Go app remains the local runtime and API
owner.

```text
Home Assistant ingress
  -> HomeSignal app HTTP server
  -> static React UI
  -> relative local app API calls
  -> Go runtime / local storage / Supervisor API / HomeSignal cloud
```

The UI must feel like a local Home Assistant app panel, not the HomeSignal
cloud app.

## Design Fidelity Gate

For the Home Assistant app, `design-mock/src/App.jsx` is the product source of
truth, not a mood board. The mounted app UI should be a copy/paste-grade port
of the mock surface listed below.

Implementation must preserve:

- information architecture, page order, and navigation labels
- visible copy, button labels, badges, empty states, and warning text
- component density, spacing, hierarchy, and Home Assistant-style control shape
- status, loading, degraded, paired, unpaired, revoked, and action-required
  states
- responsive behavior at Home Assistant ingress widths

Acceptable deviations are limited to real Home Assistant ingress/runtime
constraints, accessibility fixes, or replacing mock data with live adapter
data. Each deviation must be logged in the implementation notes with the reason
and the closest matching behavior from the mock. "Inspired by the mock" is not
complete for the Home Assistant app.

## What Moves From The Mock

Move only the Home Assistant app surface:

- `HomeSignal Manager` shell
- `Status`
- `Pairing`
- `Permissions`
- `Advanced`
- Home Assistant-style navigation
- update notice / action-required notice
- managed-by card
- health status drawer
- pairing code verification and approval flow
- local management permission component
- local unpair confirmation pattern

Do not move the broader cloud dashboard, sites, devices, internal admin, schema
coverage, or public portal pairing page into the app bundle.

The public `/ha_app_pairing` page belongs to the cloud/web app later. The app
may keep the hidden iframe bridge behavior, but it should load the cloud-owned
public page, not a second copy of that page from inside the app.

The public pairing page and hidden bridge depend on browser origin state. For
real staging and production, that origin must be a stable owned HTTPS
HomeSignal domain. Generated AWS endpoints are acceptable for Stage 0 smoke
testing only and must not be encoded into durable app environment config.

## Build And Serve

Recommended first mount:

1. Extract the app UI into an app-owned frontend directory, for example:

   ```text
   homesignal/ui/
   ```

2. Build it with Vite using relative assets. The mock now has
   `design-mock/vite.config.js` with `base: "./"` for this reason.

3. Copy or build the output into the app image, for example:

   ```text
   homesignal/ui/dist/
   ```

4. Replace the current inline Go `uiHTML` template with a static file server.
   The Go server should still own `/healthz`, `/readyz`, `/status`, `/version`,
   and the local API routes.

5. Serve the app shell for `/`, `/ui`, and frontend subpaths so refresh does not
   break inside Home Assistant ingress.

The static bundle should use relative API URLs such as `./api/ui/status` or
`./status` rather than absolute host URLs. Home Assistant ingress may rewrite
the external path.

## App API Adapter

The React UI should not read global mock objects. It should call an adapter with
this shape.

```ts
type AppUiAdapter = {
  getStatus(): Promise<AppStatusView>;
  getReadiness(): Promise<AppReadinessView>;
  getVersion(): Promise<AppVersionView>;
  getPermissions(): Promise<LocalManagementPolicyView>;
  savePermissions(policy: LocalManagementPolicyInput): Promise<LocalManagementPolicyView>;
  verifyPairingCode(code: string): Promise<PairingVerificationView>;
  confirmPairing(input: PairingConfirmInput): Promise<PairingSuccessView>;
  unpairLocal(): Promise<LocalUnpairResult>;
  getLoggingStatus(): Promise<AppLoggingStatusView>;
  getLogTail(query?: AppLogTailQuery): Promise<AppLogTailView>;
  saveLoggingSettings(input: AppLoggingSettingsInput): Promise<AppLoggingStatusView>;
};
```

Implementation default:

- `mockAppAdapter` remains in the design mock.
- `httpAppAdapter` is used by the mounted app UI.
- Components receive adapter data as props or through a small provider.
- The UI should not know whether a value came from `/status`, `/readyz`,
  Supervisor API, or cached local storage.

## Minimum Local Routes

Existing routes:

| Route | Keep | Purpose |
| --- | --- | --- |
| `GET /healthz` | yes | process liveness |
| `GET /readyz` | yes | local readiness/degraded state |
| `GET /status` | yes | local enrollment and non-secret runtime metadata |
| `GET /version` | yes | build metadata |
| `GET /ui` | replace internals | serve the React app shell |

New UI-facing routes:

| Route | Purpose |
| --- | --- |
| `GET /api/ui/status` | status page view model assembled by the app |
| `GET /api/ui/readiness` | local Home Assistant/Supervisor/readiness view model |
| `GET /api/ui/permissions` | current locally persisted management policy |
| `PUT /api/ui/permissions` | save local management policy |
| `POST /api/ui/pairing/verify` | verify a pairing code with HomeSignal cloud |
| `POST /api/ui/pairing/confirm` | commit pairing with selected local permissions |
| `POST /api/ui/unpair` | local unpair/reset, cloud-independent |
| `GET /api/ui/logging` | local logging configuration and storage status |
| `GET /api/ui/logging/tail` | read-only bounded recent local log entries |
| `PUT /api/ui/logging` | local-only logging setting changes allowed by policy |

The UI-facing routes can initially wrap the existing status/readiness structs,
but they should eventually return page-ready view models so the frontend does
not re-derive business rules.

## Pairing Flow Contract

The mounted UI flow is:

```text
Step 1: enter pairing code
  - user pastes code, or browser bridge provides code from cloud page localStorage

Step 2: verify code and review request
  - app calls HomeSignal claim verification API through local Go runtime
  - UI shows organization, requester, requester email, site, and pairing code
  - UI shows local management permission selection

Step 3: approve and pair
  - app sends verification token plus explicit local permissions
  - backend finalizes claim
  - app stores durable local credentials

Step 4: success
  - UI shows paired/managed-by details
```

The frontend never treats browser localStorage as authority. It is only a
convenience source for the pairing code.

## Permission Source Of Truth

The permission list should be derived from the Local Management Permission
Catalog in `design-docs/home-assistant-app-backend-reconciliation.md`.

The UI can offer presets:

- Grant full permissions
- Choose custom permissions

But persisted and submitted policy must remain an explicit permission list, not
a single full-access boolean.

Status-page access chips must render from the saved/agreed policy, not the
unsaved draft policy on the Permissions page.

## Data Ownership

| UI area | Source of truth |
| --- | --- |
| claim state | `/config/device.json` through app runtime |
| pairing code input | user input or browser bridge convenience value |
| verified claim details | HomeSignal claim verification API response |
| durable device identity | confirmed claim response persisted by app |
| managed-by display | local non-secret claim metadata, later refreshed by `claim_welcome` |
| update notice | app version plus desired/latest version policy |
| health status | latest local/runtime facts and cloud-accepted telemetry metadata |
| permissions | locally persisted Local Management Permission Catalog policy |
| logging status | local logging service, bounded diagnostic log ring, and active debug session metadata |
| unpair | local app runtime; must work without cloud |

## Security Rules

- Do not expose device tokens, private keys, poll tokens, certificate contents,
  or temporary AWS claim material to the UI.
- Pairing code verification and confirmation are POST-only local API calls.
- Browser localStorage is convenience state only.
- Unknown permission keys do not grant authority.
- Local unpair preserves `installation_id` by default.
- The app runtime remains the final local policy gate for Home Assistant
  actions.
- The UI may show local logging status, but cloud log retrieval must use the
  command/artifact contract in `ha-app-runtime-architecture.md` and
  `app-runtime-error-and-artifact-contract.md`.

## Implementation Steps

1. Capture reference screenshots from the mock for Status, Pairing,
   Permissions, Advanced, drawers/dialogs, and degraded/error states.
2. Extract the HA app UI components from `design-mock/src/App.jsx` into a
   small frontend package without restyling or changing copy.
3. Split mock state from view components.
4. Add `mockAppAdapter` for design review and `httpAppAdapter` for the real
   app.
5. Add Go UI API route handlers under `homesignal/cmd/agent`.
6. Replace inline `uiHTML` with embedded or file-served static React assets.
7. Add the local logging service and expose logging status in Advanced.
8. Build the frontend in the app Docker image.
9. Add local tests for API view models, static shell serving, logging budget
   behavior, and log bundle generation.
10. Add browser smoke coverage for Status, Pairing, Permissions, Advanced, and
   logging status, comparing the mounted app against the reference screenshots
   before calling the UI done.

## Current Prep Done

- `design-mock/vite.config.js` sets `base: "./"` so built assets are portable
  under Home Assistant ingress prefixes.
- `design-mock/src/App.jsx` marks `HaApp` as the real app mount candidate
  and points to this document.
- App UI implementation now has an explicit fidelity gate: direct mock port
  first, logged platform deviations only.
