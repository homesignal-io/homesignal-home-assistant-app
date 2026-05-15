# Identity And Authorization Workstream

Identity and authorization is both a service responsibility and a cross-cutting platform concern. The Auth/RBAC service owns the central model, but every service that accepts requests or performs sensitive work must comply with the authorization boundary.

## Agent Use

Read this when touching:

- human authentication
- route authorization
- service-to-service authorization
- account, site, user, role, or permission models
- audit context for sensitive actions
- customer/dealer/support access
- command, enrollment, diagnostics, update, backup, or remote access authority

Then reconcile the local plan with `auth-service.md`.

## Current Anchors

- `auth-service.md`
- `auth-mfa.md`
- `service-map.md`
- `workstreams/local-cloud-trust-boundaries.md`

## Principles

- Cognito owns human authentication.
- The application owns authorization.
- Account is the authority container.
- Site is the durable data container.
- Authorization must go through a central interface, not route-local role checks.
- Permissions are additive in v0.
- No hidden vendor god-mode.
- Auth-sensitive actions must be audited.
- API keys authenticate service account subjects, not themselves.
- Device transport identity is not human or service authorization.

## Service Versus Workstream Boundary

The Auth/RBAC service owns:

- users
- account memberships
- membership invitations
- roles
- permissions
- access grants
- service account subjects
- permission evaluation

Account / Site owns account records, customer records, sites, buildings, zones, and site relationships. Authorization consumes those facts when evaluating permissions.

Each local service owns:

- calling the central authorization interface
- passing the correct subject, action, resource, and context
- rejecting unauthorized work before side effects
- attaching audit context to sensitive actions
- avoiding local permission shortcuts

## Implementation Defaults

- Use Cognito JWT verification for human authentication.
- Map Cognito subjects to local users before authorization.
- Use `AuthorizationService.can(subject, action, resource, context)` as the conceptual boundary.
- Store authorization state in Postgres.
- Audit claim, release, command, credential, access, update, diagnostics, and backup actions.
- Treat service-to-service auth as subject-based authorization, not raw token possession.
- For AWS-hosted out-of-process service calls, use AWS IAM role identity and
  SigV4-signed requests as the default service-to-service authentication
  mechanism.
- Map the verified AWS principal/role to a HomeSignal service subject such as
  `service:telemetry-ingest`, then run app-level authorization with that
  subject.
- Do not invent static shared internal API keys for AWS-hosted service calls.
- Keep logical service calls contract-shaped even inside the v0 monolith. A call
  between logical services should take an explicit request/context object and
  return a typed response/error shape that can later be adapted to HTTP without
  rewriting the domain boundary.
- Do not let in-process calls bypass the same subject, action, resource, and
  context checks that a future HTTP service boundary would require.

## Extractable Service Contract Default

V0 domain services may run in one process, but their interfaces should look like
future service contracts.

Default shape:

```text
API/worker adapter
  -> builds RequestContext
  -> validates route/command envelope
  -> calls logical service method with typed request
  -> logical service authorizes via AuthorizationService where needed
  -> logical service returns typed response or standard error
```

When a logical service is later split into its own deployable, the adapter should
move from in-process method call to HTTP/internal RPC without changing the
business contract. AWS-hosted HTTP/internal RPC calls should use IAM/SigV4 at
the transport boundary and then map the AWS principal into the same HomeSignal
service subject used by the in-process `RequestContext`. Network placement and
VPC boundaries are deployment/security concerns layered around the same contract
shape.

Example mapping:

```text
arn:aws:iam::123456789012:role/homesignal-telemetry-ingest
  -> service:telemetry-ingest
```

This does not require every internal call to be HTTP in v0. It does require the
contract to be explicit enough that extracting Telemetry Ingest, Artifact Broker,
Diagnostics, Command, or Alerting does not force a domain rewrite.

## Required Local Plan Checks

Every affected service plan should state:

- which actors can call each route or worker path
- which resource is authorized
- which action name is evaluated
- which audit events are emitted
- what happens when a local user is disabled or a site relationship ends
- whether service accounts are involved
- whether the work crosses into device/local authority

## V0 Decisions (Closed)

- Authorization action names use `resource:action` with lower-case resource
  names and lower_snake_case actions, such as `device:command`,
  `member:update_role`, `backup:trigger`, or `debug_session:start`.
- Command-specific permission details belong in authorization context rather
  than exploding every command into a separate top-level action name, unless a
  command family becomes product/security-distinct enough to warrant its own
  action.
- Physically split AWS-hosted services use IAM/SigV4 service identity plus the
  same app-level authorization contract described above.
- Network placement follows `workstreams/deployment.md`: internal routes are
  not internet-facing when split, keep a minimal VPC skeleton for future private
  resources, and avoid forcing every v0 service into private networking before
  there is a concrete need.

## Acceptance Criteria

- No route or worker performs sensitive work without a named authorization check.
- No service grants authority from raw API keys, device certs, MQTT client IDs, or payload fields.
- Local plans identify audit events before implementation.
- Auth service changes update both service architecture and affected cross-cutting checks.
