# Local/Cloud Trust Boundaries Workstream

Local/cloud trust boundaries define what the cloud may request, what the local app or future supervisor may execute, and which authority checks must happen before host-affecting behavior.

Device lifecycle, identity, and runtime authority rules live in `device-lifecycle.md`. This workstream defines what trusted devices may be asked to do locally, not what makes them trusted devices in the first place.

## Agent Use

Read this when touching:

- local command execution
- host writes
- backups
- diagnostics
- update orchestration
- remote access
- app/device approval
- cloud-initiated desired state
- cloud-provisioned local policy
- scoped policy refresh and stale-policy behavior
- local supervisor design
- Home Assistant permission surface

## Current Anchors

- `workstreams/identity-and-authorization.md`
- `identity-and-authorization.md`
- `command-lifecycle.md`
- `edge-state-adapter.md`
- `service-map.md`
- `update-architecture.md`
- `auth-service.md`
- `enrollment-claiming-contract.md`
- `state-change-and-policy-propagation.md`

## Principles

- The cloud can authorize product intent, but the local agent must protect the host.
- No hidden vendor god-mode.
- Remote/host control requires cloud permission, site policy, local app policy, and app/device approval.
- Local private keys and host authority stay local.
- The local agent should fail closed for unsafe commands.
- User-visible and audit-visible authority changes are preferred over silent privilege expansion.
- Future local supervisor work should narrow and harden host mutation paths.
- Every cloud-to-device state change must distinguish durable desired state from notification and command delivery.
- Every direct cloud-to-device message must be classified as a notification or a command.
- Notifications are fire-and-forget; commands require an ACK/result path.

## Implementation Defaults

- Treat local command gates as mandatory for host-affecting actions.
- Treat command ACK as app accepted/rejected, not transport receipt.
- Keep cloud desired state separate from local execution result.
- Use the Edge State Adapter / AWS IoT named shadows for compact durable desired edge state.
- Keep current cloud policy separate from last accepted local policy.
- Require explicit ACK/result paths for commands that affect local state.
- Do not use notification delivery as evidence that a device accepted policy or changed local state.
- Treat scoped remote policy refresh as a convergence accelerator, not correctness authority.
- Keep publish-policy refresh separate from general config, credential, command, and host-control policy.
- Prefer allowlisted local operations.
- Audit cloud-side request intent and local-side execution outcome where practical.
- Keep diagnostics and backups scoped and redacted.

## V0 Trust Boundary Stance

V0 does not provide arbitrary remote control of devices behind the Home Assistant gateway. The cloud may request only allowlisted device actions that are explicitly modeled by service specs, authorized by the control plane, delivered through the approved device transport for that action, and accepted by the local app policy gate.

The cloud can set product intent, but it cannot directly reach through Home Assistant as an unrestricted operator. Local execution remains bounded by app capabilities, Home Assistant/Supervisor permissions, local policy, command type, and safety checks.

Allowed or expected v0 host-affecting cases:

- HomeSignal app update intent/status flow through the release/update architecture
- backup trigger and backup status/reporting
- scoped publish-policy refresh
- release/revoke convergence and local credential cleanup where explicitly supported
- approved artifact upload flows where AWS IoT carries only the command notice and the mTLS Agent HTTPS API handles command details, ACK, signed URL negotiation, completion, and result
- health/status collection from Home Assistant, Supervisor, app runtime, storage, backup, update, and connectivity signals

Deferred or not allowed in v0:

- arbitrary shell execution
- generic remote file read/upload
- generic artifact transfer outside an approved product flow
- unrestricted Home Assistant service calls
- LAN scanning or subnet routing
- user impersonation or hidden vendor access
- generic customer-facing remote access/control

Potential future trust-boundary product areas, not v0 blockers:

- Whether a customer/site can opt into broader remote support actions, and what visible consent is required.
- Whether local approval is required per command, per command family, or by standing site policy.
- Whether diagnostics can ever include broader Home Assistant configuration snapshots, and under what redaction policy.
- Whether support/provider accounts can request emergency safety actions, and how those actions are audited and surfaced.
- Whether local Home Assistant users should have a local kill switch that blocks cloud-initiated host-affecting commands.

Settled v0 rollback boundary:

- User/integrator-approved rollback is a normal platform capability for HomeSignal app releases.
- Automatic rollback is allowed for failed HomeSignal-controlled app update attempts when bounded health/startup/reconnect checks fail.
- Automatic rollback must not mutate broader Home Assistant, Supervisor, OS, database, or arbitrary host state.
- Support-only hidden rollback authority is not a v0 model; support actions use ordinary authorization and audit paths.

Settled v0 diagnostics approval boundary:

- Bounded HomeSignal app diagnostics and temporary debug capture do not
  require local user approval.
- They do require cloud-side authorization, audit, TTL, redaction, size/rate
  limits, and app allowlists.
- This covers HomeSignal app runtime status, HomeSignal cloud/API
  connectivity, AWS IoT connectivity, app version, policy version, update
  status, backup summary, redacted HomeSignal app log excerpts, and bounded
  diagnostic/debug/error bundles through the approved artifact path.
- Future broad Home Assistant or host diagnostics require a separate local
  policy/approval model before implementation.

Diagnostics v0 scope:

- V0 diagnostics may include explicit, bounded commands such as `collect_app_status`, `collect_connectivity_check`, `collect_recent_error_excerpt`, `collect_update_readiness`, `collect_backup_status`, and `request_error_log_bundle`.
- V0 diagnostics must be scoped to HomeSignal app/runtime health, cloud connectivity, local update readiness, backup status, and redacted log excerpts.
- Broad Home Assistant configuration snapshots are not v0 diagnostics.

Future support-action optionality:

- The app may ship with a dormant, deny-by-default support-action framework
  so future support capabilities do not always require an app update.
- Dormant support actions must be declared as local capabilities with stable
  names, minimum app version, required local permission surface, redaction
  profile, max runtime, max bytes, and whether local approval is required.
- Shipping a dormant capability does not make it enabled. Cloud policy,
  Authorization Service, command allowlists, and local app policy must all
  allow the action before execution.
- Unknown, disabled, expired, unsupported, or policy-disallowed actions are
  rejected locally and reported through normal command result semantics.
- Future broad Home Assistant or host-affecting support actions still require a
  product/security decision before they are enabled, even if the app already
  contains the dormant local handler.

## Required Local Plan Checks

Every affected service or app plan should state:

- who requested the action
- state-transition class: desired state, notification, command, or device event
- cloud-to-device message class: notification or command
- which cloud authorization was checked
- which local policy gate was checked
- whether local policy was current, stale, or defaulted
- whether Home Assistant or Supervisor permissions are required
- what local files or host surfaces can be changed
- how the action is acknowledged
- command ACK window and result deadline, if command-class
- how failure is reported
- which audit or operational events are emitted

## V0 Settled Decisions

- V0 command policy and command-shape defaults are now locked:
  - `refresh_publish_policy` is command-class, ACK-required, with 15-second ACK window and command-type result deadline per `command-lifecycle.md`.
  - Local command policy is allowlisted per command type and destination, persisted in app/Cloud policy record as a compact allowlist, and rejects everything else by default.
  - `refresh_publish_policy` is a repair/acceleration command for publish policy convergence only; it must not change unrelated command permissions, credentials, enrollment state, or host-control policy.
  - Repeated missing ACK remains internal/support-visible by default; high-consequence command families add internal alert candidates rather than customer-facing status.
- Future local supervisor responsibility split is still a future product consideration; v0 assumes app-host boundary for bounded local execution.

## Acceptance Criteria

- No cloud route can imply arbitrary local host execution.
- Host writes are allowlisted and justified.
- Command/update/diagnostic flows have explicit local approval or policy gates.
- Command-class flows have ACK/result reporting; notification-class flows do not claim local acceptance.
- Cloud policy changes have a cloud-side enforcement backstop when local refresh is delayed.
- Publish-policy refresh cannot mutate unrelated local policy or credentials.
- Failures are visible to both local status and cloud operations where relevant.
