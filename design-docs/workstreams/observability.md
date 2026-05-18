# Observability Workstream

Observability is a platform obligation, not a service-specific enhancement. Every service should be understandable in production from logs, metrics, traces or spans, health checks, and audit records appropriate to its authority.

## Agent Use

Read this when creating or changing:

- service entrypoints
- HTTP routes
- background workers
- queues or event consumers
- MQTT or AWS IoT adapters
- deployment plans
- error handling
- health checks
- audit-producing flows

## Current Anchors

- `service-map.md`
- `platform-health-monitoring-service.md`
- `telemetry-ingest-architecture.md`
- `telemetry-ingest-build-plan.md`
- `aws-iot-routing-contract.md`
- `auth-service.md`
- `workstreams/deployment-readiness-matrix.md`

## Principles

- A service is not production-ready until it can explain its own behavior.
- Logs are for investigation, metrics are for alerting and trends, traces are for cross-boundary flow, audit events are for authority history.
- Platform Health / Monitoring interprets cross-service signals into internal findings; observability only provides the signal substrate.
- Correlation must survive HTTP, queue, worker, and device-ingest boundaries where practical.
- Sensitive values must never be logged.
- Health checks should distinguish process liveness from dependency readiness.
- Observability should be designed with local fixtures and staging in mind, not added only after deployment.

## Implementation Defaults

- Use structured logs.
- Include a request or correlation ID on inbound requests.
- Propagate correlation IDs through outbound service calls and queue messages.
- Include service name, environment, version, and route/worker name in logs.
- Include account, site, device, and actor identifiers only when authorized and safe to log.
- Redact tokens, private keys, claim invite codes, claim verification tokens, certificates, Authorization headers, cookies, and raw secrets.
- Provide `/healthz` for liveness and `/readyz` for dependency readiness on HTTP services.
- Emit metrics for request counts, latency, error counts, queue lag where a
  queue exists, retry counts, dead-letter counts where a dead-letter path
  exists, dependency failures, and domain-specific state transitions.
- Emit audit events separately from operational logs for sensitive authority changes.
- Emit bounded counters suitable for future platform health monitors, especially per-device/per-account runtime publish rates, rejection counts, quarantine counts, command ACK absence, artifact upload failures, and suppression counters.

## V0 Value-Engineered Posture

V0 observability should answer operational questions without making every
telemetry/event message a billable observability object.

Use:

- CloudWatch Metrics and alarms for coarse service health, request/error rates,
  dependency failures, command outcomes, ingest accept/drop/reject counts, and
  artifact failures.
- CloudWatch Logs for short-retention structured service logs.
- App DB records for command, audit, device status, artifact status, and product
  truth.
- S3 for cold diagnostic bundles, sampled summaries, and support artifacts.
- Targeted temporary debug mode for richer per-device/per-site detail.

Avoid by default:

- per-message distributed tracing
- per-message CloudWatch custom metrics
- high-cardinality CloudWatch metric dimensions such as `device_id`, `site_id`,
  or `org_id`
- always-on verbose device logs
- paid external observability platforms until AWS-native signals are proven
  insufficient

CloudWatch metrics should use coarse dimensions such as service, environment,
route template, status class, `message_type`, `schema_type`, command type, and
reason code.
Device/site/account identifiers belong in product records, structured logs, S3
summaries, or debug sessions, not hot metric dimensions.

V0 backend defaults:

- CloudWatch Metrics is the default metrics backend.
- CloudWatch Logs is the default structured service log backend.
- Do not enable default distributed tracing in v0. Add targeted tracing only for
  a concrete investigation or high-value flow.
- Retain structured service logs for 7 days in staging and 14 days in
  production unless a stricter security/product reason overrides it.
- MVP alarms cover service health/readiness, elevated 5xx/error rates, ingest
  rejects/quarantine spikes, Agent HTTPS auth/cert failures, command
  ACK/result failures, artifact failures, and database/dependency failures.
- The launch-critical v0 alarm and smoke-check inventory is maintained in
  `deployment-readiness-matrix.md`; service plans should satisfy that matrix
  before adding service-local alarm rules.
- Do not create per-device, per-site, or per-message CloudWatch custom metrics
  by default.

## Degraded UX Posture

V0 does not productize detailed degraded/convergence UX. The platform should
store enough state for support and future UI design, but the customer-facing UI
should not expose raw internals such as policy drift, command no-ACK,
convergence windows, or Platform Health findings.

Near-term product surfaces may show simple device/app status derived from
latest telemetry and presence, such as healthy, disconnected, updating, backup
failing, or needs attention. Detailed causes, operator notes, debug sessions,
policy convergence state, and no-ACK behavior remain internal/support-visible
until a UI/product spec defines the language and customer actions.

## Ingress Annotations And Log-Level Response

Devices are not the authority for deciding that a message deserves extra cloud
attention. A device may send ordinary envelope fields, policy version, routine
log level, and payload facts, but the cloud ingress boundary decides whether a
message is captured, retained, elevated, dropped, or quarantined.

Ingress-side annotation is the pattern:

```text
message arrives
  -> authenticate and derive device/site/org identity
  -> parse and classify envelope
  -> check current publish policy and debug/watch context
  -> attach internal ingest annotations
  -> enforce retention/logging/drop/quarantine behavior
  -> pass normalized facts to downstream services
```

Example internal annotation:

```json
{
  "ingest_annotations": {
    "capture_level": "debug",
    "capture_reason": "active_debug_session",
    "debug_session_id": "dbg_123",
    "retention": "debug_copy",
    "matched_rule": "device_debug_session"
  }
}
```

These annotations are internal metadata. They are not trusted from the device
payload and should not leak into product telemetry unless a service deliberately
stores a derived support/debug record.

Annotation sources:

| Source | Meaning |
| --- | --- |
| Publish policy | Standing device observability level, event/log budget, diagnostic excerpt allowance, and policy freshness. |
| Debug session | Temporary device/site support capture with TTL, allowed categories, redaction profile, and artifact limits. |
| Watch rule | Optional internal support/ops rule for a device/site/account/schema/reason-code/time window. |
| Ingest classification | Security or quality decision such as schema rejection, identity drift, over-budget, suspicious payload, or quarantine. |

Standard capture levels:

| Capture level | Cloud behavior |
| --- | --- |
| `none` | Do not retain beyond normal counters/drop accounting. |
| `normal` | Apply ordinary product-state writes, aggregate metrics, and short operational logs for errors. |
| `support` | Retain sparse support record or structured log sample with bounded payload. |
| `debug` | Retain richer debug copy or route to debug-session storage/S3 summary according to TTL and budget. |
| `quarantine` | Do not update product state; retain bounded failure metadata for investigation. |

Service response rules:

- Telemetry Ingest attaches and enforces the annotation.
- Downstream services should receive normalized facts and storage decisions, not
  raw "please log harder" requests.
- CloudWatch log level should be elevated only for bounded support/debug or
  quarantine cases, not for every message in a noisy category.
- CloudWatch metrics remain coarse; debug/watch annotations must not create
  high-cardinality metric dimensions.
- S3/debug storage is used for larger retained debug copies or bundles.
- App DB stores command/status/audit/product truth and sparse support/debug
  references, not unbounded raw message streams.

If a message matches an active debug/watch context, the service may log more
about processing that message, but it must still redact secrets and respect the
session's size, TTL, category, and retention limits.

## Temporary Debug Mode

Temporary debug mode is the escape hatch for unusual device/site issues. It is
not a customer self-service toggle.

Purpose:

- temporarily increase app diagnostic detail for one device or site
- capture bounded logs, message samples, command traces, health checks, and
  local environment facts needed for support/debugging
- ship larger debug output through the approved artifact upload path
- avoid paying for fleet-wide forensic observability all the time

Ownership:

- Observability defines log/metric/debug signal shape, redaction, retention, and
  cost posture.
- Diagnostics owns the product meaning of debug sessions and diagnostic
  commands.
- API Facade exposes the operator/support route and enforces authentication.
- Authorization Service decides whether the operator/support actor may start or
  stop a debug session.
- Command Service delivers the bounded debug command and tracks ACK/result.
- Artifact Upload Broker handles any debug bundle upload.
- App enforces local debug policy, TTL, redaction, rate limits, and upload
  recursion guards.
- App DB stores debug session state, command IDs, artifact IDs, audit records,
  and status.

V0 flow:

```text
operator/support actor requests debug session
  -> API authenticates actor and builds request context
  -> AuthorizationService authorizes internal/support debug action
  -> Diagnostics creates debug_session row with device/site scope and TTL
  -> Command Service sends enable_debug_capture command over IoT Core
  -> app ACKs accepted/rejected through Agent HTTPS
  -> app captures bounded debug detail until expires_at or stop command
  -> app emits small summaries through normal telemetry/event path
  -> larger bundles use Artifact Upload Broker and S3
  -> command result/debug session status update App DB
```

V0 rules:

- Debug mode must be time-boxed. The default TTL is 1 hour. The hard maximum is
  24 hours, and extensions must be explicit.
- Debug mode must be scoped to device or site; fleet-wide debug mode is not v0.
- Customer users cannot enable debug mode directly.
- Debug-mode start, stop, extension, and artifact request are audited.
- Debug mode must not permit arbitrary file upload, arbitrary shell execution,
  or broad Home Assistant configuration snapshots.
- Debug output must be redacted before upload and must never include private
  keys, claim invite codes, tokens, signed URLs, cookies, raw secrets, or full local
  config dumps.
- The app must automatically turn debug mode off at expiry even if cloud is
  unreachable.
- Debug mode must have local rate/size limits so it cannot create a device-side
  or cloud-side cost storm.
- Debug mode should use command ACK/result semantics; it is not proof of local
  state until ACK/result or artifact completion is recorded.
- Debug artifacts are retained for 14 days by default. Debug session metadata,
  support summaries, and command/result records may be retained for 90 days
  unless a stricter product/security policy applies.
- Debug TTLs, artifact limits, retention periods, and standing device
  observability levels are policy/config records. They should be auditable and
  reviewable in an internal/admin surface later; services should not hide these
  values as unreviewable constants.

Candidate debug commands:

- `enable_debug_capture`
- `disable_debug_capture`
- `collect_debug_snapshot`
- `request_debug_bundle`

Candidate debug artifacts:

- `debug_bundle`
- `error_log_bundle`
- `diagnostic_bundle`

Telemetry retention:

V0 should store user/product-facing telemetry needed for device history and UI
separately from observability logs. Keep latest-state projections in the app DB
and store bounded historical telemetry according to product retention policy.
Do not use CloudWatch as the primary user telemetry store.

Product telemetry retention default:

- Keep latest device state in Postgres while the device exists.
- Keep sparse historical telemetry hot in Postgres for 7 days.
- Archive verified daily plain NDJSON plus optional summaries to device-rooted
  object storage after the hot window.
- On device deletion, delete product telemetry history and archives after a
  7-day operational grace period.
- Audit, security, billing, and authority records are retained separately with
  minimized payloads and must not preserve raw telemetry merely because they
  reference the deleted device.

## Device Observability Policy

Routine device-originated logging and health verbosity belongs in publish policy,
not in ad hoc debug commands or platform log-level config.

Reason:

- publish policy already has versioning, freshness, expiry, local enforcement,
  and cloud ingest backstops
- device verbosity has direct cost and abuse implications
- stale/missing policy already falls conservative
- each runtime message can report `applied_publish_policy_version`

Device observability policy controls:

- health snapshot cadence
- backup-summary inclusion in routine telemetry
- routine runtime log/event allowance
- minimum log level for routine device-originated log events
- max events and bytes per time window
- diagnostic excerpt allowance and size
- `agent_alarm` budget

Standing policy levels should be conservative: `quiet`, `normal`, and
`verbose`. Rich `debug` capture remains a temporary debug session, not a
standing publish-policy level.

Device-originated log events should be structured and collapsed by component,
reason code, and time window. They must not be raw log streams.

Example shape:

```json
{
  "message_type": "telemetry",
  "schema_type": "device.health_snapshot",
  "schema_version": 1,
  "message_id": "01J00000000000000000000000",
  "applied_publish_policy_version": "pp_123",
  "observed_at": "2026-05-14T20:20:00Z",
  "payload": {
    "agent": {
      "status": "degraded"
    },
    "runtime_log_summary": [
      {
        "level": "warning",
        "source": "agent",
        "component": "cloud_connection",
        "reason_code": "reconnect_backoff",
        "sample_message": "Cloud reconnect backoff active",
        "occurrence_count": 3,
        "first_seen_at": "2026-05-14T20:10:00Z",
        "last_seen_at": "2026-05-14T20:20:00Z"
      }
    ]
  }
}
```

Routine device logs must not include secrets, private keys, claim invite codes,
tokens, signed URLs, cookies, raw Home Assistant config, or unbounded stack
traces. If more detail is needed, the device should send a small
`more_logs_available` hint and wait for a cloud-authorized artifact/debug flow.

## V0 Audit Candidates

Audit events should be listed by each owning service before implementation. Mandatory v0 audit events must ship with their owning flows; additional candidates may be staged, but sensitive authority changes should not disappear into operational logs.

Mandatory v0 audit events:

- user invite, membership change, role assignment, and role removal
- sensitive authorization denial, such as denied device claim, release/revoke, command issue, artifact request, update rollout intent/status action, or role/membership change
- claim invite creation, email send attempt, cancellation, and expiry
- claim invite verification and confirmation attempt, success, and failure
- device claim finalization
- device repair/reconnect
- device fresh claim
- device release, revoke, credential disablement, or credential rotation
- command issued for sensitive command classes
- artifact requested
- update rollout intent/status action triggered

Additional high-value v0 audit candidates:

- account creation, archive, or deactivation
- site creation, archive, or deactivation
- site owner/manager/support-provider relationship add, end, or failed invariant check
- command rejected by authorization, accepted by device, failed, expired, or succeeded for sensitive command classes
- backup trigger terminal result
- update rollout intent change, terminal result, and rollback
- publish-policy change that materially changes device event/telemetry allowance
- admin/remediation action

Operational logs remain separate from audit. Runtime telemetry, health snapshots, agent alarms, and platform-health findings are not automatically audit records unless they represent or cause an authority change.

## Required Local Plan Checks

Every affected service plan should state:

- log fields and redaction rules
- metrics emitted
- health and readiness checks
- correlation ID behavior
- retry/dead-letter visibility where applicable
- audit events, if sensitive actions exist
- staging verification for observable failure paths
- whether the emitted signals support future Platform Health / Monitoring rules

## V0 Decisions (Closed)

- Platform Health / Monitoring rule storage and evaluation cadence are future
  implementation requirements.
- V0 services should emit/store the source facts named in
  `platform-health-monitoring-service.md`; a later slow evaluator can read those
  facts on a windowed cadence and create internal findings.
- Do not put Platform Health evaluation in the hot ingest path.

## Acceptance Criteria

- Each service has a documented health/readiness surface.
- Sensitive flows have audit events or an explicit reason they do not.
- Queue and worker failures are visible without reading raw infrastructure internals.
- Fixture or staging tests can exercise at least one success path and one failure path with observable output.
