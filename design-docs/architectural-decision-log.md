# Architectural Decision Log

This log is a receipt book, not a second architecture source of truth.

Use it to find where resolved architecture work now lives. Do not restate the architecture here. If a topic is logged as resolved, read the canonical location before asking the user to revisit it.

When changing architecture, update the doc that owns each touched boundary.

Common boundary docs:

- HTTP routes/auth -> `api-facade.md`
- IoT topics/rules/shadows -> `aws-iot-routing-contract.md`
- command ACK/result semantics -> `command-lifecycle.md`
- device identity/trust -> `workstreams/device-lifecycle.md`
- artifact upload negotiation/storage -> `artifact-upload-broker.md`
- service responsibility map -> `service-map.md`

If another boundary is touched, update its owning doc too.

## 2026-05-15 - V1 Draft Architecture Isolation

Status: Resolved

Outcome:
V1 architecture exploration has a separate, clearly labeled draft folder at
top-level `v1-draft-architecture/`, outside the current V0 `design-docs/`
folder. That folder is future-only, non-authoritative, and not deployable for
V0. V0 has not deployed yet, so V1 drafts must not be used to implement, scope,
unblock, or deploy V0 unless the user explicitly asks to promote a V1 decision.
Promotion requires updating the owning V0 boundary doc, reconciling affected
implementation/deployment docs, and recording the decision in this log.

Canonical locations:
- `design-docs/README.md`
- `v1-draft-architecture/README.md`

Touched docs:
- `AGENTS.md`
- `design-docs/README.md`
- `v1-draft-architecture/README.md`
- `v1-draft-architecture/architecture-deltas.md`
- `v1-draft-architecture/open-questions.md`
- `design-docs/architectural-decision-log.md`

## 2026-05-15 - First Deploy Greenlight Protocol

Status: Resolved

Outcome:
The first real deploy path now has an execution protocol. When the user gives a
clear first-deploy greenlight, the agent reads
`design-docs/workstreams/first-deploy-greenlight.md` and proceeds through a
script-first, staging-only deploy path without asking about implementation
minutia. The protocol defaults to a minimal control-plane skeleton with
operational `GET /healthz`, `GET /readyz`, and `GET /version`, proves
`scripts/deploy.sh staging` and `scripts/smoke.sh staging` before adding
CI/CD, and defers Postgres, AWS IoT, mTLS, product object storage, email, and
production until their owning slices. IaC state or build artifact storage may
be created only when the selected staging deploy path requires it.

The protocol also defines stop conditions and a judgment log format for true
external inputs such as AWS account/profile, region override, domain names,
production approval location, Neon, and Resend.

Canonical locations:
- `design-docs/workstreams/first-deploy-greenlight.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/deployment-readiness-matrix.md`

Touched docs:
- `design-docs/workstreams/first-deploy-greenlight.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/cross-cutting-workstreams.md`
- `design-docs/implementation-plan.md`

## 2026-05-15 - V0 AWS Region Selection

Status: Resolved

Outcome:
HomeSignal v0 should use AWS `us-east-1` as its primary region. The selection
matches the verified Neon AWS region from the existing voice-extraction staging
database connection; the active `database-url-staging` Secret Manager value in
that repo resolves to a Neon host suffix of `us-east-1.aws.neon.tech`.

This decision means same AWS region, not same availability zone or exact
physical data center. Availability zone names are account-relative in AWS, and
Neon does not expose a stable cross-account AZ placement target for HomeSignal
to chase. If an operator later rejects `us-east-1` before IaC is created, record
that as an explicit region override.

Canonical locations:
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/first-deploy-greenlight.md`

Touched docs:
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/first-deploy-greenlight.md`
- `design-docs/architectural-decision-log.md`

## 2026-05-15 - Launch Environment And Resource Naming Defaults

Status: Resolved

Outcome:
The first cloud environment is `staging`, not `dev`. Launch cloud environments
are `staging` and `production`; local development uses local config, fake
adapters, and explicit staging fixtures. Any later personal or preview cloud
environment is a temporary sandbox with owner, TTL, cost limit, and cleanup
metadata, not part of the launch environment model.

Durable AWS resource names use lowercase kebab-case with the default shape
`homesignal-{environment}-{boundary}-{purpose}`. Use full environment names in
durable resources, for example `homesignal-staging-control-plane-runtime`,
`homesignal-staging-public-api`, and
`homesignal-staging-platform-iac-state-{account_id}`. Ephemeral staging
resources use `hs-stg-<purpose>-<yyyymmdd>-<shortid>` and carry cleanup
metadata. S3 names must include a globally unique suffix such as the AWS account
ID.

Canonical location:
- `design-docs/workstreams/deployment-readiness-matrix.md`

Touched docs:
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/first-deploy-greenlight.md`
- `design-docs/architectural-decision-log.md`

## 2026-05-15 - Cost Guardrail Before Cloud Deploy

Status: Resolved

Outcome:
HomeSignal should create or verify an account-level AWS Budget or equivalent
cost notification before the first staging cloud deploy when credentials,
permissions, and an operator alert target are available. If the alert target or
permission is missing, first-deploy work may still prepare local code, scripts,
and IaC, but the blocker must be logged and the cloud deploy must stay on the
low-cost skeleton path. Production requires actual and forecasted spend
notifications before first production deploy.

The budget/cost alert target is an operator email address or SNS topic, not a
customer notification channel. Budget thresholds are operator/account setup
values, not product architecture.

Canonical locations:
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/first-deploy-greenlight.md`
- `design-docs/workstreams/operator-prerequisites.md`

Touched docs:
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/first-deploy-greenlight.md`
- `design-docs/workstreams/operator-prerequisites.md`
- `design-docs/architectural-decision-log.md`

## 2026-05-15 - Operator Prerequisite List

Status: Resolved

Outcome:
Operator-owned deploy prerequisites now live in
`design-docs/workstreams/operator-prerequisites.md`. The list separates values
the operator must supply, such as AWS account/profile, budget alert target,
initial thresholds, account security baseline confirmation, optional cost
allocation tag values, hosted zone/domain names, Neon project details, Resend
sender setup, and production approval location, from work Codex should perform
once access exists, such as creating budgets, preparing IaC state, applying
resource naming/tagging policy, validating identity/region/quotas, and creating
the low-cost staging skeleton.

Canonical location:
- `design-docs/workstreams/operator-prerequisites.md`

Touched docs:
- `design-docs/workstreams/operator-prerequisites.md`
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/first-deploy-greenlight.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/cross-cutting-workstreams.md`
- `design-docs/implementation-plan.md`
- `design-docs/architectural-decision-log.md`

## 2026-05-15 - Portal-Created Device Claim Invite Flow

Status: Resolved

Outcome:
Device enrollment moved from add-on-created six-digit pairing codes to
portal-created, site-bound GUID-style claim invites. An authenticated portal
user creates a 72-hour claim invite for a site/customer; HomeSignal returns or
emails the raw code once and stores only a hash/fingerprint. The local add-on
verifies the code first and receives integrator, creator, site, and customer
display details plus a short verification token. The add-on can commit the
claim only after this verification step, using the verification token and
presented details hash. Invite creation, verification, and confirmation are
rate-limited and audited. The public add-on enrollment endpoints are POST-only,
code-in-body routes with `no-store` responses; portal retries are idempotent;
raw codes are unrecoverable after the one-time response/email; replacement
creates a new invite. Invite/verifications have explicit expiry/GC rules and
terminal cleanup for token hashes, code hashes, and unused display PII.

Canonical locations:
- `design-docs/enrollment-claiming-contract.md`
- `design-docs/workstreams/device-lifecycle.md`

Touched docs:
- `design-docs/enrollment-claiming-contract.md`
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/api-facade.md`
- `design-docs/service-map.md`
- `design-docs/implementation-plan.md`
- `design-docs/account-site-service.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/device-broker.md`
- `design-docs/site-skeleton.md`
- `design-docs/ha-plugin-user-journey-report.md`
- `design-docs/home-assistant-add-on-backend-reconciliation.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`
- `design-docs/artifact-upload-broker.md`

## 2026-05-14 - V0 Non-Functional Readiness Matrix

Status: Resolved

Outcome:
V0 non-functional architecture readiness is now captured in a concrete
deployment readiness matrix. The target architecture-readiness grade is `B+` or
better before implementation starts and `A-` or better before production launch.
The matrix closes the prior inventory gaps by naming required resource classes,
environment defaults, runtime target defaults, CI/CD stages and gates,
service-level secret/config classes, smoke checks, test harness gates,
CloudWatch alarm coverage, launch runbooks, and remaining external inputs.
Follow-on reconciliation aligned service/build docs to the matrix: runtime
defaults no longer imply ECS/Fargate, queues, or DLQs as blanket v0 requirements;
product alerting/email is consistently a v0 surface; and missing command ACK
behavior is routed to `command-lifecycle.md`. A later Telemetry Ingest decision
narrows this for the ingest boundary because hot dedupe and batched persistence
are runtime requirements.

The matrix remains architecture-only: it defines what future implementation
must satisfy without implying that CI/CD, IaC, staging, or production resources
already exist.

Canonical locations:
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/deployment.md`

Touched docs:
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/workstreams/test-environments-and-fixtures.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/migration-strategy.md`
- `design-docs/cross-cutting-workstreams.md`
- `design-docs/platform-doctrine.md`
- `design-docs/implementation-plan.md`
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/service-map.md`
- `design-docs/site-skeleton.md`
- `design-docs/device-broker.md`

## 2026-05-14 - Runtime Telemetry Envelope And Health Snapshot Payload

Status: Resolved

Outcome:
The v0 Agent HTTPS runtime envelope now uses product-neutral fields:
`message_type`, `schema_type`, `schema_version`, `message_id`,
`applied_publish_policy_version`, `observed_at`, and `payload`. The older
branded three-part envelope style is retired before implementation.
`message_type` is the broad lane such as `telemetry`, `event`,
`command_ack`, or `command_result`; `schema_type` is the exact versioned
contract such as `device.health_snapshot`.

The v0 hourly health snapshot payload is namespaced by source/ownership:
`agent` for the local reporting runtime, `home_assistant` for observed Home
Assistant Core/Supervisor/backup/storage facts, `addons` as an array of
observed Home Assistant add-on objects, and `runtime_log_summary` for collapsed,
redacted log-derived telemetry context. Raw logs and diagnostic bundles remain
outside routine telemetry and continue to use authorized Diagnostics/Artifact
Broker flows.

Canonical locations:
- `design-docs/api-facade.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/workstreams/observability.md`

Touched docs:
- `design-docs/api-facade.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/workstreams/topic-design.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`
- `design-docs/platform-health-monitoring-service.md`
- `design-docs/service-map.md`
- `design-docs/edge-state-adapter.md`
- `design-docs/implementation-plan.md`
- `design-docs/site-skeleton.md`

## 2026-05-13 - Device Lifecycle, Trust, And Authority

Status: Resolved

Outcome:
Device identity, transport provenance, lifecycle states, claim/release/transfer/revoke behavior, and runtime trust rules were consolidated under the Device Lifecycle workstream.

Canonical location:
- `design-docs/workstreams/device-lifecycle.md`

Touched docs:
- `design-docs/cross-cutting-workstreams.md`
- `design-docs/service-map.md`
- `design-docs/enrollment-claiming-contract.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/workstreams/topic-design.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/platform-doctrine.md`

## 2026-05-13 - Claimed Device ID And AWS IoT Thing Identity

Status: Resolved

Outcome:
Claimed-device runtime identity was clarified: HomeSignal `device_id`, AWS IoT Thing name, and required MQTT client ID are the same durable identifier. AWS IoT Core owns transport authentication, policy enforcement, and transport provenance; HomeSignal ingest consumes IoT Rule-enriched messages keyed by durable `device_id`.

Canonical locations:
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/enrollment-claiming-contract.md`

Touched docs:
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/enrollment-claiming-contract.md`

## 2026-05-13 - Config Wipe, Re-Pairing, And Recognition Signals

Status: Resolved

Outcome:
Config wipe and re-pairing behavior was clarified: wiped add-ons return to unclaimed local behavior; cloud device state is not released merely because local config disappeared; reusing a prior `device_id` requires explicit cloud-authorized repair/reconnect behavior. Hardware/install recognition signals are advisory only.

Canonical locations:
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/enrollment-claiming-contract.md`

Touched docs:
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/enrollment-claiming-contract.md`

## 2026-05-13 - Local Admin Reset And Claim Context Resolution

Status: Resolved

Outcome:
HomeSignal is not irrevocable MDM for a Home Assistant installation. A local Home Assistant administrator may remove the add-on or reset HomeSignal local identity and create a fresh claim with another account/integrator. The web claim flow owns repair/reconnect/fresh-claim choices based on authenticated user, account, site, prior-match context, and authorization; cross-account matches must not expose or mutate prior account records without authority.

Canonical locations:
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/enrollment-claiming-contract.md`

Touched docs:
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/enrollment-claiming-contract.md`

## 2026-05-13 - Pairing Code Claim API Context Step

Status: Resolved

Outcome:
The pairing-code claim API was clarified as a context/finalize flow, not a single code-to-claim mutation. The add-on pairing-session request carries recognition signals including Home Assistant instance UUID and Supervisor machine ID when available; the authenticated web flow evaluates claim context and allowed actions before finalization.

Superseded by the 2026-05-15 portal-created claim invite flow.

Canonical location:
- `design-docs/enrollment-claiming-contract.md`

Touched docs:
- `design-docs/enrollment-claiming-contract.md`

## 2026-05-13 - Artifact Upload Control/Data Split

Status: Resolved

Outcome:
Device artifact upload was revised from "signed URL over IoT Core" to a split architecture. IoT Core remains the realtime command notice layer. The mTLS Agent HTTPS API handles command details, artifact upload negotiation, signed URL issuance, upload completion, and command results. Object storage holds bytes, and the app DB remains the source of truth for devices, commands, uploads, audit, and ownership.

Device HTTPS authentication uses the same AWS IoT-signed device certificate identity. The add-on keeps the private key locally, sends a CSR during claim, receives the signed certificate PEM back through the claim response, and later presents that certificate to `/agent/*` during TLS handshake. The API edge validates the certificate chain against a CA truststore, while HomeSignal authorizes by exact certificate fingerprint/serial stored during claim and derives `device_id -> site_id -> org_id` from app DB records.

Canonical locations:
- `design-docs/artifact-upload-broker.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`

Touched docs:
- `design-docs/artifact-upload-broker.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`
- `design-docs/api-facade.md`
- `design-docs/service-map.md`
- `design-docs/enrollment-claiming-contract.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/command-lifecycle.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`

Boundary docs checked:
- `design-docs/api-facade.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/command-lifecycle.md`
- `design-docs/artifact-upload-broker.md`
- `design-docs/service-map.md`

## 2026-05-13 - V0 Telemetry/Event HTTPS Ingress

Status: Resolved

Outcome:
Routine v0 device-to-cloud telemetry/events use the mTLS Agent HTTPS API, not AWS IoT Basic Ingest. The same AWS IoT-signed device certificate identity used for `/agent/*` artifact flows authenticates telemetry/event requests. The HTTPS edge validates the certificate chain, and HomeSignal authorizes by exact certificate fingerprint/serial before deriving `device_id -> site_id -> org_id`. AWS IoT Core remains the realtime control/session layer for commands, notifications, shadows, lifecycle presence, and optional future/high-volume MQTT telemetry families. Basic Ingest is optional/future and must not become a second authority for the same telemetry family without updating the owning docs.

Canonical locations:
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/api-facade.md`
- `design-docs/aws-iot-routing-contract.md`

Touched docs:
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/api-facade.md`
- `design-docs/service-map.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/command-lifecycle.md`
- `design-docs/artifact-upload-broker.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`
- `design-docs/workstreams/device-lifecycle.md`

## 2026-05-13 - Agent HTTPS Runtime Envelope And Command Results

Status: Resolved

Outcome:
Telemetry, events, command ACK, and command result use a shared Agent HTTPS runtime envelope style with route-specific endpoints and typed JSON payloads. Endpoints remain separate for ownership, authorization, rate limits, and future evolution, but backend parsing may be common. Blob transfer remains separate through object storage and signed URLs. Backup, update, and diagnostics use the same command result wrapper with domain-typed payloads.

Canonical locations:
- `design-docs/api-facade.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/command-lifecycle.md`

Touched docs:
- `design-docs/api-facade.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/command-lifecycle.md`
- `design-docs/service-map.md`

## 2026-05-13 - Claim-As-New Old Record Behavior

Status: Resolved

Outcome:
When a locally reset installation creates a fresh claim, the old cloud record remains protected by its original account authority and should appear disconnected when its old credentials stop reporting. Do not use `stale`, `superseded`, or `conflicted` as fresh-claim lifecycle end states. The old account/site must not be told that the installation was claimed elsewhere. History does not migrate from the old `device_id` to the new `device_id`; same-account repair/reconnect can resume history for the original `device_id`.

Canonical location:
- `design-docs/workstreams/device-lifecycle.md`

Touched docs:
- `design-docs/workstreams/device-lifecycle.md`

## 2026-05-13 - Command No-ACK V0 Behavior

Status: Resolved

Outcome:
Missing command ACK is internal/support-visible only by default in v0. Safe/idempotent commands may retry with bounded backoff until command expiry; non-idempotent or unsafe commands record timeout instead of automatic retry. High-consequence no-ACK creates an internal alert candidate for release/revoke, update, policy refresh, backup trigger, and artifact request. Repeated no-ACK while AWS IoT lifecycle says connected is recorded for platform-health correlation.

Canonical location:
- `design-docs/command-lifecycle.md`

Touched docs:
- `design-docs/command-lifecycle.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`

## 2026-05-13 - V0 Audit Events

Status: Resolved

Outcome:
Mandatory v0 audit events include user invite/membership/role changes, sensitive authorization denials, claim invite creation/email/cancellation/expiry, claim invite verification and confirmation attempts/outcomes, device claim finalization, repair/reconnect, fresh claim, release/revoke, credential disable/rotation, sensitive command issue, artifact request, and update rollout intent/status actions. Device-originated agent alarms remain telemetry/security signals unless they represent or cause an authority change.

Canonical location:
- `design-docs/workstreams/observability.md`

Touched docs:
- `design-docs/workstreams/observability.md`

## 2026-05-13 - Test Fixture And Staging Strategy

Status: Resolved

Outcome:
CI uses simulator contract tests for changed runtime contracts. Staging must include at least one long-lived real Home Assistant/add-on canary paired with staging AWS IoT Core. Ephemeral Home Assistant/add-on instances are used for enrollment, claim, repair/reconnect, and fresh-claim lifecycle tests.

Canonical location:
- `design-docs/workstreams/test-environments-and-fixtures.md`

Touched docs:
- `design-docs/workstreams/test-environments-and-fixtures.md`

## 2026-05-13 - Platform Health V0 Signals

Status: Resolved

Outcome:
Platform Health / Monitoring remains future and internal/support-only in v0. V0 emits and retains summarized signals for stale telemetry, command no-ACK/timeouts, policy drift/apply failure/stale policy, artifact upload failures and suppression counters, over-budget/drop/quarantine counts, Agent HTTPS auth/cert rejects, schema rejection, and identity drift. Product UI may show simple device health from telemetry/latest-state projections, not platform-health findings.

Canonical location:
- `design-docs/platform-health-monitoring-service.md`

Touched docs:
- `design-docs/platform-health-monitoring-service.md`

## 2026-05-13 - V0 Deployment Shape

Status: Resolved

Outcome:
V0 physical deployment was clarified as a separately deployable Telemetry Ingest service plus a control-plane monolith containing API Facade and other logical services. Staging and production are the launch environments, and staging uses real AWS IoT.

Canonical locations:
- `design-docs/service-map.md`
- `design-docs/platform-doctrine.md`
- `design-docs/workstreams/deployment.md`

Touched docs:
- `design-docs/api-facade.md`
- `design-docs/account-site-service.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/telemetry-ingest-build-plan.md`

## 2026-05-13 - Telemetry Ingest And IoT Routing

Status: Resolved

Outcome:
Superseded by `2026-05-13 - V0 Telemetry/Event HTTPS Ingress`. V0 device-to-cloud telemetry/events now use the mTLS Agent HTTPS API. AWS IoT Basic Ingest is optional/future for runtime families that deliberately move to MQTT/rules routing. V0 still does not add an ingest queue.

Canonical locations:
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/workstreams/topic-design.md`

Touched docs:
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/service-map.md`

## 2026-05-13 - State Change, Commands, And Policy Propagation

Status: Resolved

Outcome:
Desired state, commands, fire-and-forget notifications, acknowledged commands, publish policy sync, and per-device publish budgets were separated conceptually and assigned to the state-change/policy workstream.

Effective publish policy freshness is settled for v0: policies carry `policy_version`, `issued_at`, `refresh_after`, and `expires_at`; default freshness is `refresh_after=24h` and `expires_at=7d`; expired, missing, unreadable, or invalid local policy falls back to conservative local behavior.

Edge shadow shape is settled for v0: use one named shadow, `homesignal_edge`, with compact `publish_policy` and `update` sections. Desired state carries version/reference pointers plus tiny resolved config values needed for immediate convergence. Convergence projection remains internal/support-visible in v0. Durable desired fields are not cleared automatically after convergence; they remain the target until superseded.

Canonical location:
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/edge-state-adapter.md`

Touched docs:
- `design-docs/command-lifecycle.md`
- `design-docs/edge-state-adapter.md`
- `design-docs/service-map.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`

## 2026-05-13 - Local Cloud Trust Boundary

Status: Resolved

Outcome:
The cloud may request only modeled, allowlisted local actions over AWS IoT. Local execution policy means add-on-side safety policy, not default per-command human approval.

Canonical location:
- `design-docs/workstreams/local-cloud-trust-boundaries.md`

Touched docs:
- `design-docs/service-map.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/command-lifecycle.md`

## 2026-05-13 - API Facade V0 Scope

Status: Resolved

Outcome:
The API Facade owns the external API surface and auth enforcement, acts as route glue for external sanity, and delegates to internal logical services inside the v0 control-plane monolith.

Canonical location:
- `design-docs/api-facade.md`

Touched docs:
- `design-docs/service-map.md`
- `design-docs/auth-service.md`

## 2026-05-13 - Authorization And RBAC V0 Shape

Status: Resolved

Outcome:
Authorization is centralized. Default roles are seeded and structurally configurable in the backend, but customer-defined roles are not exposed in the v0/v1 UI. Impersonation is not supported in v0.

Canonical location:
- `design-docs/auth-service.md`

Touched docs:
- `design-docs/api-facade.md`
- `design-docs/account-site-service.md`

## 2026-05-13 - Update Rollback, Diagnostics, Artifact Guardrails, Credential Overlap, And Publish Tiers

Status: Resolved

Outcome:
HomeSignal add-on rollback is a normal platform capability, not a hidden support-only action. User/integrator-approved rollback is allowed for supported HomeSignal add-on releases. Automatic rollback is allowed only for failed HomeSignal-controlled add-on update attempts that fail bounded local health/startup/reconnect checks, and it must not mutate broader Home Assistant, Supervisor, OS, database, or arbitrary host state. V0 add-on update execution remains through the local Home Assistant add-on/Supervisor release path; HomeSignal may publish desired version/channel intent and observe/report status.

V0 diagnostics are scoped to bounded HomeSignal add-on/runtime checks and redacted log/error collection. Broad Home Assistant configuration snapshots are not v0 diagnostics.

Artifact upload defaults now include bounded TTL, purpose-specific default size limits, content expectations, server-generated object keys, completion validation, and redaction expectations.

Device credential replacement uses a primary/secondary credential slot model. A short overlap window is allowed during explicit repair or rotation, defaulting to a few hours and never exceeding 24 hours without an operational exception. Replacing credentials does not change `device_id`.

Publish policy starts with a free/default tier for all devices while supporting admin-defined tiers from the beginning. Tiers resolve server-side into per-device policy records; devices receive resolved policy, not tier labels.

Canonical locations:
- `design-docs/update-architecture.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/artifact-upload-broker.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/workstreams/migration-strategy.md`

Touched docs:
- `design-docs/update-architecture.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/artifact-upload-broker.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/workstreams/migration-strategy.md`

## 2026-05-13 - V0 Observability And Temporary Debug Mode

Status: Resolved

Outcome:
V0 observability uses a value-engineered posture: coarse CloudWatch metrics and alarms, short-retention structured logs, App DB records for command/audit/status/product truth, S3 for cold diagnostic/support artifacts, and no default per-message tracing or high-cardinality metrics.

Temporary debug mode is the targeted escape hatch. It is cloud-initiated by an authorized internal/support actor, not customer self-service. Diagnostics owns debug-session product behavior; API Facade owns route shape; Authorization Service owns permission checks; Command Service owns device command lifecycle; Artifact Upload Broker owns debug bundle transfer; the add-on enforces TTL, redaction, local rate/size limits, and safe capture categories.

Routine device-originated logging and health verbosity belongs inside publish
policy. Publish policy carries the standing device observability policy because
it already has versioning, expiry, local enforcement, and cloud ingest
backstops. Standing levels are `quiet`, `normal`, and `verbose`; rich `debug`
capture remains a temporary debug session, not a durable policy level.

Runtime message "pay attention" flags are ingress-side internal annotations.
Telemetry Ingest derives them from authenticated identity, publish policy,
debug sessions, watch rules, and ingest classification. Devices do not author
these flags, and downstream services should not reinterpret raw payloads to
decide log level or retention.

Canonical locations:
- `design-docs/workstreams/observability.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`

Touched docs:
- `design-docs/workstreams/observability.md`
- `design-docs/command-lifecycle.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`
- `design-docs/artifact-upload-broker.md`
- `design-docs/api-facade.md`
- `design-docs/platform-health-monitoring-service.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/telemetry-ingest-architecture.md`

## 2026-05-13 - Historical Telemetry Storage And Extractable Internal Contracts

Status: Resolved

Outcome:
HomeSignal should capture historical telemetry for future product evolution,
support, replay, and analysis, but v0 should not treat Postgres as an unbounded
time-series store. Postgres owns latest state, sparse material history,
support/debug references, and product UI queries that need ordinary database
latency. Cold historical telemetry should be written or exported to object
storage in a device-rooted partition layout so per-device deletion/export is
straightforward. V0 keeps latest state in Postgres while the device exists and
keeps sparse historical telemetry rows hot in Postgres for 7 days. After that,
an archive worker writes verified daily plain NDJSON records plus optional
`summary.json` under the device root, then prunes archived DB history beyond the
hot window. Compression, compaction, and colder storage classes are deferred
cost optimizations. Daily summaries are for support and future LLM consumption;
they are deterministic summaries, not product authority. A dedicated
time-series/analytics store is deferred until product requirements need dense
history, fleet analytics, or long-window query performance.

Logical services inside the v0 control-plane monolith should keep
adapter-friendly service contracts. Internal calls may be in-process, but they
should use explicit request/context and response/error shapes so future physical
service extraction can replace the adapter with HTTP/internal RPC without
rewriting domain boundaries. AWS-hosted out-of-process service calls should use
AWS IAM role identity and SigV4-signed requests by default. HomeSignal maps the
verified AWS principal to an app-level service subject and still performs
app-level authorization for sensitive actions. Deployment tooling, migration
tooling, and VPC/private networking remain separate architecture topics.

Canonical locations:
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/workstreams/identity-and-authorization.md`
- `design-docs/service-map.md`

Touched docs:
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/workstreams/identity-and-authorization.md`
- `design-docs/service-map.md`

## 2026-05-13 - V0 Database Provider Posture

Status: Resolved

Outcome:
Neon Postgres is acceptable for v0 when it materially helps cost and development
velocity. RDS is not required solely for AWS purity or private networking
neatness. If Neon is used, prefer the same AWS region as HomeSignal services
when practical, treat PostgreSQL as the application contract, keep database
access behind repositories/storage adapters, use provider-neutral secret/config
injection, and avoid Neon-specific application semantics. A future RDS migration
should remain a provider migration with a rehearsable dump/restore or replication
cutover path, not a domain architecture rewrite.

Canonical locations:
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/migration-strategy.md`
- `design-docs/service-map.md`

Touched docs:
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/migration-strategy.md`
- `design-docs/service-map.md`

## 2026-05-13 - Network Boundary And Scriptable Operations

Status: Resolved

Outcome:
`/internal/*` routes must not be internet-facing. Public edge routes are limited
to product/client APIs and authenticated agent/device APIs. When internal
services are physically split, internal routes are reachable only through
trusted AWS/service networking or equivalent private integration, plus IAM/SigV4
service authentication.

Routine developer and Codex operations must be scriptable through AWS CLI and
repo scripts. Deployment, health checks, log tailing, smoke checks, and
controlled private access must not depend on console-only or VPN-only workflows.

HomeSignal should keep a minimal VPC skeleton for future private resources and
stateful services, but should not force every v0 service into private networking
before there is a concrete need. Avoid NAT Gateway/private-egress complexity
until a specific service requirement justifies the fixed cost and operational
surface.

Canonical locations:
- `design-docs/workstreams/deployment.md`
- `design-docs/api-facade.md`

Touched docs:
- `design-docs/workstreams/deployment.md`
- `design-docs/api-facade.md`

## 2026-05-13 - Script-First CI/CD And CodeBuild Runner

Status: Resolved

Outcome:
HomeSignal CI/CD is script-first. Repo-owned scripts are the real build, test,
migrate, deploy, and smoke-test interface so Codex, local development, and CI
systems can all drive the same operations. AWS CodeBuild is the preferred runner
for AWS-heavy build, test, and deploy work because it can run with AWS IAM roles
and avoids making GitHub-hosted runner usage the center of gravity. GitHub
Actions may provide lightweight repo feedback or trigger CodeBuild, but should
not own AWS deployment behavior. AWS CodePipeline remains optional and should
only be introduced when promotion, approvals, or multi-stage orchestration
justify it.

Canonical location:
- `design-docs/workstreams/deployment.md`

Touched docs:
- `design-docs/workstreams/deployment.md`

## 2026-05-14 - IaC, Schema Rollback, Debug Retention, And Device Telemetry Deletion Defaults

Status: Resolved

Outcome:
V0 infrastructure as code should use an OpenTofu/Terraform-style workflow, with
OpenTofu preferred unless provider/tooling friction makes Terraform materially
simpler. Schema migration policy is additive-first and app-rollback-compatible:
backfills, verification, and destructive cleanup are separate deploy steps, and
rollback is normally application rollback or forward-fix rather than database
undo. Temporary debug mode defaults to a 1-hour TTL, 24-hour hard maximum,
14-day debug artifact retention, and 90-day metadata/support summary retention.
Product telemetry history remains hot in Postgres for 7 days, archives to
device-rooted plain NDJSON object storage, and is deleted after a 7-day
operational grace period when the device is deleted; audit/security/authority
records are retained separately with minimized payloads.

Canonical locations:
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/migration-strategy.md`
- `design-docs/workstreams/observability.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/workstreams/device-lifecycle.md`

Touched docs:
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/migration-strategy.md`
- `design-docs/workstreams/observability.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/workstreams/device-lifecycle.md`

## 2026-05-14 - V0 Policy Defaults, Release Approval, And Protocol Window

Status: Resolved

Outcome:
V0 publish-policy and debug/observability values are seeded, auditable
policy/config records, not hidden business constants. The free/default
publish-policy seed uses hourly telemetry and health cadence, backup summary in
the hourly snapshot, 25 `ha_event` events/hour when enabled, 10
`agent_alarm` events/hour with a 3/minute burst, 10 routine runtime log
events/hour, 5 KB diagnostic excerpts, cloud-authorized artifact uploads only,
24-hour policy refresh, and 7-day policy expiry. These defaults are deliberately
coarse and may become finer by plan, site, support relationship, event family,
or temporary support override without changing the add-on contract.

Staging deploys may be automatic/script-driven after tests pass. Production
deploys require explicit operator approval after staging smoke checks pass;
production migrations require precheck and postcheck; production deploy records
must state rollback or forward-fix expectations.

The v0 add-on compatibility window is current protocol family plus one prior
compatible protocol family, with 30 days normal deprecation notice when
practical and immediate cutoff allowed for security or severe abuse. Unsupported
add-on versions should be visible in the UI.

Canonical locations:
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/migration-strategy.md`

Touched docs:
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/migration-strategy.md`

## 2026-05-14 - Diagnostics, Backup, Observability Backend, Secrets, Migration Tool, And Non-Productized Futures

Status: Resolved

Outcome:
Bounded v0 HomeSignal add-on diagnostics and temporary debug capture do not
require local user approval. They require cloud authorization, audit, TTL,
redaction, size/rate limits, and add-on allowlists. Future broad Home Assistant
or host diagnostics require a separate local policy/approval model.
Diagnostic/debug/error-log artifact uploads are not a generic v0 upload surface.
They are permitted only when an owning Diagnostics/Debug flow explicitly creates
an allowlisted command/request with TTL, redaction, size limits, and audit.

Backup Service is the cloud owner of backup meaning: backup policy/status
records, trigger attempts, command/result interpretation, last success/failure,
overdue/in-progress state, backup summary metadata, retention, visibility, and
product interpretation. The add-on performs or reports local backup facts. V0
may store offsite Home Assistant backup bytes in object storage; Artifact Upload
Broker owns upload capability and object metadata for that transfer.

Stale local policy, policy drift, pending convergence, and publish-policy
no-ACK are internal/support-visible by default in v0 and are not productized or
directly exposed to customers. The platform still retains enough state for
support/internal diagnosis and future product work.

Live event streams are not a v0 product requirement. The policy model keeps
event-family gates and budgets so live events can be productized later without
reworking the add-on/cloud contract, but v0 does not build pricing or UX around
live event streams.

Claim-as-new history behavior remains settled: history stays with the
account/site authority that owned the old device record when the history was
produced. A different account/integrator does not receive old history by
claiming the local installation as new. Future explicit history transfer/copy
may be designed later, but it is not a v0 product feature.

V0 observability uses CloudWatch Metrics and CloudWatch Logs by default, no
default distributed tracing, 7-day staging logs, 14-day production logs, and
MVP alarms for service health/readiness, elevated errors, ingest
rejects/quarantine spikes, Agent HTTPS auth/cert failures, command failures,
artifact failures, and database/dependency failures.

Secrets/config use standard `staging` and `production` environment names,
`/homesignal/{environment}/{service}/{secret_name}` secret paths,
`/homesignal/{environment}/{service}/config/{config_name}` non-secret
parameter paths, and `HOMESIGNAL_*` environment variables. Broad automatic
service credential rotation is not v0, but Neon/Postgres credential rotation is
a day-zero requirement with a tested staging runbook/script. HomeSignal does
not own local device private-key backup; lost local key material is handled
through repair/reconnect/credential replacement.

V0 database migrations use Goose-style SQL migrations through repo scripts.

Canonical locations:
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/service-map.md`
- `design-docs/artifact-upload-broker.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/workstreams/migration-strategy.md`

Touched docs:
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/service-map.md`
- `design-docs/artifact-upload-broker.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/workstreams/migration-strategy.md`

## 2026-05-14 - Platform Health Scaffold, Dormant Support Capabilities, And Degraded UX Posture

Status: Resolved

Outcome:
Platform Health / Monitoring should be scaffolded through source facts first,
not by building a v0 rule engine. V0 services should emit/store bounded facts
that a later slow evaluator can consume: ingest reject/drop/quarantine counts,
command ACK/result failures, artifact failures, Agent HTTPS auth rejects,
Edge State convergence projections, coarse metrics, and short-retention logs.
The future service creates internal findings and recommends routed remediation;
it does not sit on the hot ingest path or directly mutate product authority.

The add-on may include dormant, deny-by-default support-action handlers for
future optionality. Shipping a dormant handler does not enable it. Cloud policy,
Authorization Service, command allowlists, and local add-on policy must all
allow execution. Future broad Home Assistant or host-affecting support actions
still require a product/security decision before activation.

Detailed degraded/convergence UX is not productized in v0. Store enough support
state for future UI design, but do not expose raw policy drift, command no-ACK,
convergence windows, or Platform Health findings to customers. Near-term UI may
show simple device/add-on health derived from latest telemetry and presence.

Canonical locations:
- `design-docs/platform-health-monitoring-service.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/workstreams/observability.md`

Touched docs:
- `design-docs/platform-health-monitoring-service.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/workstreams/observability.md`

## 2026-05-14 - Workstream Open-Question Cleanup Defaults

Status: Resolved

Outcome:
Stale workstream "Open Questions" headings were converted to closed v0
defaults where the architecture already had enough authority. Authorization
action names use `resource:action` with lower_snake_case actions. Split-service
networking follows the existing deployment posture: IAM/SigV4 service identity,
internal routes not internet-facing when split, minimal VPC skeleton, and no
forced private networking for every v0 service. Test fixtures use service-local
`testdata/fixtures` by default, live/staging tests are explicitly marked, and
ephemeral staging resources use staging-scoped names plus cleanup metadata.
Add-on local state migrations are versioned, idempotent, atomically written,
and fail to degraded/unclaimed safe mode when local state is unsafe.
The older `device-broker.md` requirements sketch was given an explicit
canonical-doc precedence note and aligned with current v0 transport,
command-lifecycle, credential, drift/UX, and alert posture.
Residual MQTT-specific wording in runtime error/policy/service-map text was
broadened to runtime event/command-result language so v0 Agent HTTPS telemetry
does not conflict with optional/future MQTT ingest.
Backup artifact wording was aligned so v0 offsite Home Assistant backup bytes
are consistently treated as Backup Service-owned product data transferred
through the Artifact Upload Broker, not as a generic future artifact feature.
Alerting/notification wording in the service map was clarified as future
product/customer-facing lifecycle unless a product rule promotes an alert
candidate; platform-health findings and most v0 runtime candidates remain
internal/support-visible by default.
Update wording was aligned with the update architecture: v0 HomeSignal add-on
updates are desired version/channel intent plus status observation through the
Home Assistant Supervisor/add-on release path. HomeSignal does not initiate
binary installation over IoT Core in v0. Stage/apply update commands are
future/local-supervisor scope only unless a later update spec explicitly owns
that execution surface.
Older requirements-sketch terminology was normalized: fresh claim is distinct
from transfer, fresh claim does not migrate history, topology product state is
not assigned to a vague Device State/Twin owner, and command examples refer to
update status/repair instead of broad update execution.
The desired HomeSignal add-on version is explicitly modeled as
`homesignal_edge.update` shadow desired state. CI/CD publishes the add-on
release through the normal GitHub/Home Assistant add-on release channel first;
Release / Update Orchestrator later promotes a compact desired version/channel
for a cohort through the Edge State Adapter. The shadow is rollout intent and
convergence tracking, not artifact delivery or forced installation.
Command lifecycle wording was narrowed so v0 update-related commands mean
bounded status/check/repair only. Downloading, installing, and applying updates
remain future local-supervisor command scope unless a later update spec
explicitly owns that execution.

Canonical locations:
- `design-docs/workstreams/identity-and-authorization.md`
- `design-docs/workstreams/test-environments-and-fixtures.md`
- `design-docs/workstreams/migration-strategy.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/edge-state-adapter.md`
- `design-docs/device-broker.md`
- `design-docs/service-map.md`
- `design-docs/update-architecture.md`
- `design-docs/aws-iot-routing-contract.md`
- `design-docs/command-lifecycle.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/service-map.md`
- `design-docs/command-lifecycle.md`
- `design-docs/workstreams/local-cloud-trust-boundaries.md`
- `design-docs/update-architecture.md`
- `design-docs/update-architecture.md`
- `design-docs/aws-iot-routing-contract.md`

Touched docs:
- `design-docs/workstreams/identity-and-authorization.md`
- `design-docs/workstreams/test-environments-and-fixtures.md`
- `design-docs/workstreams/migration-strategy.md`
- `design-docs/workstreams/observability.md`
- `design-docs/workstreams/deployment.md`
- `design-docs/workstreams/secrets-and-config.md`
- `design-docs/workstreams/device-lifecycle.md`
- `design-docs/edge-state-adapter.md`
- `design-docs/device-broker.md`
- `design-docs/service-map.md`
- `design-docs/add-on-runtime-error-and-artifact-contract.md`
- `design-docs/workstreams/state-change-and-policy-propagation.md`
- `design-docs/telemetry-ingest-build-plan.md`
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/service-map.md`

## 2026-05-14 - Customer Email Alerting And Notification Delivery

Status: Resolved

Outcome:
Customer-facing email alerts are part of the platform architecture as a v0
product surface. Telemetry, presence, backup, command, and other
domain services may emit alert candidates or product state changes, but
Alerting Service owns product alert lifecycle and recipient preference/scope
evaluation. Notification Service owns transactional message rendering,
outbox/attempt processing, delivery attempt/result metadata,
suppression/cooldown, and provider-error handling. Email delivery uses a
provider adapter; Resend is the default v0 connector, chosen to match the
existing voice-extraction API pattern. The provider remains replaceable and must
not become public API shape.

Canonical locations:
- `design-docs/service-map.md`
- `design-docs/api-facade.md`
- `design-docs/auth-service.md`

Touched docs:
- `design-docs/service-map.md`
- `design-docs/api-facade.md`
- `design-docs/auth-service.md`

## 2026-05-14 - V0 Portal Read Models And Dashboard Wiring

Status: Resolved

Outcome:
The Dashboard, Devices fleet view, Activity timeline, and email-alert settings
shown in the portal mock are v0 product surface. API Facade owns the public
read-model contracts for dashboard summary, device-list rows, issue projection,
and activity feed. Domain services continue to own the underlying facts.

Dashboard and device rows use one server-side issue projection so Dashboard,
Devices, Alerts, and future detail pages do not calculate different answers from
the same state. V0 customer-facing issue codes are `device_disconnected`,
`backup_failed`, `backup_overdue`, `addon_update_attention`,
`ha_update_advisory`, and `storage_warning`. `addon_update_attention` uses a
48-hour grace period before surfacing. `ha_update_advisory` is portal-only in v0
and is hidden when the latest-version source is unavailable.

The public Activity feed is a product timeline adapted from audit, lifecycle,
backup, update, alert, enrollment, account, and selected telemetry state-change
facts. It excludes internal platform-health findings, provider-error noise,
sensitive authorization denials, and debug-session details unless a later
internal/admin read model intentionally exposes them.

Email alerts are v0. Alert recipients are email-recipient scoped, have
independent alert-family subscriptions, may have optional site scope, and require
verification before Notification Service sends product alerts unless the address
is the authenticated user's already-verified email.

Site icon variation is presentation-only. Account/Site may expose optional
`site_category` values such as `residential`, `business`, and `other`, but the
UI falls back to the default Home Assistant/site icon when absent. The category
must not drive authorization, billing, lifecycle, or device-placement behavior.

Canonical locations:
- `design-docs/api-facade.md`
- `design-docs/account-site-service.md`
- `design-docs/service-map.md`
- `design-docs/update-architecture.md`
- `design-docs/ui-data-wiring-reconciliation.md`
- `design-docs/implementation-plan.md`

Touched docs:
- `design-docs/api-facade.md`
- `design-docs/account-site-service.md`
- `design-docs/service-map.md`
- `design-docs/update-architecture.md`
- `design-docs/ui-data-wiring-reconciliation.md`
- `design-docs/implementation-plan.md`

## 2026-05-16 - Telemetry Ingest Write Suppression Runtime

Status: Resolved

Outcome:
V0 Telemetry Ingest is not a per-message database writer. It must use schema-aware
material hashes, hot dedupe/coalescing state, publish-policy budgets, and batched
persistence so unchanged telemetry does not become one Postgres write per
accepted report. Postgres stores current product state, sparse material history,
support/debug references, failure/quarantine records, and cold-archive pointers,
not an unbounded raw telemetry stream.

The default cloud runtime for Telemetry Ingest is a small long-lived service,
such as ECS/Fargate, because the runtime needs effective hot state and batched
DB writes. Lambda remains acceptable for the first control-plane deployment
proof and for future ingest adapters only if a shared dedupe/batching layer
preserves the same write-suppression behavior. Per-message Lambda
direct-to-Postgres for Telemetry Ingest requires an explicit architecture
exception before implementation.

Canonical locations:
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/service-map.md`
- `design-docs/implementation-plan.md`

Touched docs:
- `design-docs/telemetry-ingest-architecture.md`
- `design-docs/workstreams/deployment-readiness-matrix.md`
- `design-docs/service-map.md`
- `design-docs/implementation-plan.md`
