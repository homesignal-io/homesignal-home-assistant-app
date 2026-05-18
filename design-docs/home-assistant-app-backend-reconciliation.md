# Home Assistant App Backend Reconciliation

Scope: browser walkthrough of
`http://127.0.0.1:5173/#page=HA+App&site=site_123&device=dev_123&app=onboarding`,
source review of `design-mock/src/App.jsx`, and follow-up product decisions from
the local Home Assistant settings reference.

## Position

The HA app shell is directionally valid. The main status surface maps to the
existing `device.health_snapshot`, `device_presence`, `device_latest_state`,
`homesignal_edge.update`, enrollment, and command lifecycle architecture.

The remaining product/contract work is specific:

- Claim authority remains HTTPS enrollment/finalization.
- After claim, cloud sends a simple IoT `claim_welcome` nudge with non-secret
  display metadata. No command ACK/result. The app just stores the values.
- `claim_welcome` should be a retained MQTT message on an exact per-device
  custom topic so the app can receive it after first subscription without
  relying on an opaque broker handshake.
- The settled name for the permission list is **Local Management Permission
  Catalog**.
- Local management policy is enumerated per permission from that catalog, not
  one global "yes to everything".
- Local management policy is included in the pairing request when available.
- Later local policy edits are made from a local settings page and sent to cloud
  as a policy delta with the normal telemetry/runtime reporting path.
- Auto-update, watchdog, and start-on-boot are Home Assistant local settings.
  HomeSignal only instructs the user to set them locally.
- Local unpair/reset is a hard requirement and must work without cloud.
- A local settings/search page is needed.

No remaining item below requires product judgment. Remaining work is contract
implementation, local UI implementation, or telemetry/read-model depth.

## Source Map

| UI surface | Backend/source of truth | Status |
| --- | --- | --- |
| Claim invite code, expiry, loading, rate limit | `device_claim_invites`, `device_claim_verifications`, and app local pairing state | Ready after invite-flow update |
| Claim state: unclaimed, pairing pending, claimed, revoked | `/config/device.json` plus enrollment finalization | Ready |
| Post-claim friendly metadata | Retained IoT `claim_welcome` notification after `CLAIMED` | Settled |
| Device ID / Thing name | durable `device_id`, with Thing name equal to `device_id` | Ready |
| Cloud paths: Agent HTTPS and IoT connected | `device.health_snapshot.payload.agent.cloud_connection` plus IoT presence | Ready |
| Last telemetry/report time | local send state and cloud `last_accepted_telemetry_at` | Ready, implementation depth needed |
| Agent/Core/Supervisor/backup/storage rows | `device.health_snapshot.payload.agent` and `payload.home_assistant` | Ready at architecture level |
| Managed apps row and event counts | `device.health_snapshot.payload.ha_apps[]` | Ready at architecture level |
| Runtime warnings and suppression budget | `runtime_log_summary[]` and `agent.suppression_counts[]` | Ready at architecture level |
| Local diagnostic log store | bounded `/config/logs` ring plus local registry | Ready after app runtime architecture update |
| Cloud-requested app log bundle | `upload_artifact` command with `error_log_bundle` purpose and brokered upload | Ready after app runtime architecture update |
| App current/desired/latest version | `agent.update` plus `homesignal_edge.update` desired/reported state | Ready |
| Auto-update/watchdog/start on boot | Local Home Assistant instructions only | Ready if instruction-only |
| Remote management policy switches | Local Management Permission Catalog plus telemetry delta | Settled |
| Local unpair | Local reset action; cloud release/revoke is separate | Settled requirement |
| Advanced/search settings page | Home Assistant-style local UI | Settled requirement |
| Telemetry projection/read models | Current ingest only projects a subset | Follow-up |

## Post-Claim Flow

Claiming must happen first:

```text
SaaS user creates a site-bound claim invite
  -> local app verifies the claim invite and user confirms the details
  -> HTTPS enrollment/finalization completes
  -> backend marks device CLAIMED
  -> app stores durable credentials and boots runtime
  -> app connects to IoT Core and subscribes
  -> cloud publishes retained claim_welcome over IoT
  -> app stores non-secret display metadata
  -> next telemetry reports current local metadata/policy revision
```

The `claim_welcome` message is not claim authority. It is a friendly post-claim
state nudge and an end-to-end runtime smoke test.

### IoT `claim_welcome` Notification

Topic:

```text
homesignal/devices/{device_id}/notifications/claim_welcome
```

Delivery:

- Publish as an MQTT retained message on the exact topic above.
- The app must subscribe to the exact topic, not only a wildcard filter, if
  it wants retained delivery on first subscription.
- This is a custom HomeSignal topic, not a reserved AWS shadow topic.
- The app applies the latest retained message idempotently.
- Cloud may replace the retained message when display metadata changes, or clear
  it after the app has reported the current metadata revision/hash through
  telemetry.

Payload:

```json
{
  "message_type": "notification",
  "schema_type": "claim_welcome",
  "schema_version": 1,
  "message_id": "01J00000000000000000000010",
  "sent_at": "2026-05-14T12:03:00Z",
  "payload": {
    "device_id": "dev_123",
    "claim_operation_id": "claimop_123",
    "claimed_at": "2026-05-14T12:02:00Z",
    "account": {
      "account_id": "acct_123",
      "display_name": "Northstar Smart Homes"
    },
    "site": {
      "site_id": "site_123",
      "display_name": "Smith Residence"
    },
    "claimed_by": {
      "display_name": "Maya Patel",
      "email": "maya.patel@northstar.example"
    },
    "portal": {
      "label": "Open HomeSignal portal",
      "url": "https://app.homesignal.local/sites/site_123/devices/dev_123"
    },
    "local_display": {
      "manager_title": "HomeSignal Manager",
      "managed_by_label": "Northstar Smart Homes",
      "site_label": "Smith Residence"
    }
  }
}
```

Rules:

- Send only after backend has marked the device `CLAIMED`.
- No command ACK/result is required.
- No secrets, tokens, certs, signed URLs, private keys, broad customer data, or
  policy authority are allowed in the message.
- The app applies the message only when `payload.device_id` matches its local
  claimed `device_id`.
- The app stores the non-secret display metadata locally.
- If the message is missed, the device remains claimed. The next health snapshot
  can still report what local metadata revision it has.
- Retained delivery is the race guard. Do not depend on AWS IoT Core sending a
  special first message when a device pairs.

Suggested next health snapshot addition:

```json
{
  "agent": {
    "claim_display": {
      "metadata_revision": 1,
      "metadata_hash": "sha256:...",
      "last_claim_welcome_message_id": "01J00000000000000000000010",
      "applied_at": "2026-05-14T12:03:10Z"
    }
  }
}
```

## Local Management Permission Catalog

The settled catalog name is **Local Management Permission Catalog**. The app
should enumerate local management permissions from this catalog. Do not use a
single "full access" boolean as the backend contract. The UI can offer presets,
but the stored/sent shape should be explicit.

### Claim Confirmation Shape

Add this optional block to
`POST /api/v1/device-enrollment/claim-verifications/{claim_verification_id}/confirm`
when the local UI has collected the policy:

```json
{
  "local_management_policy": {
    "schema_version": 1,
    "policy_revision": 1,
    "authority_profile": "managed_admin",
    "source": "local_ha_app_pairing",
    "selected_at": "2026-05-14T12:00:00Z",
    "permissions": [
      { "key": "ha_status_read", "enabled": true },
      { "key": "ha_backup_status_read", "enabled": true },
      { "key": "ha_backup_trigger", "enabled": true },
      { "key": "ha_storage_status_read", "enabled": true },
      { "key": "ha_app_inventory_read", "enabled": true },
      { "key": "homesignal_app_update_status_read", "enabled": true },
      { "key": "homesignal_app_update_intent", "enabled": true },
      { "key": "diagnostics_basic", "enabled": true },
      { "key": "diagnostics_error_log_bundle", "enabled": false },
      { "key": "runtime_log_summary", "enabled": true }
    ]
  }
}
```

Initial v0 Local Management Permission Catalog keys:

| Key | Meaning | Default managed install |
| --- | --- | --- |
| `ha_status_read` | Read Home Assistant Core/Supervisor reachability and versions | enabled |
| `ha_backup_status_read` | Read backup summary/status | enabled |
| `ha_backup_trigger` | Allow approved cloud backup trigger command | enabled for managed installs |
| `ha_storage_status_read` | Read bounded storage status | enabled |
| `ha_app_inventory_read` | Read installed app inventory/update summary | enabled |
| `homesignal_app_update_status_read` | Read HomeSignal app update posture | enabled |
| `homesignal_app_update_intent` | Allow HomeSignal app desired version/channel intent | enabled |
| `diagnostics_basic` | Allow bounded app/connectivity/update-readiness diagnostics | enabled |
| `diagnostics_error_log_bundle` | Allow approved brokered redacted error-log bundle | disabled by default |
| `runtime_log_summary` | Send bounded collapsed runtime warning summaries | enabled |

Future/high-risk keys may exist in the UI as locked/off/local-only rows, but
they are not executable until an owning command/service contract exists:

- `ha_app_install`
- `ha_app_rollback`
- `ha_core_update`
- `broad_ha_diagnostics`
- `raw_log_export`
- `arbitrary_ha_service_call`

Rules:

- This policy metadata does not authorize the claim by itself.
- Cloud command execution still requires normal cloud authorization, site policy,
  command allowlist, and the local app policy gate.
- Unknown permission keys are ignored or recorded as unsupported; they must not
  grant authority.
- Local policy must be persisted locally and survive restart.

### Policy Delta With Telemetry

When the user changes policy later from the local settings page, the app
should persist the new local policy revision and send a delta to cloud through
the approved Agent HTTPS runtime reporting path.

Do not publish the whole policy after pairing. Send only changed keys plus
revision/hash metadata.

Event shape:

```json
{
  "message_type": "event",
  "schema_type": "local_management_policy_delta",
  "schema_version": 1,
  "message_id": "01J00000000000000000000020",
  "applied_publish_policy_version": "ppv_123",
  "observed_at": "2026-05-14T13:00:00Z",
  "payload": {
    "policy_revision": 2,
    "previous_policy_revision": 1,
    "changed_at": "2026-05-14T13:00:00Z",
    "source": "local_app_settings",
    "changes": [
      {
        "key": "diagnostics_error_log_bundle",
        "previous_enabled": false,
        "enabled": true
      }
    ],
    "full_policy_hash": "sha256:..."
  }
}
```

The next `device.health_snapshot` should also carry the current policy revision
and hash so cloud can detect missed deltas:

```json
{
  "agent": {
    "local_management_policy": {
      "schema_version": 1,
      "policy_revision": 2,
      "authority_profile": "managed_admin",
      "policy_hash": "sha256:...",
      "updated_at": "2026-05-14T13:00:00Z"
    }
  }
}
```

If cloud detects a revision/hash mismatch, it should mark policy sync as
out-of-sync and wait for the next delta or a future explicit reconciliation
flow. The routine telemetry path should not send the full policy blob.

## Local Settings Page

The local settings UI should follow Home Assistant's own settings style, like
the Analytics screen reference: top app bar with back affordance, a centered
card, row labels with short explanatory text, and right-aligned toggles.

Needed local pages:

- Status
- Pairing
- Management policy
- Advanced

If Status or Pairing already exists in the app UI, this does not require a
new page. The requirement is that the local app has these surfaces available
or reachable in the Home Assistant-style navigation.

Management policy page:

- Shows the enumerated permissions above.
- Uses toggles for local policy values.
- Presets are allowed, but they must write the explicit permission list.
- Future/unsupported actions should be locked/off with short local copy.
- Saves locally first, then ships telemetry delta.

Advanced page:

- Includes local unpair/reset.
- Includes search/filter for local policy rows and advanced actions.
- Shows current local policy revision/hash and last cloud sync time.
- Shows current claim/display metadata revision.
- Shows local logging status, storage budget usage, active debug session
  metadata, dropped/suppressed counts, and last cloud log request/result.
- Shows a read-only local log tail for recent app runtime logs; no send/upload
  action.

## Local Logging And Cloud Log Requests

The clean app runtime architecture, local log store, logging levels, UI
adapter, and cloud log-request contract are defined in
`ha-app-runtime-architecture.md`.

Closed defaults:

- local diagnostic logs are bounded under `/config/logs`
- default local diagnostic log budget is `32 MiB`
- oldest retained log segment is overwritten/deleted first
- routine cloud logging remains collapsed `runtime_log_summary[]`, not raw logs
- local UI log tail is read-only and display-only
- cloud-requested app runtime logs use `upload_artifact` with purpose
  `error_log_bundle`
- cloud may select by local artifact ref or bounded query selectors, never by
  arbitrary local path
- no local "send logs" button; cloud pulls logs through the brokered artifact
  command path
- local capture level, routine cloud publish verbosity, and temporary debug
  capture are separate controls

## Local Unpair Requirement

The app must support local unpair/reset from Home Assistant.

Required behavior:

- Works without cloud connectivity.
- Stops runtime cloud messaging.
- Removes or invalidates local claimed-device metadata and local runtime
  credentials enough that the app returns to unclaimed behavior.
- Preserves `installation_id` by default so the next pairing can carry
  recognition context for repair/reconnect/fresh-claim decisions.
- Does not delete, transfer, release, or revoke the cloud device record by
  itself.
- Old cloud record becomes disconnected/unhealthy when old credentials stop
  reporting.
- A new pairing can later create a fresh claim or go through an authorized
  repair/reconnect flow in the web claim context.
- A stronger factory reset may be added under Advanced later, but local unpair
  itself should not clear `installation_id`.

Suggested local event when cloud is reachable:

```json
{
  "message_type": "event",
  "schema_type": "local_unpair_performed",
  "schema_version": 1,
  "message_id": "01J00000000000000000000030",
  "observed_at": "2026-05-14T14:00:00Z",
  "payload": {
    "previous_device_id": "dev_123",
    "reason": "local_user_requested",
    "local_reset_at": "2026-05-14T14:00:00Z"
  }
}
```

This event is best-effort only. Local unpair must not depend on it.

## Auto-Update, Watchdog, Start On Boot

HomeSignal should not mutate these Home Assistant app settings. The UI should
show instructions for the user to set them locally.

Instruction-only copy is valid:

- enable Auto update
- enable Watchdog
- enable Start on boot
- install the latest HomeSignal app version if Home Assistant shows one

If Supervisor later exposes reliable read-only values, the app may report
them as optional observed facts. They remain local-only settings.

## Telemetry Implementation Follow-Up

The architecture is ready, but current ingest implementation is not yet deep
enough to live-back every shell row. Current code validates much of
`device.health_snapshot`, but only projects a small material subset.

Follow-up needed before live wiring:

- project cloud connection details
- project telemetry freshness
- project Supervisor status/version
- project storage details
- project app inventory/activity
- project runtime warning summaries and suppression counters
- add read-model/API fields for the shell

This is not a product-design blocker, but it is a backend implementation task
before the shell can stop using mock data.

## Closed Judgment Gaps

- **Post-claim metadata delivery:** retained MQTT `claim_welcome` on an exact
  per-device topic, not command ACK/result and not claim authority.
- **Permission catalog name:** Local Management Permission Catalog.
- **Policy shape:** enumerated permission keys; no single global full-access
  backend flag.
- **Policy updates:** send changed keys plus revision/hash metadata; do not
  publish the whole policy after pairing through routine telemetry.
- **Local pages:** Status, Pairing, Management policy, and Advanced, reusing
  existing Status/Pairing pages when present.
- **Local unpair:** required, cloud-independent, preserves `installation_id`,
  and does not release/revoke/delete the cloud device record.
- **Auto-update/watchdog/start-on-boot:** instruction-only local Home Assistant
  settings.
