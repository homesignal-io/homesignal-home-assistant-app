# V1 Draft Prioritized Exploration Backlog

Status: Future draft, non-authoritative for V0.

This is a first priority cut for V1 architecture and product exploration. It is
not V0 scope, not a deployment blocker, and not an implementation plan.

## Priority Model

Use these labels until V1 scope is promoted:

- `P0` means investigate before committing to V1 scope.
- `P1` means likely V1 candidate after the P0 reliability/security base is
  understood.
- `P2` means keep warm, prototype only if it clarifies a P0/P1 decision.
- `Defer` means explicitly avoid for V1 unless the product thesis changes.

## P0: Testing And Reliability First

Testing should be a first-class V1 product capability, not just CI plumbing.
HomeSignal is a high-reliability platform touching customer homes, device
identity, backups, alerts, and support workflows. V1 should prove the platform
can fail predictably before adding broad new behavior.

Priorities:

- Build a production-shaped virtual environment harness: Home Assistant,
  Supervisor/add-on, fake or staged cloud APIs, fake AWS IoT where possible,
  fake email provider, fake object storage, and deterministic clocks.
- Add multi-environment test lanes: local fake, local virtual HA, ephemeral
  integration environment, long-lived staging canary, and production smoke.
- Create end-to-end scenario tests for claim, certificate issuance, mTLS Agent
  HTTPS, telemetry, command ACK/result, shadow convergence, backup trigger,
  artifact upload, alert creation, and notification delivery.
- Build device/add-on version matrix tests across supported Home Assistant,
  Supervisor, add-on, protocol, and cloud API versions.
- Add failure-injection tests for network loss, IoT disconnects, expired or
  revoked certs, database outage, object storage failure, email provider
  failure, rate limits, duplicate messages, stale desired state, and clock skew.
- Add migration and rollback tests: schema precheck/postcheck, additive
  migration compatibility, app rollback compatibility, and restore rehearsal.
- Add load, soak, and cost-shape tests for telemetry cadence, command fanout,
  alert storms, debug artifact bursts, and reconnect storms after outage.
- Add contract tests with replayable fixtures for every external boundary:
  public API, Agent HTTPS, MQTT topics, shadow documents, notification provider,
  artifact broker, and audit events.
- Add production smoke tests that are non-destructive, non-customer-data, and
  safe to run after every deploy.

Exploration questions:

- What is the smallest virtual HA environment that catches real add-on failures
  without becoming too slow for daily use?
- Where should simulator devices stop and real HA canaries begin?
- Which tests must run on every PR, every staging deploy, nightly, and before
  production?
- What pass/fail signal is strong enough to block V1 production promotion?

## P0: Security, Trust, And Privacy

Security should be explored before V1 feature expansion because later product
surface will reuse the same authority model.

Priorities:

- Threat-model enrollment, claim invites, certificate issuance, Agent HTTPS,
  command execution, backup/artifact transfer, support access, notification
  delivery, and customer-facing sharing.
- Prove tenant isolation with authorization matrix tests, cross-account
  fixtures, and negative tests for every account/site/device route family.
- Design certificate lifecycle hardening: rotation, overlap windows,
  revocation, repair/reconnect, compromised device recovery, and expired
  credential behavior.
- Define support access controls: scoped grants, reason codes, TTL, approval
  flow where needed, immutable audit, and visible support activity history.
- Expand secrets and supply-chain posture: automatic credential rotation where
  justified, signed build artifacts, dependency/SBOM scanning, provenance, and
  release integrity checks.
- Define privacy and retention posture for V1 telemetry, diagnostics,
  customer records, support notes, backup metadata, and deleted devices.
- Add security regression tests for redaction, secret leakage, raw claim code
  handling, signed URL exposure, cert material, logs, and audit completeness.
- Define abuse controls for claim verification, telemetry storms, alert spam,
  artifact upload loops, and support/debug endpoints.

Exploration questions:

- Which support actions require explicit customer approval versus integrator or
  operator approval?
- Should audit events gain tamper-evident export or append-only storage in V1?
- What data must be customer-visible for trust without exposing sensitive
  operational internals?

## P0: Maintenance And Operations

V1 should reduce operator load. If V0 proves the product, V1 should make it
boring to run.

Priorities:

- Turn Platform Health / Monitoring from stored source facts into an internal
  evaluator with findings, suppression, cooldown, severity, ownership, and
  routed remediation recommendations.
- Build operator and support dashboards for service health, canary state,
  fleet risk, failed commands, ingest rejects, auth failures, artifact failures,
  alert delivery failures, and cost anomalies.
- Add runbook-backed maintenance actions: rotate credentials, repair device
  credentials, replay a failed notification, reissue desired state, inspect
  command lifecycle, quarantine noisy devices, and clean expired artifacts.
- Define SLOs and error budgets for enrollment, Agent HTTPS ingest, command
  delivery, backup status freshness, notification delivery, and portal reads.
- Build incident tooling: correlation ID search, deploy timeline overlays,
  canary comparison, audit lookup, and safe log/artifact retrieval.
- Create lifecycle jobs for retention, archives, expired debug sessions,
  orphaned artifacts, stale claim invites, inactive devices, and old canary
  fixtures.
- Improve compatibility management: protocol-family support windows,
  unsupported add-on visibility, upgrade pressure, staged deprecation, and
  customer-safe cutoff rules.

Exploration questions:

- What incidents should be auto-detected versus only dashboard-visible?
- Which remediation actions are safe to automate, and which must stay
  human-approved?
- What operational data should be visible to integrators versus internal
  operators only?

## P1: Deployment And Multiple Environments

V1 should support more than staging and production once the launch path is
stable. The goal is faster testing without creating uncontrolled cloud sprawl.

Priorities:

- Add controlled preview or virtual environments with owner, TTL, cost limit,
  cleanup metadata, isolated secrets, and seeded non-customer data.
- Support disposable integration environments for PR-level or branch-level
  tests when the change touches cloud contracts.
- Split environment classes clearly: local fake, local virtual, ephemeral
  cloud, staging, production, and long-lived canary.
- Make test device identity safe across environments so a device cannot be
  accidentally claimed or trusted by the wrong environment.
- Add automated cleanup and budget guardrails for all ephemeral environments.
- Define environment promotion evidence: test results, smoke record, migration
  check, canary health, cost check, and rollback or forward-fix note.

Exploration questions:

- Do preview environments need real AWS IoT, or can most contract testing stay
  in a simulator?
- Which V1 features require separate AWS accounts versus environment-scoped
  resources?
- What is the cost ceiling for daily ephemeral environment use?

## P1: User And Product Features

These are likely V1 candidates after the testing, security, and maintenance base
is understood.

Priorities:

- Customer portal access: customer login, site status, alert preferences,
  trusted support visibility, and safe read-only history.
- Scoped sharing: external recipients or viewers for status, alerts, or limited
  site visibility without turning site relationships into broad social sharing.
- Better integrator fleet management: fleet risk view, issue triage, customer
  communication state, maintenance windows, and service notes.
- Richer alerting: more alert families, escalation rules, quiet hours,
  recipient verification UX, suppression, digest mode, and alert history.
- Backup product expansion: offsite backup policy, restore-readiness checks,
  backup verification, retention visibility, and customer-facing backup health.
- Update experience expansion: rollout rings, customer/integrator visibility,
  maintenance windows, failed-update remediation, and compatibility warnings.
- Topology and inventory: safe Home Assistant topology summaries, device/add-on
  inventory, integration inventory, and health context without collecting raw
  private home data.
- Diagnostics and debug UX: customer-visible support sessions, scoped
  approvals where needed, artifact redaction previews, and support history.
- Live or near-live event streams: opt-in event families, budgets, pricing
  implications, and UI value before committing to high-volume ingestion.
- Reporting: account/site health reports, backup compliance reports, device
  reliability trends, and support handoff exports.

Exploration questions:

- Which V1 feature most improves trust for customers versus efficiency for
  integrators?
- Should customer login be read-only first, or should it include alert and
  support approvals?
- Which data makes the product feel useful without making HomeSignal a general
  Home Assistant remote-control platform?

## P1: Data, Analytics, And Platform Evolution

These topics shape future reliability and product quality, but they should not
drive V1 scope until the testing and security tracks have enough evidence.

Priorities:

- Decide whether Postgres plus object archive remains enough for V1 telemetry
  trends, or whether a time-series/analytics store is justified.
- Explore a queue or event bus for workloads that need durability,
  backpressure, fanout, or replay, without putting unnecessary complexity on the
  hot telemetry path.
- Design event replay for alert candidates, read-model rebuilds, support
  investigations, and incident reconstruction.
- Define analytics boundaries for product usage, fleet reliability, alert
  quality, and cost attribution while minimizing customer data exposure.
- Explore optional MQTT/Basic Ingest for runtime families only if scale,
  latency, or cost makes Agent HTTPS insufficient.

Exploration questions:

- What V1 data volume would force a storage or ingestion architecture change?
- Which events need replay semantics versus simple state projection?
- What customer-visible analytics are valuable enough to retain longer?

## P2: Local Supervisor And Advanced Device Control

This is strategically important but potentially risky. Treat it as an
exploration unless the V1 thesis depends on stronger local automation.

Priorities:

- Explore a future local supervisor for safer local convergence, backup checks,
  health checks, staged local operations, and rollback support.
- Define hard safety boundaries for local actions: no arbitrary shell, no broad
  Home Assistant service execution, explicit allowlists, local policy, cloud
  authorization, audit, and customer approval where needed.
- Prototype only bounded actions with fake or virtual HA harness coverage.

Exploration questions:

- Which local operations create enough customer value to justify the safety
  surface?
- Can update/backup remediation remain Home Assistant-native instead of
  HomeSignal-controlled?
- What local failure modes must never be remotely triggered?

## Defer Unless Strategy Changes

Avoid making these V1 by accident:

- Generic remote shell or arbitrary Home Assistant service calls.
- Broad remote access tunnel product.
- Full customer-defined role editor.
- Unrestricted artifact upload/download.
- High-volume live event product without pricing, budgets, and privacy model.
- Device twin replacement for AWS IoT named shadows unless AWS IoT no longer
  fits the product.
- Broad multi-region active-active architecture before there is a clear
  reliability or compliance reason.

## Suggested V1 Discovery Order

1. Testing/reliability harness and environment model.
2. Security threat model and tenant-isolation proof.
3. Maintenance/operations and Platform Health evaluator shape.
4. User feature ranking: customer portal, scoped sharing, fleet management,
   backup/update expansion, diagnostics UX, and reporting.
5. Data/platform decisions: queue, analytics store, replay, optional MQTT
   ingest, and retention.
6. Local supervisor exploration only after the safety model is explicit.
