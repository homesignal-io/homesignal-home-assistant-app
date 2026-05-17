# Deployment Workstream

Deployment is the platform policy for how HomeSignal services move from code to running environments. It includes environment shape, infrastructure ownership, release sequencing, rollback, and operational readiness.

## Agent Use

Read this when touching:

- service build plans
- Dockerfiles or runtime packaging
- infrastructure as code
- CI/CD
- environment variables
- database migrations
- AWS resource creation
- release or rollback procedures
- staging or production validation

## Current Anchors

- `service-map.md`
- `telemetry-ingest-build-plan.md`
- `update-architecture.md`
- `deployment-readiness-matrix.md`
- `first-deploy-greenlight.md`
- `operator-prerequisites.md`

## Principles

- Use production-shaped environments from the beginning.
- Launch has two cloud environments only: staging and production.
- Prefer infrastructure as code over hand-created resources.
- Separate build, deploy, migrate, and verify steps.
- A deploy plan must include rollback or forward-fix expectations.
- Staging must use real AWS IoT Core, real IoT Rules, and the same class of cloud infrastructure as production.
- Runtime secrets and non-secret config should be injected, not baked into images.
- Services should be deployable independently only when their contracts and compatibility allow it.
- Public edge routes are limited to product/client APIs and authenticated
  agent/device APIs. Internal routes must not be internet-facing.
- Routine developer/Codex operations must be scriptable through AWS CLI and repo
  scripts; production operations must not depend on console-only or VPN-only
  workflows.
- Use a minimal VPC skeleton for future private resources and stateful services,
  but do not force every v0 service into private networking before there is a
  concrete need.
- Avoid NAT Gateway/private-egress complexity until a service has a concrete
  requirement that justifies the fixed cost and operational surface.
- V0 may use Neon Postgres for cost and development velocity; RDS is not required
  solely for AWS purity.
- If Neon is used, prefer the same AWS region as HomeSignal services when
  practical, keep database access behind storage/repository adapters, and avoid
  Neon-specific application semantics so a future RDS migration remains
  straightforward.

## Implementation Defaults

- Use immutable build artifacts.
- Use environment-specific configuration, not environment-specific code paths.
- Keep staging structurally similar to production. Do not introduce a separate preprod, demo, or launch environment unless a later product decision explicitly creates one.
- CI/CD is script-first: repo-owned scripts are the real build, test, migrate,
  deploy, and smoke-test interface.
- AWS CodeBuild is the preferred runner for AWS-heavy build, test, and deploy
  work because it can run with AWS IAM roles and avoids making GitHub-hosted
  runner minutes the center of gravity.
- GitHub Actions may be used for lightweight repo feedback or to trigger
  CodeBuild, but should not own AWS deployment behavior.
- AWS CodePipeline remains optional; do not require it unless promotion,
  approvals, or multi-stage orchestration justify the added service.
- Use OpenTofu/Terraform-style infrastructure as code for AWS resources.
  OpenTofu is the default preference unless provider/tooling friction makes
  Terraform materially simpler for the v0 stack.
- Deploy the control-plane API/domain backend as one monolith in v0.
- Deploy Telemetry Ingest as the only separately deployable v0 backend service.
- Use `deployment-readiness-matrix.md` as the canonical v0 non-functional
  readiness inventory for resources, secrets/config, CI/CD gates, smoke checks,
  alarms, and launch runbooks.
- When the user greenlights first deploy work, follow
  `first-deploy-greenlight.md` as the execution protocol.
- Use `operator-prerequisites.md` for account/provider values, budget alert
  targets, production approval location, and other operator-owned setup inputs.
- Describe AWS resources in IaC before relying on them operationally.
- Run database migrations as explicit deploy steps with pre/post checks.
- Gate promotion on service health, readiness, and smoke checks.
- Prefer small, reversible deploy increments.
- Record temporary manual deploy steps as exceptions with a removal condition.
- Store database connection material in the same secrets/config path the service
  would use for RDS later; do not let provider choice leak into domain code.
- Provide scriptable operational entrypoints for common tasks such as deploy,
  service health, log tailing, smoke checks, and controlled private access when
  needed.
- Schema changes should be additive-first and app-rollback-compatible. Do not
  pair destructive schema cleanup with the same release that begins relying on a
  new shape. Backfills, verification, and destructive cleanup are separate
  deploy steps. Rollback is usually application rollback or forward-fix, not a
  blind database undo.
- Staging deploys may be automatic/script-driven after tests pass.
- Production deploys require explicit operator approval after staging smoke
  checks pass.
- Production migrations require an explicit precheck and postcheck.
- Every production deploy record must state rollback or forward-fix
  expectations.

## Current First Deploy Slice

The first staging deploy is script-driven and intentionally narrow:

- service: Go control-plane skeleton in `backend/`
- runtime: Lambda custom runtime behind HTTP API Gateway
- region: `us-east-1`
- routes: `GET /healthz`, `GET /readyz`, and `GET /version`
- IaC: `infra/envs/staging`
- scripts: `scripts/test.sh`, `scripts/build.sh`, `scripts/deploy.sh staging`,
  `scripts/smoke.sh staging`, and `scripts/logs.sh staging`

It does not include database access, Cognito/auth, AWS IoT, Agent mTLS, email,
object storage, product routes, production, or CI/CD. CI/CD wraps these scripts
only after `scripts/deploy.sh staging` and `scripts/smoke.sh staging` work
locally.

The first staging cloud deploy requires either `HOMESIGNAL_BUDGET_ALERT_EMAIL`
for the staging budget guardrail task, or
`HOMESIGNAL_BUDGET_GUARDRAIL_CONFIRMED=1` when the guardrail already exists.
When staging is an AWS Organizations member account, the actual AWS Budget may
need to be enabled or created from the payer/management account rather than the
member-account deploy workspace.

## Required Local Plan Checks

Every affected service plan should state:

- runtime target
- build artifact
- required infrastructure
- config and secret injection
- deployment order
- migration requirements
- smoke checks
- rollback or forward-fix behavior
- operational alarms required before production traffic

## V0 Decisions (Closed)

V0 broad deployment policy is settled in this workstream.

## Acceptance Criteria

- No production service depends on undocumented manual infrastructure.
- Deployment plans describe how to verify readiness after deploy.
- Migrations and runtime rollout are sequenced deliberately.
- Rollback limitations are stated before implementation.
