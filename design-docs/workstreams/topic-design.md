# Topic Design Workstream

Topic design governs MQTT topic families, AWS IoT rules, message envelopes, schema hints, lifecycle routing, Agent HTTPS runtime envelopes, and the boundary between bootstrap and runtime device traffic.

Canonical device lifecycle, trust, and authority rules live in `workstreams/device-lifecycle.md`. Topic design must preserve that lifecycle rather than inventing identity or authority rules locally.

## Agent Use

Read this when changing:

- MQTT topic names
- AWS IoT rule SQL
- message envelopes
- schema version fields
- lifecycle event handling
- telemetry ingest routing
- command ACK/result return paths
- registration/bootstrap topics
- event allowlists or publish budgets
- AWS IoT named shadow boundaries for desired/reported edge state

## Current Anchors

- `command-lifecycle.md`
- `edge-state-adapter.md`
- `aws-iot-routing-contract.md`
- `telemetry-ingest-architecture.md`
- `device-lifecycle.md`
- `telemetry-ingest-build-plan.md`
- `service-map.md`
- `enrollment-claiming-contract.md`
- `state-change-and-policy-propagation.md`

## Principles

- Route optional MQTT messages by topic family first.
- Keep optional MQTT runtime topics narrow.
- Keep bootstrap/registration traffic separate from claimed-device runtime traffic.
- Require MQTT5 for future claimed-device runtime messages that use MQTT.
- Use MQTT5 user properties as the canonical routing and schema metadata when optional MQTT ingest is used.
- Use the mTLS Agent HTTPS API by default for v0 device-to-cloud telemetry/events.
- Use AWS IoT Basic Ingest only for future runtime families that deliberately move to MQTT/rules routing.
- Keep AWS IoT named shadows separate from HomeSignal runtime telemetry/event topic families.
- Use normal MQTT broker topics for cloud-to-device commands or any runtime family that needs MQTT subscribers.
- Cloud-to-device traffic has two logical classes: fire-and-forget notifications and ACK-required commands.
- Notification delivery is never proof that local device state changed.
- Payload device identifiers are annotations, not authority.
- Optional MQTT/AWS IoT credential identity must resolve to HomeSignal
  `device_id` before product state changes.
- Runtime events must be allowlisted, budgeted locally, and rate-limited in cloud ingest.
- Lifecycle topics are operational signals and must be handled deliberately.

## Implementation Defaults

Optional/future MQTT runtime topics:

```text
homesignal/devices/{device_id}/telemetry
homesignal/devices/{device_id}/events
```

Cloud-to-device logical topic families:

```text
homesignal/devices/{device_id}/notifications
homesignal/devices/{device_id}/commands
```

`notifications` are fire-and-forget hints. They are useful for cheap convergence nudges and should not create product state that depends on device acceptance.

`commands` are ACK-required instructions. They need command identity, expiry, allowlisted command type, a short-window accepted/rejected ACK, and a terminal result when ACK is not itself terminal.

Optional/future Basic Ingest publish topics:

```text
$aws/rules/homesignal_runtime_ingest/homesignal/devices/{device_id}/telemetry
$aws/rules/homesignal_runtime_ingest/homesignal/devices/{device_id}/events
```

The logical topic family remains the suffix after `$aws/rules/homesignal_runtime_ingest/`.

Required MQTT5 user properties:

```text
message_type
schema_type
schema_version
message_id
applied_publish_policy_version
observed_at
```

Event publishing:

- No broad `ha_event` stream in v0.
- Device-originated events must be allowlisted by schema type.
- Cloud provisions the local app publish budget.
- The app enforces the last accepted budget hard.
- The app reports `applied_publish_policy_version` with every claimed-device runtime publish.
- Conservative defaults allow low-rate `device.health_snapshot` telemetry and strict-budget `agent_alarm`, while disabling live `ha_event`.
- Cloud ingest enforces current server policy independently.
- `refresh_publish_policy` may accelerate budget changes but is scoped to publish policy and is not required for cloud-side correctness.
- Sustained over-budget publishing creates an internal security/abuse signal after a threshold.

Cloud-to-device publish-policy delivery:

- Active publish-policy version is durable desired state through the Edge State Adapter and AWS IoT named shadows.
- A policy-changed hint may use `notifications`, but it is not proof of convergence.
- `refresh_publish_policy` uses `commands` when cloud needs a bounded repair/acceleration attempt with ACK/result semantics.
- Command ACK/results should return through the mTLS Agent HTTPS API unless the command spec later chooses another authenticated return path.
- ACK means accepted/rejected within the command ACK window, not mere message receipt. See `command-lifecycle.md`.
- Missing command ACK handling is product/operations policy, not topic semantics.
- If publish-policy application fails, the app reports the bounded failure through shadow reported state or command result, depending on the path, and emits a bounded internal `agent_alarm`.
- Error events may include a redacted diagnostic excerpt capped at 5 KB total.
- If more local logs exist, the event should include `more_logs_available=true` plus a local correlation/reference ID.
- Large logs beyond the 5 KB excerpt, raw policy blobs, signed URLs, secrets, and local file contents beyond the bounded redacted excerpt must not be sent over MQTT; use brokered diagnostic artifact upload when full detail is needed.
- Artifact upload failures may emit `artifact_upload_failed`, but must not recursively set `more_logs_available=true` or trigger automatic `error_log_bundle` requests.

Possible future bootstrap topics:

```text
homesignal/registration/{claim_or_installation_id}/...
```

Do not deploy broad catch-all runtime rules such as:

```text
homesignal/#
```

## Required Local Plan Checks

Every affected service plan should state:

- topic family
- cloud-to-device message class, if applicable
- Edge State Adapter/shadow boundary, if desired/reported edge state is involved
- publisher identity
- expected credential type
- AWS IoT rule route
- Basic Ingest vs normal broker path
- MQTT5 user properties
- schema version behavior
- publish budget behavior, if events are involved
- scoped publish-policy refresh behavior, if events are involved
- identity drift behavior
- failure/quarantine behavior
- contract examples

## V0 Decisions (Closed)

The following are intentionally settled for v0 and should be treated as
architecture lock-in:

- Command ACK/result runtime shape is the shared Agent HTTPS envelope defined in
  `api-facade.md` (for /agent routes) and is not topic-defined.
- Notifications remain a separate logical topic family in v0:
  `homesignal/devices/{device_id}/notifications`; commands remain
  `homesignal/devices/{device_id}/commands`.
- Unclaimed/device bootstrap remains HTTPS enrollment APIs in v0.
  If IoT-based bootstrap is introduced later, define the registration family in
  the owning boundary doc at that time.
- Raw archive routing/storage format is owned by `artifact-upload-broker.md`; no
  raw archive contract exists in topic design today.

## Acceptance Criteria

- Runtime and bootstrap topics cannot be confused.
- Ingest can route messages without trusting payload identity.
- Telemetry/events do not pay for broker pub/sub behavior unless a real subscriber/fanout requirement exists.
- Topic changes include contract example updates.
- AWS IoT rules are narrow enough to make accidental broad ingestion unlikely.
