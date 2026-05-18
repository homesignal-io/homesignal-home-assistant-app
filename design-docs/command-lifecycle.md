# Command Lifecycle

This spec defines the default lifecycle for cloud-to-device commands. It reconciles command intent, ACK behavior, terminal results, optional progress, and desired-state convergence.

Commands are ACK-required cloud-to-device instructions. Notifications are fire-and-forget hints and are outside this lifecycle.

## Principles

- ACK means the app accepted or rejected the command, not merely that a packet arrived.
- ACK should arrive quickly enough to prove the app is alive and understood the request.
- Terminal result is separate from ACK because some commands take much longer than the acceptance window.
- Progress updates are opt-in by command type.
- Desired-state convergence is a related but separate state-management concern.
- Durable edge desired state normally uses the Edge State Adapter and AWS IoT named shadows, not command records.
- Sending a command is never proof that local state changed.
- V0 command delivery uses AWS IoT Core MQTT; command ACKs, progress, and terminal results return through the mTLS Agent HTTPS API unless a command-specific spec says otherwise.

## Default Lifecycle

```text
queued
  -> sent
  -> ack_accepted | ack_rejected | ack_timed_out
  -> running/progress, optional
  -> succeeded | failed | timed_out | canceled
```

Default semantics:

| Stage | Meaning |
| --- | --- |
| `queued` | Cloud accepted the request and created a durable command record. |
| `sent` | Cloud attempted delivery to the device transport. |
| `ack_accepted` | App received, validated, and agreed to attempt the command. |
| `ack_rejected` | App received and rejected the command with a bounded reason code. |
| `ack_timed_out` | No accepted/rejected ACK arrived inside the ACK window. |
| `running` / `progress` | Optional command-specific execution update. |
| `succeeded` | Command reached terminal success. |
| `failed` | Command reached terminal failure with bounded reason code. |
| `timed_out` | Execution did not complete inside its command-specific result deadline. |
| `canceled` | Cloud or command owner canceled before terminal execution. |

Do not create a required separate `received` state for v0. If a future transport exposes delivery receipts, store them as operational metadata, not as the command ACK.

## Default Timing

Default ACK window:

```text
15 seconds
```

The ACK window is measured from command delivery attempt or publish time, according to the transport adapter. A future command spec may override this when the command cannot safely validate in 15 seconds, but slow validation should be unusual.

Result deadlines are command-specific:

- publish policy refresh/forced apply: short; ACK may also be the effective terminal result
- artifact upload: ACK quickly after accepting the IoT command notice; command details, upload negotiation, and result use the mTLS Agent HTTPS API
- diagnostics: ACK quickly, sparse progress if useful, result after collection/upload
- backup: ACK quickly, optional phase progress, result after backup attempt
- update status/repair: ACK quickly, sparse status progress only when explicitly modeled, result after bounded check/repair work
- release/revoke: ACK quickly when local cleanup/convergence is explicitly commanded, result after bounded local cleanup

After deadlines pass:

- ACK-timeout => `ack_timed_out`. The command remains tracked by `command_id` so we can decide whether to retry or treat as operational failure.
- Result-timeout => `timed_out` terminal state when the command-specific result deadline is exceeded.
- In v0, timeout does not automatically grant or revoke command-level cloud authority; it is visible to support/internal tooling and command-family policy.

## ACK Payload

Command ACK and terminal result use the common Agent HTTPS runtime envelope style defined by `api-facade.md`. ACK and result remain separate endpoints. A command that completes immediately may send ACK followed by terminal result with the same effective outcome, but the ACK still records accepted/rejected semantics.

ACK payload should include:

- `command_id`
- `status`: `accepted` or `rejected`
- `device_id`, inferred from credential identity in cloud
- `command_type`
- bounded `reason_code`, when rejected
- accepted local preconditions, when useful
- active local policy/version facts, when relevant
- timestamp from device, treated as advisory

ACK payload must not include:

- signed URLs
- secrets
- large logs
- raw policy blobs
- local file contents beyond allowed bounded diagnostic excerpts

## Progress Updates

Progress updates are optional and must be command-type specific.

Use progress only for long-running, operator-visible commands where intermediate state changes product or operations understanding.

Examples:

- update status/repair: checking reported version, verifying local update readiness, reporting blocked/failed/applied state
- backup: requested, running, upload pending, verifying
- diagnostics/artifact upload: packaging, uploading, uploaded

Do not send percentage spam. Prefer phase changes and rate-limited updates.

## Terminal Result

Terminal result should include the common command result wrapper plus a typed domain payload:

```text
command_id
status
result_type
started_at
finished_at
reason_code
payload
```

`payload` is typed by `result_type` and owned by the domain service, such as Backup, Release/Update, or Diagnostics.

Terminal result should include:

- `command_id`
- final status
- bounded reason code on failure
- result summary
- correlation IDs for artifacts or follow-up events
- size/checksum metadata for artifact uploads

The terminal result must not echo signed URLs or secrets.

For artifact upload commands, signed URLs must not be sent over IoT Core. The command notice carries command identity/type only; the agent retrieves command details and upload capability through the mTLS Agent HTTPS API defined in `artifact-upload-broker.md`.

## Desired-State Convergence

Some commands are repair or execution attempts related to durable desired state.

Example:

```text
desired state: device active publish policy is pp_124
shadow desired: publish_policy.version is pp_124
optional command: refresh_publish_policy
ACK: app accepted/rejected the refresh request within 15 seconds
convergence: app reports applied policy version pp_124 through edge-state projection
```

If convergence matters, track it separately from command delivery:

- desired value
- last reported/accepted local value
- convergence window/deadline
- convergence status: pending, converged, late, failed, unknown

Missing ACK handling and missed convergence-window handling are product/operations policy. Do not treat missing ACK as success.

V0 no-ACK behavior:

- Missing ACK is internal/support-visible only by default; do not surface it in normal customer UI.
- Safe/idempotent commands may retry with bounded backoff until command expiry.
- Non-idempotent or unsafe commands should record timeout rather than automatic retry.
- High-consequence no-ACK creates an internal alert candidate for release/revoke, update, policy refresh, backup trigger, and artifact request.
- If AWS IoT lifecycle says the device is connected but ACKs repeatedly fail, record the condition for platform health correlation.

## Command Vs Desired State

Not every state-affecting change is a command, and not every command changes desired state.

Use durable desired state for targets that should remain true until changed:

- active publish policy version
- desired agent/app/supervisor version
- device suspended/revoked/released lifecycle authority
- backup schedule or policy
- remote access enabled/configured policy
- local command/execution policy

Use commands for bounded attempts:

- `refresh_publish_policy`
- `upload_artifact`
- `run_diagnostic`
- `trigger_backup`
- `test_remote_access`
- update status/check or update repair commands, where explicitly supported
- future credential/local cleanup actions

Use both when a command is a bounded execution attempt related to a durable target:

```text
desired state: desired agent version is 1.8.3
edge desired state: desired agent version is 1.8.3
optional command: check/report update status, or a later explicit local-supervisor apply command
ACK: accepted/rejected within the ACK window
progress/result: bounded status/repair phases and terminal outcome
convergence: reported agent version becomes 1.8.3
```

In v0, HomeSignal does not deliver binary update artifacts or initiate app
installation over IoT Core. Normal HomeSignal app update execution remains
with the local Home Assistant Supervisor/app update path. Command-class update
work is limited to explicitly modeled bounded checks, status, repair, or future
apply flows.

Do not use commands for fire-and-forget nudges. Use notifications for hints such as "policy changed, check in soon" when no result is required.

## Initial Command Types

Initial command types using this lifecycle:

- `refresh_publish_policy`
- `trigger_backup`
- explicitly modeled update check/status/repair commands
- release/revoke local convergence commands, where explicitly supported

Future command types using this lifecycle:

- `upload_artifact`
- `run_diagnostic`
- `enable_debug_capture` / `disable_debug_capture`
- `collect_debug_snapshot` / `request_debug_bundle`
- `apply_update`, only if a later update spec explicitly enables command-driven
  local update execution
- `test_remote_access`

## Acceptance Criteria

- ACK means accepted/rejected within a short window, not mere packet receipt.
- Every command type defines ACK window and result deadline.
- Progress is opt-in and rate-limited.
- Terminal results are separate from ACK for long-running commands.
- Desired-state convergence is tracked separately when local state must catch up to cloud intent.
- Signed URLs and secrets are never echoed in ACK, progress, or result payloads.
