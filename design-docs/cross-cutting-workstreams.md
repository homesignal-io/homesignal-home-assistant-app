# Cross-Cutting Workstreams

This document is the routing index for work that cuts across multiple HomeSignal services, implementation disciplines, or trust boundaries.

Cross-cutting docs do not replace service architecture. They define shared invariants that local service plans must satisfy.

## Agent Use Protocol

Before implementing or revising a service plan:

1. Read `platform-doctrine.md`.
2. Identify every relevant workstream in the table below.
3. Read the local service architecture doc and any existing build plan. For the
   current cross-platform build sequence, read `implementation-plan.md`.
4. Reconcile the local plan against the cross-cutting checks.
5. Record unresolved conflicts as decisions, not hidden assumptions.

Do not implement from a service-local plan alone when the work touches identity, authorization, device authority, topics, secrets, deployment, observability, platform health monitoring, local/cloud trust, cloud-to-local policy propagation, fixtures, or durable state migrations.

When work adds or changes public or internal HTTP routes, read `api-facade.md` before implementation. The API Facade spec owns route shape, OpenAPI expectations, request context, idempotency, rate limiting, standard errors, and delegation rules.

When work adds or changes accounts, customer records, sites, buildings, zones, site relationships, or site lifecycle, read `account-site-service.md` before implementation. Account / Site owns those business records; Authorization owns membership and permissions.

When work adds or changes cloud-to-device command behavior, ACK/result semantics, claimed add-on runtime behavior, error reporting, log upload requests, pre-signed object-storage URLs, topology uploads, diagnostics bundle uploads, backup artifacts, release artifacts, or add-on file transfer behavior, read `command-lifecycle.md`, `add-on-runtime-error-and-artifact-contract.md`, and `artifact-upload-broker.md` before implementation. Generic artifact transfer remains deferred in v0; approved bounded flows (e.g., backup/diagnostics) follow this boundary and API/Facade contracts.

When work adds or changes desired/reported edge state, AWS IoT named shadows, publish-policy delivery to devices, or convergence projections, read `edge-state-adapter.md` before implementation. The Edge State Adapter owns shadow I/O; product services own the product truth that drives desired state.

When a decision is marked open in `outstanding-decisions.md`, do not settle it inside a local service plan without updating that file.

When work adds or changes cross-service operational detection, runaway-device handling, monitor rules, internal incidents, or remediation recommendation behavior, read `platform-health-monitoring-service.md` before implementation. Platform Health / Monitoring is future, not v0.

## Workstream Index

| Workstream | Doc | Applies When | Reconciles With |
| --- | --- | --- | --- |
| Identity and authorization | `workstreams/identity-and-authorization.md` | Authn, authz, roles, service auth, audit context, route protection | `auth-service.md`, `auth-mfa.md`, service API plans |
| Observability | `workstreams/observability.md` | Logs, metrics, traces, health checks, audit/event correlation | All services and deploy plans |
| Platform health monitoring | `platform-health-monitoring-service.md` | Cross-service health findings, monitor rules, runaway device messaging, internal remediation candidates | Observability, Telemetry Ingest, Device Registry, Command, Artifact Broker |
| Deployment | `workstreams/deployment.md`; readiness matrix in `workstreams/deployment-readiness-matrix.md`; operator inputs in `workstreams/operator-prerequisites.md` | Runtime environments, IaC, CI/CD, release, rollback, operational readiness, operator-owned account/provider values | Service build plans, infra plans |
| Topic design | `workstreams/topic-design.md` | MQTT topics, AWS IoT rules, message envelopes, schema hints, lifecycle routing | `aws-iot-routing-contract.md`, `telemetry-ingest-architecture.md` |
| Edge state adapter | `edge-state-adapter.md` | AWS IoT named shadows, compact desired/reported edge state, convergence projections | State change and policy propagation, Command Lifecycle, Service Map |
| Secrets and config | `workstreams/secrets-and-config.md` | Cloud secrets, local device secrets, config injection, rotation, redaction | Add-on docs, deploy plans, service configs |
| Device lifecycle | `workstreams/device-lifecycle.md` | Device lifecycle states, trust/authority rules, claiming, device registry, pairing, AWS IoT CSR signing/provisioning, runtime identity resolution, release/transfer/revoke | `enrollment-claiming-contract.md`, `service-map.md`, add-on implementation, ingest and command plans |
| Local/cloud trust boundaries | `workstreams/local-cloud-trust-boundaries.md` | Local command authority, host writes, cloud control, add-on policy, future supervisor | `workstreams/identity-and-authorization.md`, command/update plans |
| State change and policy propagation | `workstreams/state-change-and-policy-propagation.md` | Desired state, local accepted policy, event budgets, entitlement-driven limits, config refresh | Topic, command, update, diagnostics, backup, and add-on plans |
| Test environments and fixtures | `workstreams/test-environments-and-fixtures.md` | Local fake adapters, staging, contract fixtures, end-to-end validation | Service build plans and CI/CD |
| Migration strategy | `workstreams/migration-strategy.md` | Durable DB, local state, topic, schema, credential, and ownership changes | Service data models, add-on state, deploy plans |

## Reconciliation Checklist

Every local service plan should include:

- applicable workstreams
- owned state and external state
- inbound and outbound contracts
- API Facade route contract, when HTTP routes are involved
- secrets and config model
- observability obligations
- deploy and rollback shape
- fixtures or contract tests
- temporary exceptions, if any

If a workstream is relevant but not yet settled, the service plan should mark the dependency as a decision. Do not fill the gap by inventing a one-off local rule.

For launch-critical non-functional work, `workstreams/deployment-readiness-matrix.md`
is the concrete cross-workstream checklist. Service and infra plans should use
that matrix for resource inventory, secret/config classes, CI/CD gates, smoke
checks, alarms, and runbooks rather than duplicating local readiness rules.
Use `workstreams/operator-prerequisites.md` for operator-owned account,
provider, budget, domain, and approval inputs.
When the user explicitly greenlights first deploy execution, read
`workstreams/first-deploy-greenlight.md` and follow it as the task protocol.

## Workstream Doc Shape

Use this shape for new cross-cutting docs:

- Purpose
- Agent Use
- Current Anchors
- Principles
- Implementation Defaults
- Required Local Plan Checks
- Closed Defaults / Open Questions, only when unresolved judgment remains
- Acceptance Criteria

The workstream doc should stay focused on shared policy and coordination. Concrete tasks belong in service plans.
