# First Deploy Greenlight Protocol

This is the execution protocol for the first real HomeSignal deploy.

When the user gives a clear greenlight such as "start first deploy", "go to
town on deploy", or "greenlight the staging deploy", the agent should read this
file first, then execute the sequence as far as the current credentials and
environment allow.

This protocol is intentionally pragmatic. It exists to prevent two failure
modes:

- building CI/CD before a deploy process has been proven once
- trying to create the whole AWS platform in one pass

## Operating Rules

- Stay in implementation mode once greenlit.
- Do not ask the user about implementation minutia.
- Use the defaults in this file unless a hard blocker or true product/operator
  judgment is required.
- Log judgment/external-input items and continue around them when possible.
- Stop only for missing credentials/access, destructive operations, cloud spend
  or account choices that cannot be safely defaulted, or an architecture
  conflict with canonical docs.
- Do not create production resources during first-deploy work unless the user
  explicitly asks for production.
- Do not build CodeBuild/CI/CD until `scripts/deploy.sh staging` and
  `scripts/smoke.sh staging` work locally or the blocker is explicitly logged.

## Read First

Read these before making deploy changes:

1. `AGENTS.md`
2. `design-docs/architectural-decision-log.md`
3. `design-docs/platform-doctrine.md`
4. `design-docs/workstreams/first-deploy-greenlight.md`
5. `design-docs/workstreams/deployment-readiness-matrix.md`
6. `design-docs/workstreams/deployment.md`
7. `design-docs/api-facade.md`
8. `design-docs/workstreams/observability.md`
9. `design-docs/workstreams/operator-prerequisites.md`
10. `design-docs/implementation-plan.md`, especially M0 and M14
11. `design-docs/workstreams/secrets-and-config.md`
12. `design-docs/workstreams/test-environments-and-fixtures.md`
13. `design-docs/workstreams/migration-strategy.md`

## Defaults

Use these unless the user has already provided a different value:

| Concern | Default |
| --- | --- |
| First environment | `staging` |
| AWS region | `us-east-1`, selected to align with the verified Neon AWS region |
| Resource names | Follow `deployment-readiness-matrix.md`; first deploy uses `homesignal-staging-*` names only |
| Runtime target | API Gateway-facing Go control-plane skeleton; prefer Lambda or another API Gateway-native integration when it fits |
| First deployed service | Control-plane skeleton only |
| First routes | operational `GET /healthz`, `GET /readyz`, and `GET /version` only |
| DNS/custom domain | Stage 0 skeleton may skip custom DNS and use the generated AWS endpoint. Domain-backed staging is required before public pairing, browser bridge, HA App environment profiles, email links, or customer/integrator-visible staging UX. |
| Database | Do not require DB for first skeleton liveness |
| AWS IoT | Defer until enrollment/device runtime slice |
| Agent mTLS | Defer until the Agent HTTPS boundary slice |
| Object storage | Create IaC state or build artifact storage only if the selected deploy path needs it; defer product object storage |
| Email/Resend | Defer until Notification Service slice |
| CI/CD | Defer until script-driven staging deploy works |

## Staging Domain Path

Staging has two deploy phases:

1. **Stage 0: skeleton smoke**
   Use the generated AWS endpoint. Prove artifact build, runtime boot, IaC,
   deploy script, smoke script, logs, `/healthz`, `/readyz`, and `/version`.
   Do not expose customer or integrator pairing UX from this endpoint.
2. **Stage 1: domain-backed staging**
   Use an owned stable HTTPS domain before enabling public pairing pages,
   browser localStorage/postMessage bridge behavior, Home Assistant app staging
   environment profiles, claim flows, email links, or any human-visible staging
   UX that depends on browser origin trust.

If no domain exists yet, proceed with Stage 0 and record the domain as a Stage
1 blocker. Do not work around missing DNS by adding a freeform cloud URL field,
a hidden URL-parameter override, or a generated AWS endpoint to durable Home
Assistant app environment config.

## Required First Deploy Shape

The first deploy proves only:

- source can build a deployable artifact
- staging infrastructure can be created from IaC
- runtime can boot
- logs are visible
- `/healthz`, `/readyz`, and `/version` respond
- smoke script can verify the deployed endpoint
- deploy and smoke are scriptable

It does not prove:

- enrollment
- Postgres migrations
- mTLS device auth
- AWS IoT provisioning or lifecycle
- telemetry ingest
- notification delivery
- production readiness

## Execution Sequence

### 0. Preflight

- Run `codex-title-sync enable`.
- Check `git status --short`.
- Identify unrelated dirty files and do not revert them.
- Check available AWS tooling and current identity if AWS CLI is present.
- Resolve the AWS account ID and planned region before preparing cloud
  resources.
- Do not deploy from root credentials. Use a named IAM Identity Center role, IAM
  role, or profile and log the deploy principal.
- Create or verify the staging account budget/cost guardrail when credentials,
  permissions, and alert recipient are available. If not available, log the
  blocker and prepare local artifacts without running a cloud deploy unless an
  existing budget guardrail is explicitly confirmed.
- If AWS credentials/profile are absent, prepare local scripts/IaC and log the
  credential blocker.

### 1. Script-First Surface

Create or update root scripts named by the readiness matrix:

- `scripts/test.sh`
- `scripts/build.sh`
- `scripts/migrate.sh`
- `scripts/deploy.sh`
- `scripts/smoke.sh`
- `scripts/logs.sh`
- `scripts/rotate-db-credentials.sh`
- `scripts/cleanup-staging-fixtures.sh`

Minimum first behavior:

- `test.sh` runs current local tests.
- `build.sh` builds the first deployable artifact.
- `deploy.sh staging` deploys the staging skeleton or clearly reports the
  missing external prerequisite.
- `deploy.sh staging` refuses cloud deployment until the staging budget
  guardrail can be created or is explicitly confirmed as already present.
- `smoke.sh staging` verifies `/healthz`, `/readyz`, and `/version`.
- scripts accept environment as an argument where relevant.
- unimplemented future surfaces should fail clearly with a useful message, not
  silently succeed.

### 2. Control-Plane Skeleton

If absent, create the minimal backend skeleton from M0:

- Go module under `backend/`
- `cmd/control-plane`
- `/healthz`
- `/readyz`
- `/version`
- config loader with `HOMESIGNAL_ENV`, `HOMESIGNAL_SERVICE_NAME`, and
  `HOMESIGNAL_VERSION`
- structured startup/request logs with service/environment/version

No database, Cognito, AWS IoT, mTLS, or domain routes are required for the
Stage 0 skeleton deploy.

These endpoints are operational liveness/version endpoints, not public
`/api/v1` product routes or `/agent/*` device routes. Any product route,
enrollment route, internal route, or agent route still follows `api-facade.md`
and the source-controlled OpenAPI rule where applicable.

### 3. IaC Staging Skeleton

Create the smallest staging IaC needed for the skeleton:

- IaC env path for staging
- state/backend documentation or bootstrap script; if local IaC state is used
  for the first staging attempt, record it as a temporary exception with a
  removal condition before CI/CD or production
- runtime role
- log group
- public API/runtime integration for the skeleton
- output containing the staging base URL

Do not create production resources in this step.

### 4. Manual Staging Deploy

Use local scripts first:

```text
scripts/test.sh
scripts/build.sh
scripts/deploy.sh staging
scripts/smoke.sh staging
```

If deployment reaches AWS, capture:

- AWS account ID
- region
- staging endpoint
- artifact/version
- smoke result
- log group name

### 5. CI/CD Wrapper

Only after the script-driven staging deploy works, add thin CI/CD:

- CodeBuild test/build project
- optional GitHub Action to trigger/report CodeBuild
- CodeBuild deploy-staging project only after deploy script is proven

CI/CD must call repo scripts. It must not become a separate deploy system.

### 6. Next Boundaries

After first deploy is stable, add boundaries one at a time:

1. database and migrations
2. auth shell
3. enrollment API
4. AWS IoT provisioning
5. Agent HTTPS mTLS
6. Telemetry Ingest
7. command ACK/result
8. object storage/artifacts
9. notification/email
10. production gate

## Stop Conditions

Stop and report if:

- AWS credentials are missing or cannot identify an account
- required cloud permissions are denied and no safe local fallback remains
- an operation would create production resources without explicit approval
- an operation would require a broad admin/security choice not covered here
- a destructive command would be needed
- canonical docs conflict and cannot be reconciled by local implementation

## Judgment Log Format

When judgment/external input is needed, log it like this and continue where
possible:

```text
Judgment/external input needed:
- Item: <decision or value>
- Why it matters: <impact>
- Default/recommendation: <agent recommendation>
- Current action: <continued with default | blocked until supplied>
```

Known likely external inputs:

See `operator-prerequisites.md` for the canonical operator task list. The common
first-deploy inputs are:

- AWS account/profile to use
- account security baseline and deploy principal
- budget/cost alert recipient and initial staging threshold
- cost allocation tag overrides, if the AWS account requires them
- hosted zone/domain names before Stage 1 domain-backed staging
- region override only if the operator explicitly rejects selected `us-east-1`
- production approval location
- Neon and Resend account details for later slices

## Completion Criteria

First-deploy greenlight work is complete when one of these is true:

- staging deploy succeeds and `scripts/smoke.sh staging` verifies the running
  skeleton
- or all local deploy artifacts/scripts/IaC are prepared and the only remaining
  blocker is logged external access/input

The final handoff must include:

- files changed
- commands run
- deploy endpoint if created
- smoke result
- blockers or judgment log
- next recommended slice
