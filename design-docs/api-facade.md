# API Facade Service Spec

The API Facade is the public HTTP ingress and external contract owner for HomeSignal. It is a logical service boundary. In v0 it should be implemented in one physical control-plane backend with the domain services as internal modules. Telemetry Ingest is the v0 exception and should be separately deployable because it owns the runtime device telemetry/event path behind the authenticated Agent HTTPS API.

The API Facade owns protocol. Domain services own product meaning.

## Agent Use

Read this before adding or changing any public `/api/v1/*` route, agent `/agent/*` route, or internal `/internal/*` route.

Agents must use this spec to decide:

- whether a route belongs in the public API or internal API
- which domain service owns the route's business behavior
- which authentication, authorization, idempotency, rate-limit, and response rules apply
- whether OpenAPI must be updated
- whether a direct app API call is allowed in v0

Do not implement public route behavior that is not represented in OpenAPI.

## Current Position

v0 uses:

```text
one physical control-plane backend service
  -> API Facade routes
  -> internal logical domain services
  -> repositories/storage adapters

one separately deployable Telemetry Ingest service
  -> Agent HTTPS telemetry/event receiver
  -> runtime telemetry/event validation
  -> latest-state/event writes and alert candidate handoff
```

Logical services may later split physically when deployment, scaling, security, or ownership requires it. Public route shape should not expose that split.

## Owns

The API Facade owns:

- public HTTP route surface
- agent-authenticated HTTP route surface
- internal HTTP route surface
- public API compatibility policy
- OpenAPI contract for public routes
- authentication for inbound HTTP credentials
- request validation and response formatting
- standard request context construction
- route-level idempotency handling
- v0 in-process rate limiting
- request IDs and correlation IDs
- operational request logging
- delegation to one owning domain/workflow method per mutation
- composed read-model response contracts for portal/app sanity

For temporary debug mode, the API Facade owns the operator/support HTTP route
shape and request validation. Diagnostics owns debug-session product behavior,
Authorization Service owns permission decisions, Command Service owns device
command lifecycle, and Artifact Upload Broker owns debug bundle transfer.
Customer users must not be able to enable debug mode directly.

For message-level debug/watch behavior, API Facade may expose operator/support
routes to create a debug session or internal watch rule, but the runtime message
flagging happens in Telemetry Ingest. API routes create policy/session records;
they do not mark individual telemetry messages.

## Does Not Own

The API Facade does not own:

- authorization decisions
- domain state transitions
- domain orchestration
- direct Postgres reads or writes
- durable audit history for business actions
- device runtime ingest
- MQTT topic design
- S3 object authority beyond future brokered contracts
- cloud-to-device command execution
- local Home Assistant behavior

## Namespaces

Public client routes use:

```text
/api/v1/...
```

Internal service, worker, and hook routes use:

```text
/internal/...
```

Agent-authenticated routes use:

```text
/agent/...
```

Public routes are for portal, app bootstrap, and other client-facing product APIs. Agent routes are for claimed devices authenticated with mTLS. Internal routes are for trusted infrastructure, service identities, workers, and AWS hooks. Browser and app clients must not call `/internal/*`.

`/internal/*` routes must not be internet-facing. When services are physically
split, internal routes are reachable only through trusted AWS/service networking
or equivalent private integration, plus IAM/SigV4 service authentication.

## Compatibility Policy

`/api/v1` should evolve additively.

Allowed additive changes include:

- adding optional response fields
- adding optional request fields
- adding new endpoints
- adding new enum values only when clients are expected to tolerate unknown values

Breaking changes require explicit architecture review before introducing a new version. Breaking changes include:

- removing or renaming public fields
- changing field meanings
- changing authentication or authorization semantics
- changing workflow state meanings
- changing error codes in a way clients depend on
- changing route behavior from synchronous to asynchronous without a compatibility plan

## OpenAPI

Public `/api/v1/*` routes are OpenAPI-first.

Rules:

- Every public route must exist in the source-controlled OpenAPI spec before implementation.
- Public behavior changes must update OpenAPI in the same change.
- Internal `/internal/*` routes are excluded from public OpenAPI and may start in markdown specs.
- Route examples must use typed HomeSignal IDs, not database IDs.

## Request Lifecycle

Every request follows this conceptual order:

```text
assign request_id
resolve or generate correlation_id
perform operational logging setup
authenticate inbound credential
validate method, content type, body, params
apply route rate limit
apply idempotency policy when required
resolve subject and target resource
call AuthorizationService when route requires permission
delegate to owning domain/read service
format success or standard error response
emit operational log
```

## Authentication And Authorization

The API Facade owns authentication for inbound HTTP credentials:

- Cognito JWTs for human users.
- Claim verification tokens for unauthenticated app enrollment confirmation.
- Verified client certificates for `/agent/*` routes.
- AWS IAM/SigV4-authenticated service principals for AWS-hosted `/internal/*`
  routes when calls cross process boundaries.

Domain services validate domain-specific credential meaning. For example, the API extracts a claim verification token, but Enrollment / Device Registry validates the token hash, verification status, expiry, presented details hash, and allowed transition.

For `/agent/*`, the API Facade authenticates the client certificate presented by the edge and derives device identity from trusted certificate metadata. Route handlers must never trust `device_id`, `site_id`, or `org_id` from request bodies.

For `/internal/*`, AWS-hosted callers should authenticate with IAM/SigV4 rather
than static shared API keys. The API maps the verified AWS principal or assumed
role to a HomeSignal service subject and passes that subject in `RequestContext`.
Internal route handlers still use `AuthorizationService.can(...)` for sensitive
work.

Authorization decisions belong to Authorization Service:

```text
AuthorizationService.can(subject, action, resource, context)
```

Route handlers must not implement ad hoc role checks.

## Request Context

The API Facade builds a standard request context and passes it to every domain service.

Required context:

```text
request_id
correlation_id
correlation_id_source
actor
auth_method
source_ip
user_agent
route_template
idempotency_key, when present
```

Additional agent-authenticated context:

```text
device_id
device_certificate_fingerprint
device_certificate_serial
device_site_id
device_org_id
```

`request_id` is generated by the API for every inbound request and returned as:

```text
X-Request-ID
```

`correlation_id` is accepted only from:

```text
X-Correlation-ID
```

If the caller provides a valid `X-Correlation-ID`, the API preserves and echoes it. If the value is missing or invalid, the API generates and returns a new one. Caller-provided values must be length-bounded and must not contain control characters.

## Error Envelope

All public and internal errors use the standard error envelope:

```json
{
  "error": {
    "code": "CLAIM_INVITE_EXPIRED",
    "message": "The claim invite has expired.",
    "request_id": "req_123",
    "details": {}
  }
}
```

`code` is stable and machine-readable. `message` is human-readable. `request_id` is always present. `details` is optional and must not expose secrets.

## Rate Limiting

v0 has no required load balancer or gateway rate limiter, so the API Facade owns rate limiting. A single-instance v0 can start with in-process counters, but public claim-invite creation, verification, and confirmation limits must move to shared storage before horizontally scaled or production email-enabled deployment.

Minimum route-class limits:

| Route class | Scope |
| --- | --- |
| Claim invite creation | actor, account, site, recipient email, source IP |
| Claim invite verification | source IP, installation ID, claim code fingerprint |
| Claim verification confirmation | source IP, installation ID, claim verification ID |
| Agent routes | device ID, certificate fingerprint, route template |
| Auth-sensitive mutations | actor, target resource, action |
| Internal hook endpoints | service identity, source/network context |
| General API fallback | actor or source IP |

Claim invite limits are conjunctive: creation, verification, and confirmation
requests must pass every relevant bucket, not merely one coarse actor or source
IP bucket. Public verification must support progressive delay/backoff and
generic unavailable errors so rate limiting does not become a code-existence
oracle.

Rate limit failures return:

```text
HTTP 429
Retry-After
standard error envelope
```

## Idempotency

High-risk or retry-prone mutation routes require:

```text
Idempotency-Key
```

Examples:

- claim invite creation
- command creation
- release or revoke workflows
- claim verification confirmation/finalization
- diagnostic requests
- backup triggers or update rollout intent/status actions
- future artifact upload slot creation

v0 idempotency storage is an in-memory API Facade cache with a short TTL. It is not a durable exactly-once guarantee.

Cache scope:

```text
actor
route template
target resource
idempotency key
request hash
```

The cached value stores the prior status code and response body. Immediate retries with the same scope and request hash return the original response. A reused key with a different request hash is rejected.

Known v0 limitations:

- API restarts clear the cache.
- Multiple API instances require sticky routing or accept weaker protection.
- Domain services must still protect safety-critical invariants.

## Route Design

Public route paths are product-resource oriented. Internal service or module names must not leak into public paths unless they are also product concepts.

Use resources for stable product objects:

```text
GET /api/v1/sites/{site_id}
POST /api/v1/sites
PATCH /api/v1/sites/{site_id}
GET /api/v1/devices/{device_id}
```

Use named action endpoints for meaningful business transitions:

```text
POST /api/v1/devices/{device_id}/release
POST /api/v1/sites/{site_id}/device-claim-invites
GET /api/v1/sites/{site_id}/device-claim-invites
GET /api/v1/sites/{site_id}/device-claim-invites/{claim_invite_id}
POST /api/v1/sites/{site_id}/device-claim-invites/{claim_invite_id}/cancel
POST /api/v1/sites/{site_id}/device-claim-invites/{claim_invite_id}/replace
POST /api/v1/device-enrollment/claim-invites/verify
POST /api/v1/device-enrollment/claim-verifications/{claim_verification_id}/confirm
POST /api/v1/commands/{command_id}/cancel
```

If an operation creates a durable command or request object, use a resource collection:

```text
POST /api/v1/devices/{device_id}/commands
GET /api/v1/commands/{command_id}
```

Do not introduce a generic service command bus such as:

```text
POST /api/v1/service/{action}/{resource}
```

## Async Responses

Async actions return:

```text
HTTP 202
durable domain ID
initial status
```

Examples:

```json
{
  "command_id": "cmd_123",
  "status": "queued"
}
```

Use domain-specific status endpoints instead of a generic `/operations/{id}` endpoint in v0.

## Reads And Composition

API Facade may expose composed read models for external client sanity.

Examples:

```text
GET /api/v1/dashboard
GET /api/v1/devices
GET /api/v1/sites/{site_id}/overview
GET /api/v1/devices/{device_id}/summary
GET /api/v1/activity
GET /api/v1/alerts
GET /api/v1/alert-recipients
```

The API Facade owns the external response contract for composed read models. Domain services own the underlying facts.

Default public API responses should be product-shaped views, not storage-shaped records. Raw or internal-ish records may be exposed later only when intentionally promoted to a public API contract with authorization, pagination, filtering, redaction, and compatibility rules.

### V0 Portal Read Models

The v0 portal dashboard, managed Home Assistant list, device fleet view, and
activity timeline are part of the v0 product surface. The browser must not
derive issue counts, issue severity, primary actions, or activity copy by
stitching raw service fragments together. API Facade owns these external read
contracts; domain services own the underlying facts.

`GET /api/v1/dashboard` returns:

- account-scoped summary counts: managed sites, managed devices, online devices,
  sites needing review, open issue count, backup issue count, app update
  attention count, and email-alert configuration status
- a primary dashboard state such as `ok` or `action_required`
- managed Home Assistant rows, using the same row shape as `GET /api/v1/devices`
- recent activity rows, using the same row shape as `GET /api/v1/activity`

`GET /api/v1/devices` returns product-shaped device/site rows:

- `site_id`, `site_name`, optional presentation-only `site_category`, customer
  display name, compact location, and device summary
- presence label and last-seen facts from Presence
- Home Assistant, Supervisor, HomeSignal app, backup, storage, and update
  summary facts from latest-state/backing services
- a computed issue list and primary action

The v0 issue projection shape is:

```json
{
  "issue_code": "device_disconnected",
  "severity": "critical",
  "label": "Disconnected",
  "detail": "Last seen 2 days ago",
  "source_area": "presence",
  "sort_priority": 10,
  "primary_action": "view_device"
}
```

V0 customer-facing issue codes:

| Issue code | Severity | Source | Product behavior |
| --- | --- | --- | --- |
| `device_disconnected` | `critical` | Presence | Show as needing review; eligible for email alert. |
| `backup_failed` | `critical` | Backup Service | Show as needing review; eligible for email alert. |
| `backup_overdue` | `warning` | Backup Service | Show as needing review; eligible for email alert. |
| `app_update_attention` | `warning` | Release/Update plus latest state | Show after a 48 hour grace period when the HomeSignal app has not reached desired/current version; eligible for email alert. |
| `ha_update_advisory` | `info` | Home Assistant version catalog plus latest state | Portal advisory only in v0; not an email alert by default. Hide when latest-version source is unavailable. |
| `storage_warning` | `warning` | Telemetry latest state | Show only when the app reports bounded storage status. |

Issue projection is allowed to be computed at request time for v0. If it becomes
expensive, the same shape may be materialized later without changing the public
contract.

`GET /api/v1/activity` returns a user-visible activity feed:

```json
{
  "activity_id": "act_123",
  "occurred_at": "2026-05-14T15:00:00Z",
  "category": "backup",
  "action": "completed",
  "subject_type": "site",
  "subject_id": "site_123",
  "subject_label": "Smith Residence",
  "detail": "Offsite backup completed",
  "severity": "info",
  "actor_label": "HomeSignal"
}
```

V0 activity categories are `alert`, `backup`, `device`, `update`, `enrollment`,
and `account`. The public activity feed excludes internal platform-health
findings, provider-error noise, sensitive authorization denials, and debug
session details unless a later internal/admin read model intentionally exposes
them. Audit remains the authority trail for sensitive actions; activity is the
user-facing timeline.

## Alert And Email Notification Routes

Customer-facing alert configuration is a public product surface. Route handlers delegate to Alerting Service for alert lifecycle and recipient preference authority.

Minimal v0 route families:

```text
GET /api/v1/alerts
POST /api/v1/alerts/{alert_id}/acknowledge
POST /api/v1/alerts/{alert_id}/snooze

GET /api/v1/alert-recipients
POST /api/v1/alert-recipients
PATCH /api/v1/alert-recipients/{recipient_id}
DELETE /api/v1/alert-recipients/{recipient_id}
```

Alert recipient records are email-recipient scoped. Each recipient may have independent subscriptions for disconnected devices, backup failures, app/update attention, and later alert families, plus optional account/site scope. The API must not expose the transactional email provider as a public concept. Notification Service owns provider delivery through an internal adapter after Alerting creates notification requests. V0 uses Resend behind that adapter, but public API responses must not expose Resend-specific IDs or terminology except in internal/admin delivery diagnostics.

New email recipients start in `pending_verification` unless the address already
belongs to the verified authenticated user creating the recipient. Notification
Service must not deliver product alerts to an unverified recipient. V0 alert
families are `device_disconnected`, `backup_failed_or_overdue`, and
`app_update_attention`; Home Assistant version advisories remain portal-only
unless a later product decision promotes them to email alerts.

## Data Access

API route handlers must not read or write Postgres directly.

Rules:

- writes go through the owning domain/workflow service
- reads go through domain/read services
- repositories/storage adapters are behind domain boundaries
- handlers do not know table shape

## Orchestration

API Facade does not own multi-step business orchestration.

Each mutation route delegates to one owning domain/workflow method. Orchestration lives with the service that owns the state transition.

Examples:

| Workflow | Owner |
| --- | --- |
| Claim device | Enrollment / Device Registry |
| Release device | Enrollment / Device Registry |
| Create command | Command / Egress |
| Archive site | Account / Site |
| Acknowledge alert | Alerting |

If a future workflow has no clear owner, introduce a named application workflow service rather than moving orchestration into route handlers.

## Audit And Operational Logging

Operational request logging is not audit.

The API Facade logs operational request facts:

- request ID
- correlation ID
- actor ID when known
- method
- route template
- status
- latency
- source IP
- user agent
- rate-limit result

Durable audit events are emitted by the domain service that owns the business or security action. API Facade supplies actor, request, source, and auth context.

API Facade may emit durable audit only for API-boundary security events such as suspicious credential use, internal service auth failure, or repeated abuse thresholds.

## Standard API Conventions

- JSON mutation routes require `Content-Type: application/json`.
- Bodyless mutations may omit a body; if a body is present, it must be JSON.
- Timestamps are ISO-8601 UTC strings.
- External IDs are opaque typed strings such as `acct_...`, `site_...`, `dev_...`, `cmd_...`, `upload_...`.
- Public API clients must not use raw database IDs or AWS/provider IDs as primary identifiers.
- Provider IDs may appear only as metadata.
- List endpoints that can grow use cursor pagination:

```json
{
  "items": [],
  "next_cursor": null
}
```

- v0 does not provide generic sparse fields or `include=` expansion.
- `PATCH` uses simple partial JSON bodies, not JSON Patch.
- `DELETE` means actual delete. If the product meaning is archive, release, revoke, cancel, or end access, use an explicit action endpoint.

## Direct App API Boundary

v0 public app-to-cloud API surface is enrollment/bootstrap only:

```text
POST /api/v1/device-enrollment/claim-invites/verify
POST /api/v1/device-enrollment/claim-verifications/{claim_verification_id}/confirm
```

These public app enrollment routes are intentionally POST-only. Claim codes
must appear only in JSON request bodies, never in URL paths, query strings,
redirects, or link fragments. Responses must use `Cache-Control: no-store`.
The API must not enable broad browser CORS for these routes; the local app
backend should call HomeSignal rather than exposing claim codes to arbitrary
browser origins.

Unauthenticated app enrollment errors must avoid invite enumeration. Unknown,
malformed, expired, cancelled, used, disabled, and unauthorized invites all map
to a generic unavailable error class unless the caller is an authenticated
portal user authorized for that site.

Routine claimed-device telemetry/events use the mTLS Agent HTTPS API in v0. AWS IoT Core remains the realtime control/session layer for commands, notifications, shadows, and lifecycle presence. `/agent/*` callbacks are device-runtime flows and are intentionally separate from the public enrollment/bulk API surface above.

Compact desired/reported edge state uses AWS IoT named shadows through `edge-state-adapter.md`, not direct app API routes.

Out of scope for v0 direct app API:

- reported edge state
- command ACKs and results outside approved mTLS Agent HTTPS API callbacks
- topology upload
- generic artifact upload/download brokering
- device status/revocation polling unless explicitly reintroduced by a later app/API spec

Future artifact transfer must follow `artifact-upload-broker.md`. The approved pattern is split by layer: AWS IoT delivers only a tiny command notice, the mTLS Agent HTTPS API handles command detail retrieval, artifact upload negotiation, signed URL issuance, upload completion, and command result reporting, and object storage holds artifact bytes. Signed URLs must not be sent over IoT Core. It must include local app allowlists, destination validation, TTL limits, size limits, redaction rules, metadata records, and upload-failure recursion guards before implementation.

## Agent HTTPS API

Agent routes are for claimed-device flows that require HTTPS negotiation after the device already has durable device credentials.

Namespace:

```text
/agent/*
```

Authentication:

- mTLS is required.
- The HTTPS edge, preferably API Gateway HTTP API custom domain for v0, requests the client certificate during TLS handshake.
- The edge validates the certificate chain against a configured truststore; that truststore is a CA allowlist, not a list of HomeSignal devices.
- API Facade authenticates verified client certificate metadata supplied by the edge.
- Unknown, revoked, expired, or unbound certificates are rejected before domain delegation.
- API Facade derives `device_id`, certificate fingerprint/serial, `site_id`, and `org_id` from trusted certificate metadata and app records.
- Request bodies must not supply authoritative `device_id`, `site_id`, or `org_id`.

Device credential model:

- During claim, the app generates a private key locally and submits a CSR through the HomeSignal claim flow.
- HomeSignal coordinates AWS IoT `CreateCertificateFromCsr`; AWS IoT signs the CSR and returns `certificatePem`, `certificateId`, and `certificateArn`.
- HomeSignal returns the certificate PEM to the app, stores the AWS certificate identifiers and derived certificate metadata, and never receives or stores the private key.
- HomeSignal may pass the certificate PEM through during claim, but does not need to persist the full PEM; the durable authorization record stores fingerprint, serial, issuer, AWS certificate ID/ARN, status, `device_id`, `site_id`, and `org_id`.
- Later `/agent/*` requests are authorized by exact certificate fingerprint/serial lookup against that record.

Initial artifact endpoints:

```text
GET /agent/commands/{command_id}
POST /agent/commands/{command_id}/ack
POST /agent/commands/{command_id}/artifact-upload
POST /agent/artifact-uploads/{upload_id}/complete
POST /agent/commands/{command_id}/result
POST /agent/telemetry
POST /agent/events
```

All Agent HTTPS runtime requests use a common envelope shape: route-specific endpoint plus shared contract metadata and a typed JSON payload. The common envelope mirrors the role MQTT5 message metadata would play in optional/future MQTT ingest, while keeping request bodies self-describing for replay and debugging.

The common envelope fields are:

```text
message_type
schema_type
schema_version
message_id
applied_publish_policy_version, when policy-governed
observed_at, when the device observed the facts
payload
```

Telemetry, events, command ACK, and command result should share this envelope style. Endpoints stay separated for external clarity, rate limits, ownership, and future evolution, but backend handling may use a common runtime-envelope parser.

Payloads are JSON and typed by `message_type`, `schema_type`, and
`schema_version`. `message_type` is the broad runtime lane, such as
`telemetry`, `event`, `command_ack`, or `command_result`. `schema_type` is the
exact contract, such as `device.health_snapshot` or `agent_alarm`. Do not accept
an unbounded generic `details` object as the domain contract. Blob transfer is
separate: artifact bytes use object storage through signed URLs, not JSON
request bodies.

Ownership:

| Route | Delegates To | Required Ownership Check |
| --- | --- | --- |
| `GET /agent/commands/{command_id}` | Command Service | Command belongs to authenticated device. |
| `POST /agent/commands/{command_id}/ack` | Command Service | Command belongs to authenticated device. |
| `POST /agent/commands/{command_id}/artifact-upload` | Artifact Upload Broker / Command Service | Command belongs to authenticated device and permits artifact upload. |
| `POST /agent/artifact-uploads/{upload_id}/complete` | Artifact Upload Broker | Upload belongs to authenticated device and command. |
| `POST /agent/commands/{command_id}/result` | Command Service | Command belongs to authenticated device. |
| `POST /agent/telemetry` | Telemetry Ingest | Telemetry belongs to authenticated device and complies with publish policy. |
| `POST /agent/events` | Telemetry Ingest | Event belongs to authenticated device and complies with event allowlist/publish policy. |

Errors use the standard API error envelope. Certificate failures should use stable codes such as `DEVICE_CERTIFICATE_REJECTED`, `DEVICE_CERTIFICATE_REVOKED`, or `DEVICE_COMMAND_NOT_FOUND_OR_NOT_OWNED` without leaking another device, site, or org.

## Internal Routes

Internal routes are for trusted service and infrastructure callers.

Expected internal callers:

- AWS IoT provisioning adapter callbacks, if any
- future workers
- future service-to-service callbacks
- Telemetry Ingest, Presence, Backup, Command, and Alerting/Notification workers

Artifact-supporting internal interfaces:

- Telemetry Ingest may notify Diagnostics or Artifact Upload Broker that a device reported `more_logs_available`.
- Diagnostics or another owning domain service decides whether an artifact is needed.
- Artifact Upload Broker creates the artifact intent/upload record.
- Command Service delivers a tiny artifact command notice over AWS IoT.
- Agent HTTPS API authenticates the device with mTLS, verifies command ownership, issues the signed upload URL, and records completion/result callbacks.

These are internal and agent-authenticated interfaces, not public generic artifact routes.

Alert-supporting internal interfaces:

- `POST /internal/alert-candidates` lets ingest/presence/backup wake Alerting after persisted state changes.
- Alerting verifies current DB state before creating, updating, resolving, or notifying on a product alert.
- Alerting creates notification requests for eligible recipients after preference and scope evaluation.
- Notification Service sends email through the Resend-backed transactional provider adapter in v0 and records delivery attempt/result metadata.

Internal routes use `/internal/*`, service authentication, standard request IDs, standard correlation IDs, standard error envelopes, and route-class rate limits.

## Out Of Scope v0

- separate public Enrollment, Artifact, Command, or Auth APIs
- physical microservice split
- DB-backed idempotency records
- generic `/operations/{id}` endpoint
- broad admin/support API surface
- generic service command bus
- generic raw table browsing
- generic sparse field/include framework
- public `/api/v1` claimed-device API for routine runtime data outside approved mTLS `/agent/*` routes
- topology/artifact signed URL feature until a product-specific flow is approved under `artifact-upload-broker.md`

## Acceptance Criteria

- Public routes exist in OpenAPI before implementation.
- Public route paths use `/api/v1`.
- Internal routes use `/internal`.
- Route handlers do not directly read or write Postgres.
- Protected routes call Authorization Service instead of doing route-local role checks.
- Mutations delegate to one owning domain/workflow method.
- Async mutations return `202` and a durable domain ID.
- High-risk/retry-prone mutations enforce `Idempotency-Key`.
- API errors use the standard error envelope.
- API responses return `X-Request-ID` and `X-Correlation-ID`.
- API request logs are operational logs, not audit history.
- Public `/api/v1` app API is limited to enrollment/bootstrap in v0; claimed-device runtime HTTPS uses approved mTLS `/agent/*` routes.
