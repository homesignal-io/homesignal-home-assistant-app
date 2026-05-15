# State Change And Policy Propagation Workstream

State change and policy propagation governs cloud-authoritative product policy that must be enforced locally by the add-on or future supervisor. It covers desired state, cloud-provisioned local policy, entitlement-derived limits, scoped policy refresh, ACK/result reporting, and stale-policy behavior.

## Agent Use

Read this when touching:

- cloud-provisioned add-on configuration
- event publish budgets
- plan or entitlement limits that affect device behavior
- desired state sent to a device
- scoped policy refresh commands
- local policy files or local enforcement rules
- command, diagnostic, backup, update, or release state changes
- stale local config behavior

## Current Anchors

- `command-lifecycle.md`
- `edge-state-adapter.md`
- `local-cloud-trust-boundaries.md`
- `topic-design.md`
- `aws-iot-routing-contract.md`
- `telemetry-ingest-architecture.md`
- `service-map.md`
- `auth-service.md`
- `outstanding-decisions.md`

## Principles

- Cloud product policy is authoritative for cloud-side enforcement immediately.
- The add-on enforces the last accepted cloud policy locally.
- Remote refresh accelerates convergence, but correctness must not depend on it.
- Missing, expired, or stale local policy must fail to a conservative default, not unlimited behavior.
- Devices must not infer business entitlement from ownership alone.
- Business policy resolution stays server-side; local devices receive only the policy they need to enforce.
- Cloud desired state and local execution/result are separate facts.
- AWS IoT named shadows are the v0 desired/reported edge-state exchange surface.
- The Edge State Adapter owns shadow I/O and compact edge-state projection.
- Product services must not write shadows directly.
- Postgres stores product truth plus compact projections, not a full shadow mirror.
- Publish-policy refresh must stay scoped to publish policy; it is not a general remote configuration channel.
- Cloud-to-device messages have two logical classes: notification and command.
- Notifications are fire-and-forget hints. Commands require a device ACK/result path.
- Missing ACK handling is defined by `command-lifecycle.md`. Do not silently
  treat missing ACK as success.

## Implementation Defaults

Cloud policy changes:

```text
business policy changes
  -> cloud services enforce current policy immediately
  -> cloud records policy version/effective time
  -> Edge State Adapter updates compact shadow desired state when device behavior must converge
  -> cloud may send a fire-and-forget notification or ACK-required command only to accelerate or repair convergence
```

Local add-on behavior:

```text
last accepted policy exists and is fresh
  -> enforce it locally

policy missing, expired, unreadable, or invalid
  -> enforce conservative default

shadow desired update received or local refresh runs
  -> fetch/accept newer policy if authorized and valid
  -> report applied policy version through shadow reported state
  -> emit internal agent alarm when policy application fails or looks suspicious

publish-policy refresh command received
  -> perform bounded repair/acceleration attempt
  -> report accepted/rejected command result and applied/rejected policy version
  -> still keep durable policy convergence represented through edge-state projection
```

Cloud-to-device message classes:

| Class | Delivery contract | Use when | Must not be used for |
| --- | --- | --- | --- |
| Notification | Fire-and-forget. No ACK expected. Cloud must not assume local state changed. | Non-critical hints, reconnect nudges, "state changed, check in soon" messages. | Any instruction where cloud needs to know the device accepted or applied something. |
| Command | ACK/result required. Has command identity, expiry, allowlisted type, short-window accepted/rejected ACK, and terminal result reporting. | Scoped policy refresh/repair, diagnostics, backup actions, update status/repair where explicitly supported, revocation/release flows, future host-affecting work. | Cheap hints that do not need result tracking. |

Active publish policy is durable desired state through the Edge State Adapter. `refresh_publish_policy` is command-class only when cloud needs a bounded repair/acceleration attempt. A separate policy-changed hint may be modeled as a notification, but notification delivery alone is never proof of local convergence.

Command ACK semantics are defined in `command-lifecycle.md`: ACK means the add-on accepted or rejected the command within the ACK window, not mere packet receipt. Progress is opt-in by command type. Terminal result and desired-state convergence are separate facts.

## State Transition Classification

Every cloud-to-device or device-affecting state transition must be classified before implementation:

| Class | Meaning | Track convergence? | Delivery mechanism |
| --- | --- | --- | --- |
| Desired state | Durable target condition that remains true until superseded. | Yes, when local device behavior must catch up. | Edge State Adapter / AWS IoT named shadow; notification or command may accelerate convergence. |
| Command | Bounded operation with identity, expiry, ACK, and terminal result. | Only if the command is related to desired-state convergence or the result changes durable product state. | ACK-required command. |
| Notification | Fire-and-forget hint. | No. | Fire-and-forget cloud-to-device message. |
| Device event | Device-originated fact or availability signal. | No cloud-to-device convergence, but cloud may create follow-up desired state or command. | Runtime telemetry/event publish. |

Use desired state when:

- the target should remain true across reconnects, retries, restarts, and offline periods
- the cloud needs to compare desired vs accepted/reported local state
- missed convergence is operationally meaningful
- the target may be re-sent or re-applied without changing the business meaning

Use command when:

- the work is a single bounded attempt
- the work has an expiry, result, and bounded reason codes
- the work should not remain desired forever
- repeated execution would be a new attempt, not the same durable target

Use both when a durable desired state needs a bounded local attempt in addition to shadow convergence. In that case, the desired state is the authority and the command is a repair or execution attempt.

Known HomeSignal classifications:

| Case | Class | Owner | Notes |
| --- | --- | --- | --- |
| Active publish policy version for a device | Desired state | Publish Policy / Device Registry / Edge State Adapter | Device should converge through `homesignal_edge.publish_policy` desired/reported state and report `applied_publish_policy_version` in runtime messages. |
| Apply concrete publish policy version | Desired state, with optional command repair | Publish Policy / Edge State Adapter / Command | Normal path is `homesignal_edge.publish_policy` desired/reported. Use command only when cloud needs a bounded repair/acceleration attempt. |
| Refresh publish policy | Command | Command / Publish Policy | Scoped convergence accelerator; does not change unrelated config or become the source of durable desired state. |
| Device suspended, revoked, released, or credential disabled | Desired state, plus possible command | Device Registry | Cloud authority changes immediately; local cleanup/convergence is separate. |
| Desired agent/add-on/supervisor version | Desired state | Release / Update Orchestrator / Edge State Adapter | V0 update intent lives in `homesignal_edge.update`; CI/CD publishes the add-on version first, then Release / Update Orchestrator promotes desired version/channel for a cohort. The Home Assistant Supervisor/add-on update path performs installation according to local policy. |
| Update status/repair | Command related to desired state | Release / Update Orchestrator / Command | V0 command-class update work is limited to explicitly modeled bounded checks, status, or repair. Stage/apply update commands are future/local-supervisor scope only. |
| Enabled event families and telemetry cadence | Desired state inside publish policy | Publish Policy | Device sees resolved policy, not plan/tier labels. |
| Local command/execution policy | Desired state | Future policy owner | Separate from publish policy. |
| Remote access enabled/configured state | Desired state | Remote Access Metadata / future policy owner | Test/connect operations are commands. |
| Test remote access | Command | Remote Access / Command | Bounded operation with result; not durable target. |
| Backup schedule/policy | Desired state | Backup Service | Backup is a real v0 logical service; schedule/policy may stay minimal while one-time trigger is command-class. |
| Trigger backup now | Command | Backup / Command | V0 bounded attempt with result. |
| Run diagnostic | Command | Diagnostics / Command | Bounded attempt, possibly creates artifact intent. |
| Upload artifact | Command notice + Agent HTTPS API | Artifact Upload Broker / Command / Agent HTTPS API | Cloud-authorized upload; IoT carries command notice only, mTLS HTTPS handles command details, ACK, signed URL negotiation, completion, and result. |
| Topology upload enabled/cadence | Desired state or policy | Future topology owner / Publish Policy | Device-initiated availability is only an event. |
| Topology snapshot available | Device event | Add-on / Telemetry Ingest | Cloud may respond with `upload_artifact`; event is not upload authorization. |
| Error log available | Device event | Add-on / Telemetry Ingest | Cloud may respond with `upload_artifact`; recursion guard applies. |
| Policy changed hint | Notification | Device Notification Path | No convergence proof. |
| Check in soon / reconnect nudge | Notification | Device Notification Path | No product state should depend on delivery. |

Policy application failure:

- Shadow reported state or the command result records the immediate response: attempted policy version, active local policy version, and bounded reason code. Command ID is included only when the failure happened during a command-class attempt.
- The add-on updates shadow reported state only for desired-state convergence or drift conditions; it must not use shadow reported state as a telemetry heartbeat.
- If the add-on cannot apply a policy, it must remain on the last accepted policy or conservative default.
- The add-on should emit an internal `agent_alarm` when policy application fails, because silent local policy drift is operationally important.
- Security-relevant failures, such as wrong device, invalid signature/MAC when introduced, rollback attempt, malformed policy from cloud, or repeated invalid policy delivery, should use `agent_alarm` severity `warning` or `critical`.
- Routine incompatibility failures, such as unsupported future policy schema, should use `warning` unless they leave the device unable to enforce any safe policy.
- Runtime events and command results must contain only a small structured summary. They may include a redacted diagnostic excerpt capped at 5 KB total when it materially helps triage.
- If more diagnostic detail exists, the event should set a `more_logs_available` style flag and include a local correlation/reference ID for a later cloud-authorized artifact request.
- Runtime events and command results must not include large logs, secrets, raw policy blobs, signed URLs, or local file contents beyond the bounded redacted excerpt.
- Signed object-storage URLs must not be sent over IoT Core; artifact upload negotiation uses the mTLS Agent HTTPS API.
- Errors in the artifact upload path must not recursively request more error-log artifacts.
- Upload-failure alarms should set `more_logs_available=false` by default and report local suppression counters later through health telemetry.

Event publish budgets:

- Budgets are provisioned by the cloud and enforced hard locally.
- Cloud ingest enforces current server policy independently.
- Budget is implemented per device.
- The effective device budget is inherited from the managing plan where the device currently lives.
- A later business rule may allow an integrator/support provider to raise the effective budget for devices they manage but do not own, but the cloud still resolves that into a per-device policy.
- The device must not infer budget from owner, manager, account, plan, or tier labels.
- A stale local budget can only make the add-on more conservative than cloud policy, not more permissive from the cloud's perspective.
- Devices receive only the resolved effective publish policy, not plan or tier labels.
- Every effective publish policy includes `policy_version`, `issued_at`, `refresh_after`, and `expires_at`.
- Default v0 freshness is `refresh_after=24h` and `expires_at=7d`.
- When local publish policy is expired, missing, unreadable, or invalid, the add-on falls back to conservative local behavior.
- Publish policy includes interval rules for telemetry snapshots and event budgets for live events.
- Publish policy also carries routine device observability policy: device
  runtime log/event verbosity, diagnostic excerpt allowance, health snapshot
  cadence, and budgets for routine device-originated operational facts.
- Backup summary may ride inside periodic telemetry snapshots on lower tiers;
  event-style/live backup changes are not v0, but should remain possible soon
  through resolved policy.
- Live event streams are not a v0 product requirement. The policy model keeps
  event-family gates and budgets so live events can be productized later without
  reworking the add-on/cloud contract, but v0 should not build product pricing
  or UX around live event streams.
- `agent_alarm` is allowed for all claimed devices under a strict internal security budget.
- Live `ha_event` is disabled by default and only enabled by explicit resolved policy.
- Every claimed-device runtime publish should report `applied_publish_policy_version` as metadata.
- V0 starts with a free/default tier for all devices, but the policy model must support admin-defined tiers from the beginning.
- Admin-defined tiers produce resolved per-device policy records; devices never receive mutable tier names as authority.
- A tier can change telemetry cadence, backup-summary inclusion, live event allowance, and event budgets without changing the add-on contract.
- A tier can also change routine device observability verbosity, but `debug`
  verbosity remains time-boxed debug mode rather than a standing policy level.
- Publish-policy values are durable, auditable records. They should be seeded
  as defaults and reviewed through an internal/admin surface later; runtime code
  should consume resolved policy records rather than hard-coded business limits.
- V0 default values are intentionally coarse. They are expected to become finer
  by plan, site, support relationship, event family, and temporary support
  override without changing the add-on contract.

V0 free/default publish-policy seed:

```json
{
  "telemetry": {
    "snapshot_interval_seconds": 3600,
    "include_backup_summary": true
  },
  "events": {
    "ha_event": {
      "enabled": false,
      "max_events_per_hour": 25
    },
    "agent_alarm": {
      "enabled": true,
      "max_events_per_hour": 10,
      "burst_max_events_per_minute": 3
    }
  },
  "observability": {
    "level": "normal",
    "runtime_logs": {
      "enabled": true,
      "min_level": "warning",
      "max_events_per_hour": 10,
      "max_bytes_per_hour": 10240
    },
    "diagnostic_excerpt": {
      "enabled": true,
      "max_bytes_per_event": 5120
    },
    "health_snapshot": {
      "interval_seconds": 3600
    }
  },
  "artifact_uploads": {
    "device_initiated_uploads_enabled": false,
    "requires_cloud_authorized_command": true
  },
  "freshness": {
    "refresh_after_seconds": 86400,
    "expires_after_seconds": 604800
  }
}
```

Routine device observability policy:

```json
{
  "observability": {
    "level": "normal",
    "runtime_logs": {
      "enabled": false,
      "min_level": "warning",
      "max_events_per_hour": 5,
      "max_bytes_per_hour": 10240
    },
    "diagnostic_excerpt": {
      "enabled": true,
      "max_bytes_per_event": 5120
    },
    "health_snapshot": {
      "interval_seconds": 3600
    }
  }
}
```

Allowed standing levels:

| Level | Meaning |
| --- | --- |
| `quiet` | Required health snapshot plus critical agent alarms only. |
| `normal` | Low-rate health, backup summary when available, warning/critical agent alarms. |
| `verbose` | More routine runtime facts and selected info-level logs under strict byte/event caps. Should be support/admin controlled and may be time-bounded by policy. |

`debug` is not a durable publish-policy level. Rich debug capture uses
temporary debug mode through Diagnostics and Command Service.

Conservative default:

- allow low-rate `telemetry` with `schema_type=device.health_snapshot`
- include backup summary only inside the low-rate snapshot when available
- allow `agent_alarm` under a strict security budget
- allow routine runtime logs only at warning or above, collapsed and capped
- disable live `ha_event`
- disable paid/live event behavior

Publish Policy Refresh:

- Refresh is best-effort.
- Normal durable publish-policy convergence uses the Edge State Adapter and AWS IoT named shadows.
- Publish-policy refresh commands are idempotent and rate-limited.
- `refresh_publish_policy` refreshes only event/telemetry publish allowlists, routine device observability policy, budgets, policy version, `refresh_after`, and `expires_at`.
- `refresh_publish_policy` must not change command permissions, host-write/local execution policy, credentials, enrollment/claim state, update policy, backup policy, diagnostics policy, or remote access policy.
- Failure to refresh is visible operationally but is not a correctness gap because cloud ingest still enforces current policy.
- A received `refresh_publish_policy` command must produce an ACK/result that reports accepted or rejected policy version, while the durable applied version is still represented in edge-state projection.
- A rejected refresh that indicates local policy failure should also emit a bounded internal `agent_alarm`.
- If the device is offline, AWS IoT shadow desired state catches up on reconnect; local timers may also refresh later.

Over-budget remediation:

- Routine over-budget events are dropped/counted by cloud ingest after enough parsing to classify safely.
- Sustained over-budget publishing creates an internal security/abuse signal after a threshold, not a user-visible alert.
- Cloud ingest may request `refresh_publish_policy` automatically for sustained over-budget publishing.
- Future Platform Health / Monitoring correlates sustained over-budget and runaway messaging across devices, accounts, topics, policy versions, and time windows.
- If the device keeps violating after refresh attempts, escalation stays internal: security review, event-family disablement, or credential revocation in a later spec.

Stale policy and convergence visibility:

- V0 stale local policy, policy drift, pending convergence, and
  publish-policy no-ACK conditions are internal/support-visible by default.
- Do not productize these details or expose them directly to customers in v0.
- The platform should still retain enough state for support/internal diagnosis
  and future product work.
- Do not expose normal convergence machinery to customers until a product spec
  defines the UX language and support behavior.

## Required Local Plan Checks

Every affected service or add-on plan should state:

- state-transition class: desired state, command, notification, or device event
- authoritative cloud state or policy owner
- local cached policy shape
- policy version and freshness behavior
- conservative default when policy is missing or stale
- refresh trigger and normal refresh cadence
- whether scoped remote policy refresh exists
- cloud-side enforcement backstop
- local ACK/result reporting path, if any
- Edge State Adapter/shadow projection shape, if durable desired edge state is involved
- cloud-to-device message class: notification or command
- business entitlement source, if policy depends on plan or relationship
- observability for stale, rejected, or failed policy updates

## V0 Decisions (Closed)

- `refresh_publish_policy` command shape is standardized:
  - Created as a command-class record owned by the Command Service.
  - Delivered with the shared command envelope (`command_type`, `device_id`,
    `command_id`, optional `payload`) and uses the common ACK/result contract from
    `command-lifecycle.md`.
  - It is a repair/acceleration attempt only; success/failure is reported through
    the command result path, while durable convergence remains represented by
    the Edge State Adapter/shadow projection.

The concrete request payload is intentionally minimal: command owner, target device,
`command_type: refresh_publish_policy`, and optional hints for logging/observability.

## Acceptance Criteria

- Cloud services enforce current policy immediately after a state change.
- Add-on local behavior is bounded by last accepted policy or conservative defaults.
- Scoped remote refresh is useful but not required for correctness.
- Publish-policy refresh cannot mutate unrelated local policy or credentials.
- Missing or stale local policy never becomes unlimited publishing or unsafe execution.
- Business entitlement resolution happens server-side.
- Local plans distinguish desired state, accepted local policy, and reported execution result.
