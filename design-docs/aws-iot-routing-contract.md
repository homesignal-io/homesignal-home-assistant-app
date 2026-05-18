# AWS IoT Routing Contract

This document defines the coarse AWS IoT Core topic and rule contract for HomeSignal claimed-device command, notification, shadow, lifecycle, and optional future MQTT telemetry routes. It is intentionally separate from the Telemetry Ingest architecture so routing choices stay visible and testable.

Canonical device lifecycle, trust, and authority rules live in `workstreams/device-lifecycle.md`. This document defines the AWS IoT routing surface that must preserve those rules.

## Principles

- Route by MQTT topic family first.
- For claimed devices, HomeSignal `device_id`, AWS IoT Thing name, and required MQTT client ID are the same durable identifier.
- AWS IoT Core owns transport authentication, policy enforcement, and transport provenance.
- V0 routine telemetry/events use the mTLS Agent HTTPS API, not AWS IoT Basic Ingest.
- If a future runtime family moves to MQTT/Basic Ingest, use MQTT5 user properties as the canonical routing and schema metadata.
- Require MQTT5 for future claimed-device runtime publishers. MQTT3 publishers are not accepted for runtime telemetry or events.
- Use AWS IoT Rules SQL to enrich optional MQTT ingest messages with broker-known metadata and MQTT5 properties.
- Use AWS IoT Basic Ingest only when a runtime family deliberately chooses AWS IoT Rules routing instead of Agent HTTPS.
- Use AWS IoT named shadows through the Edge State Adapter for compact desired/reported edge state.
- Use the normal MQTT broker path for cloud-to-device notifications and commands because the app subscribes to them.
- Treat payload device identifiers as annotations, not authority.
- Keep registration/bootstrap topics separate from claimed-device runtime topics.

AWS IoT Rules can match logical topics in the `FROM` clause; Basic Ingest-only rules may also omit `FROM`. If optional MQTT ingest is introduced, HomeSignal should still keep the logical topic family explicit and narrow. MQTT5 user properties can be inspected and projected in `SELECT` and `WHERE`, but the topic family remains the first routing boundary.

Basic Ingest removes the publish/subscribe broker from a device-to-cloud MQTT ingestion path. It still uses AWS IoT Core authentication, device policies, MQTT publish handling, Rules SQL, rule actions, and error actions. It is an optional future route for telemetry/events when cost/scale or MQTT/rules semantics justify moving a family away from Agent HTTPS.

## Claimed Device Identity Binding

Claimed runtime publishing requires this identity binding:

```text
HomeSignal device_id == AWS IoT Thing name == MQTT client ID
```

The device may know and use the Thing/client ID, but it does not get to assert product identity through telemetry payload contents. AWS IoT Core must reject claimed-device runtime connections that do not use the authorized Thing/client ID for the certificate/principal.

Rules must enrich downstream messages with the durable device identity derived from authenticated transport context, for example by selecting `clientid()` as `device_id` when the policy requires client ID to equal Thing name. Additional AWS principal/certificate/session fields may be logged by infrastructure, but they are not HomeSignal product identity and are not required as a normal service join.

## Runtime Topic Families

V0 claimed devices report routine telemetry/events over Agent HTTPS. If a future runtime family uses MQTT, claimed devices must publish only to narrow runtime topic families:

```text
homesignal/devices/{device_id}/telemetry
homesignal/devices/{device_id}/events
```

Optional Basic Ingest publish topics:

```text
$aws/rules/homesignal_runtime_ingest/homesignal/devices/{device_id}/telemetry
$aws/rules/homesignal_runtime_ingest/homesignal/devices/{device_id}/events
```

For claimed devices, `{device_id}` in the topic must match the authenticated Thing/client ID. The IoT Rule-enriched identity is authoritative for ingest; payload-provided identity is not.

The logical topic family remains the suffix after the Basic Ingest prefix:

```text
homesignal/devices/{device_id}/telemetry
homesignal/devices/{device_id}/events
```

`telemetry` is reported current/latest state. It is snapshot-oriented, dedupe-friendly, and usually updates read models.

`events` are occurrences. They are append-oriented and represent something that happened, such as a Home Assistant event, `agent_alarm`, command lifecycle update, or notable app system event.

Cloud-to-device command delivery uses the normal MQTT broker path because the app subscribes to command messages. Device-originated command ACK facts may return through the Agent HTTPS API or another command-specific authenticated return path. Artifact upload completion/result uses the mTLS Agent HTTPS API.

AWS IoT named shadow reserved topics are not HomeSignal runtime telemetry/event topic families. They are owned through `edge-state-adapter.md`.

## Named Shadows And Edge State

HomeSignal uses AWS IoT named shadows for compact desired/reported edge state in v0.

The Edge State Adapter owns:

- named shadow selection and JSON shape
- cloud writes to shadow `desired`
- observation of device writes to shadow `reported`
- compact convergence projection into HomeSignal storage

Product services must not write AWS IoT shadows directly. They commit product truth in HomeSignal storage and ask the Edge State Adapter to update the device-facing desired projection.

Use named shadows for small durable edge targets such as active publish-policy version, enabled telemetry/event families, telemetry cadence, and future compact device policy/version pointers. Do not use shadows for raw telemetry, events, logs, topology blobs, diagnostics bundles, backup payloads, release artifacts, signed URLs, or secrets.

## Cloud-To-Device Message Classes

HomeSignal has two logical cloud-to-device classes:

| Class | Contract | Examples |
| --- | --- | --- |
| Notification | Fire-and-forget hint. No ACK expected. Cloud must not assume local state changed because a notification was published. | "Policy changed, check in soon", reconnect nudges, low-risk hints. |
| Command | ACK/result required. Has command identity, expiry, allowlisted type, short-window accepted/rejected ACK, and terminal result reporting from the app. | `refresh_publish_policy`, backup trigger, update status/repair where explicitly supported, release/revoke convergence, future diagnostics or artifact upload. |

Default cloud-to-device topic families:

```text
homesignal/devices/{device_id}/notifications
homesignal/devices/{device_id}/commands
```

Active publish-policy version is durable desired state and should normally be delivered through the Edge State Adapter and AWS IoT named shadows. A notification may be used only as a convergence hint. `refresh_publish_policy` remains command-class when cloud needs a bounded repair/acceleration attempt with ACK/result semantics.

The first ACK path should use the authenticated Agent HTTPS API unless a command-specific spec chooses another authenticated return path. ACK means app accepted or rejected the command, not mere packet receipt. Artifact upload completion/result uses the mTLS Agent HTTPS API. Command lifecycle semantics are defined in `command-lifecycle.md`.

Do not deploy broad catch-all runtime rules such as:

```text
homesignal/#
```

Do not use Basic Ingest for messages that require MQTT subscribers, broker fanout, retained messages, or persistent-session delivery.

## Basic Ingest Tradeoff

Use Basic Ingest only for future device-to-cloud runtime families that deliberately choose AWS IoT Rules routing instead of Agent HTTPS.

What HomeSignal saves:

- AWS IoT messaging/broker charges for the Basic Ingest reserved-topic publish.
- Paying for pub/sub distribution behavior that telemetry/events do not use.

What HomeSignal still pays for:

- AWS IoT connectivity.
- AWS IoT Rules Engine rule evaluations and actions.
- Downstream targets such as Lambda, SQS, HTTP endpoints, logs, or storage.

What HomeSignal gives up for those telemetry/event publishes:

- MQTT subscribers cannot subscribe to the Basic Ingest reserved topics.
- No broker fanout to other MQTT clients.
- No retained messages for those telemetry/event topics.
- No persistent-session queued delivery to offline MQTT subscribers.
- The device publish topic includes the IoT Rule name, so rule naming becomes device configuration.
- QoS 1 PUBACK means the Rules Engine received the message, not that HomeSignal persisted it.

If HomeSignal later needs MQTT subscribers or broker fanout for a runtime family, that family should move to a normal broker topic deliberately. Do not use normal broker topics by habit for one-way device reports.

## IoT-To-Ingest Handoff

AWS IoT routing does not own HomeSignal observability/debug semantics. It
authenticates, enforces IoT policy, enriches with broker-known metadata, and
delivers lifecycle or optional runtime messages to Telemetry Ingest.

Telemetry Ingest then:

- normalizes the IoT-originated message into the internal ingest envelope
- derives product identity from authenticated IoT context, not payload claims
- applies publish policy, debug sessions, watch rules, and failure
  classification
- attaches internal ingest annotations for capture/logging/retention
- writes Postgres read models, failure rows, support/debug capture references,
  counters, and alert candidates as appropriate

Do not encode active debug/watch lists into AWS IoT Rules as the primary
control plane. IoT rules should stay narrow and stable; HomeSignal ingest owns
runtime capture and retention decisions.

## MQTT5 Required Metadata

Every claimed-device runtime publish must include exactly one value for each required user property:

| Property | Required values / meaning |
| --- | --- |
| `message_type` | `telemetry` or `event`; must agree with the topic suffix |
| `schema_type` | Exact contract name, such as `device.health_snapshot` |
| `schema_version` | Positive integer encoded as a string |
| `message_id` | Device-generated opaque ID, preferably ULID or UUID |
| `applied_publish_policy_version` | Opaque server-issued publish-policy version the app enforced |
| `observed_at` | Device-observed RFC3339 timestamp for the facts |

Recommended MQTT5 standard properties:

| Property | Default |
| --- | --- |
| `content_type` | `application/json` |
| `payload_format_indicator` | UTF-8 data, when supported by the client library |

Initial telemetry schema types:

```text
device.health_snapshot
```

Initial event schema types:

```text
ha_event
agent_alarm
command_lifecycle
system_event
```

Event schema types are allowlisted. Do not accept a broad, client-defined
`ha_event` stream in v0.

`agent_alarm` is an internal security/abuse signal in v0, not a direct user-visible alert. It must include a required severity:

```text
info
warning
critical
```

Repeated identical `agent_alarm` events should collapse by device, alarm type, and time window while preserving first seen, last seen, count, and sample provenance.

`agent_alarm` also covers app enforcement failures that HomeSignal should know about even when they are not customer-visible. Initial alarm types include:

```text
potential_abuse_detected
publish_policy_apply_failed
publish_policy_rejected_suspicious
artifact_upload_failed
```

Policy-apply alarms must carry only bounded structured fields such as attempted policy version, active policy version, command ID, reason code, and severity. They may include a redacted diagnostic excerpt capped at 5 KB total. If additional logs exist locally, set `more_logs_available=true` and include a local correlation/reference ID that a later artifact-upload command can reference. Do not include large logs, full policy documents, signed URLs, secrets, or local file contents beyond the bounded redacted excerpt in MQTT payloads.

Artifact-upload failure alarms must not recursively request more logs. If an error occurs while uploading an `error_log_bundle`, the app may emit one bounded `artifact_upload_failed` event with `more_logs_available=false`, then collapse repeats by device, artifact purpose, failure reason, and time window.

Event publish budgets are provisioned by the cloud and enforced hard by the app from the last accepted policy. Cloud ingest independently enforces current server policy, so entitlement changes apply in cloud immediately even if the app has not refreshed yet.

The app receives resolved effective publish policy, not plan/tier labels. Policy can enable interval-based telemetry snapshots and separate live event budgets. In v0, paid/live events are modeled but not exposed: snapshots plus internal `agent_alarm` are the practical runtime surface.

Conservative default when publish policy is missing, expired, unreadable, or invalid:

- allow low-rate `telemetry` with `schema_type=device.health_snapshot`
- include backup summary only inside the low-rate snapshot when available
- allow `agent_alarm` under a strict security budget
- disable live `ha_event`
- disable paid/live event behavior

Routine over-budget events are dropped/counted by cloud ingest after enough parsing to classify safely. Structurally suspicious or security-relevant events are quarantined. Sustained over-budget publishing creates an internal security/abuse signal after a threshold.

Cloud ingest may request a scoped `refresh_publish_policy` command for sustained over-budget publishing. This command may refresh only event/telemetry publish allowlists, budgets, policy version, and expiry. It must not change command permissions, host-write/local execution policy, credentials, enrollment/claim state, update policy, backup policy, diagnostics policy, or remote access policy.

Missing required properties, duplicate values for required keys, invalid enum values, schema mismatches, or topic/property disagreement must be rejected or quarantined before product state changes. Exception: `agent_alarm` missing `applied_publish_policy_version` may be accepted as an internal signal with `applied_publish_policy_version=unknown`, while recording a contract-defect metric.

The AWS IoT MQTT5 user property quota is 8KB total per packet. That is enough for runtime routing/schema metadata. Do not put business payloads, secrets, stack traces, diagnostics blobs, or large state snapshots in user properties.

Large logs or diagnostic detail beyond the 5 KB event excerpt must move through a separate artifact path, not MQTT. The device may emit a small alarm/event that says a larger diagnostic artifact is available; the cloud must authorize and broker that upload deliberately.

## Bootstrap And Registration Topics

Unclaimed or bootstrap credentials must not publish to runtime telemetry or event topics.

If IoT-based registration is added later, use a separate topic family and route it to a registration/enrollment service:

```text
homesignal/registration/{claim_or_installation_id}/...
```

Required split:

```text
bootstrap/claim credential
  can call only HTTPS enrollment/claim endpoints

durable claimed credential
  can publish only its own runtime device topics

Telemetry Ingest rules
  match runtime telemetry/events topics only

Registration rules
  are not part of the runtime telemetry/event topic family
```

The current default remains HTTPS enrollment plus HomeSignal-authorized AWS IoT CSR signing/provisioning and HomeSignal finalization. Telemetry Ingest is post-claim only.

## Lifecycle Topics

AWS IoT Core lifecycle topics are consumed for connectivity presence:

```text
$aws/events/presence/connected/{clientId}
$aws/events/presence/disconnected/{clientId}
$aws/events/presence/connect_failed/{clientId}
```

Lifecycle events are routed to Telemetry Ingest because they affect device presence. For claimed devices, `clientId` is the durable Thing name/`device_id`; ingest uses that enriched identity before updating product state.

## Rule-Enriched Message Shape

AWS IoT Rules should wrap or project AWS IoT metadata before sending to the ingest receiver.

Minimum operational metadata:

```json
{
  "device_id": "dev_123",
  "client_id": "mqtt-client-id",
  "topic": "homesignal/devices/dev_123/telemetry",
  "raw_publish_topic": "$aws/rules/homesignal_runtime_ingest/homesignal/devices/dev_123/telemetry",
  "received_at": 1770000000000,
  "payload": {}
}
```

Recommended metadata:

```json
{
  "device_id": "dev_123",
  "client_id": "mqtt-client-id",
  "topic": "homesignal/devices/dev_123/telemetry",
  "raw_publish_topic": "$aws/rules/homesignal_runtime_ingest/homesignal/devices/dev_123/telemetry",
  "topic_device_id": "dev_123",
  "received_at": 1770000000000,
  "mqtt5": {
    "content_type": "application/json",
    "user_properties": {
      "message_type": ["telemetry"],
      "schema_type": ["device.health_snapshot"],
      "schema_version": ["1"],
      "message_id": ["01J00000000000000000000000"],
      "applied_publish_policy_version": ["ppv_01J00000000000000000000000"],
      "observed_at": ["2026-05-14T12:00:00Z"]
    }
  },
  "payload": {}
}
```

The rule should use AWS IoT metadata where available:

```text
principal()
clientid()
topic()
timestamp()
topic segment extraction for topic_device_id
get_mqtt_property() for standard MQTT5 properties
get_user_properties() for runtime envelope user properties
```

For Basic Ingest, the `$aws/rules/{rule_name}` prefix is not visible to the `topic()` function. The rule should preserve the logical topic and, if useful for debugging, separate raw publish topic or rule provenance from the receiving adapter.

AWS IoT returns user-property values as arrays because MQTT5 allows duplicate keys. Telemetry Ingest must normalize required keys by requiring exactly one value for each required runtime envelope property.

## Payload Self-Description

Device payloads may repeat runtime contract and device annotations for raw archive usefulness:

```json
{
  "contract": {
    "message_type": "telemetry",
    "schema_type": "device.health_snapshot",
    "schema_version": 1
  },
  "device": {
    "homesignal_device_id": "dev_123",
    "installation_id": "inst_123",
    "agent_version": "0.1.3"
  },
  "observed_at": "2026-05-11T12:00:00Z",
  "payload": {}
}
```

This helps later object-storage archive, replay, and analytics flows inspect raw events without requiring an immediate database join.

For optional MQTT ingest, MQTT5 properties remain canonical for routing and schema selection. If payload contract annotations disagree with MQTT5 properties, reject or quarantine the message.

Payload device annotations do not authorize the message. Optional MQTT ingest uses the AWS IoT Rule-enriched identity:

```text
clientid() / Thing name -> device_id
```

If payload or topic device annotations disagree with the rule-enriched `device_id`, the message is identity drift and must not update product state.

## Contract Examples

Every optional MQTT topic/schema change should include contract examples. These are the "fixtures" for this boundary: sample MQTT publishes with topic, MQTT5 user properties, payload, and expected ingest outcome.

Minimum contract examples:

- accepted telemetry snapshot
- accepted event
- missing required user property
- duplicate required user property
- unsupported schema version
- topic/property family mismatch
- topic device ID mismatch
- revoked or unknown credential identity

## Raw Archive Replication

Raw archive is future, not MVP.

Possible future routing:

```text
AWS IoT Core Basic Ingest
  -> operational rules -> ingest receiver -> Telemetry Ingest -> Postgres
  -> archive rules     -> object storage       -> analytics/replay/backfill
```

Archive objects should preserve:

- raw payload
- logical topic
- raw publish topic or rule provenance, if available
- provider identity key
- client ID
- thing name, if available
- MQTT5 standard properties
- MQTT5 user properties
- received time
- AWS account/region/rule provenance

The operational Postgres path should stay optimized for current product state. The archive path can be append-only and richer.

## AWS Notes

Official AWS docs currently state that:

- AWS IoT Core supports MQTT5 and user properties for `PUBLISH`.
- AWS IoT Basic Ingest sends device data to AWS IoT rule actions without messaging costs by removing the publish/subscribe broker from the ingestion path.
- Basic Ingest topics start with `$aws/rules/{rule_name}` and cannot be subscribed to.
- Basic Ingest QoS 1 PUBACK means delivery to the Rules Engine, not completion of downstream rule actions.
- AWS IoT Rules SQL functions can be used in `SELECT` and `WHERE`.
- `get_user_properties(key)` returns an array of matching values.
- The MQTT5 user properties total-size quota is 8KB per packet.

References:

- <https://docs.aws.amazon.com/iot/latest/developerguide/mqtt.html>
- <https://docs.aws.amazon.com/iot/latest/developerguide/iot-basic-ingest.html>
- <https://docs.aws.amazon.com/iot/latest/developerguide/iot-sql-functions.html#get_user_properties>
- <https://docs.aws.amazon.com/iot/latest/developerguide/iot-sql-where.html>
- <https://aws.amazon.com/iot-core/pricing/>
- <https://docs.aws.amazon.com/general/latest/gr/iot-core.html>

## Acceptance Criteria

- Future MQTT runtime messages require MQTT5.
- V0 device-to-cloud telemetry/events use Agent HTTPS by default. Basic Ingest is optional/future.
- Cloud-to-device notifications and commands use the normal MQTT broker path.
- Runtime rules match only claimed-device runtime topic families.
- Registration/bootstrap topics, if added, route to registration/enrollment, not Telemetry Ingest.
- Lifecycle topics route to Telemetry Ingest.
- Ingest receives provider identity metadata with every runtime message.
- Optional MQTT ingest can route and select schema from MQTT5 properties without trusting payload fields.
- Required MQTT5 user properties are present exactly once when optional MQTT ingest is used.
- Payload may include contract and device annotations for archive/replay, but mismatches become rejection/quarantine or identity drift.
- No broad `homesignal/#` rule is required for MVP runtime ingestion.
