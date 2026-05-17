# Operator Prerequisites

This document is the operator task list for HomeSignal deploy readiness. It
tracks values, approvals, account choices, and provider setup that cannot be
invented safely by implementation code.

It is not an implementation plan. When a value is supplied, Codex should wire,
verify, or automate it through the normal deploy path instead of asking the
operator to perform routine implementation work.

## Defaults

- First cloud environment: `staging`
- Launch cloud environments: `staging` and `production`
- No launch `dev` cloud environment
- AWS region: `us-east-1`
- Durable AWS resource prefix: `homesignal-{environment}-...`
- First deploy scope: low-cost control-plane skeleton only

## Operator Tasks Before First Staging Deploy

| Task | Needed value or action | Default/recommendation | Codex action once supplied |
| --- | --- | --- | --- |
| Confirm staging AWS account | AWS account ID and profile/role name | Use a separate staging AWS account when practical; one account with environment-scoped names is temporary only | Verify identity, region, and permissions before creating resources |
| Confirm account security baseline | Root MFA enabled, no root access keys, deploy work uses a named IAM Identity Center role, IAM role, or profile | Do not deploy from root credentials | Refuse root credentials when detectable and log the deploy principal used |
| Confirm AWS region | Region only if rejecting `us-east-1` | Keep `us-east-1` to align with verified Neon region | Pass region explicitly through scripts and IaC |
| Cost guardrail alert target | Operator email address or SNS topic | Start with email if SNS is not already established; set `HOMESIGNAL_BUDGET_ALERT_EMAIL` for the first deploy script | Create or verify AWS Budget/cost notification |
| Member-account budget enablement | Payer/management account Budgets enabled for linked accounts, or a budget created from the payer account for staging | Prefer payer-account budget ownership for Organizations member accounts | Skip member-account budget creation in staging IaC until payer-account Budgets allow it; log the guardrail task |
| Initial staging budget threshold | Monthly amount | `$25/month`, actual spend alerts at 80% and 100%, forecasted 100% when supported | Configure or document the budget guardrail |
| Confirm budget notification | Email/SNS confirmation when AWS sends it | Confirm immediately so the guardrail is active | Re-check budget state after confirmation when possible |
| Cost allocation tag overrides | Optional owner/cost-center tag values if the AWS account requires them | Use default HomeSignal tags if no account-specific cost center exists | Apply required tags through IaC/scripts |

Missing budget or alert-target setup does not block local code, scripts, or IaC
preparation. It blocks the first cloud deploy unless
`HOMESIGNAL_BUDGET_GUARDRAIL_CONFIRMED=1` records that the guardrail already
exists. When staging runs as an AWS Organizations member account, budget
creation may need to happen from the payer/management account before the member
account can own a budget.

## Later Operator Tasks

| Task | Needed before | Default/recommendation | Codex action once supplied |
| --- | --- | --- | --- |
| HomeSignal Neon project/database | Database and migration slice | Create a HomeSignal-owned Neon project/database in `us-east-1`; do not reuse the voice-extraction database | Store connection material through HomeSignal secret paths and keep app code provider-neutral |
| Hosted zone and domain names | Custom DNS/API domains | Skip custom DNS for first deploy; use generated AWS endpoint | Add Route 53/ACM/API Gateway domain wiring when ready |
| Resend account and sender verification | Notification/email slice | Use Resend with sandbox/test recipient policy first | Wire `RESEND_API_KEY`, sender config, outbox processing, and test send smoke |
| Production AWS account | Production deploy preparation | Separate production account before customer launch | Scaffold/verify production IaC without applying until approved |
| Production budget threshold and alert target | First production deploy | Use a conservative temporary threshold such as `$100/month`, then revise before customer traffic | Configure production actual and forecasted spend alerts |
| Production approval location | Production gate | A ticket, change record, or explicit operator approval record | Require approval reference in production deploy record |

## Agent-Owned Work

Codex should handle these without treating them as operator tasks once the
required account/provider access exists:

- create or verify AWS Budgets/cost notifications
- create or verify IaC state and lock resources
- name AWS resources using `deployment-readiness-matrix.md`
- tag AWS resources using `deployment-readiness-matrix.md`
- create scripts and deploy/smoke wrappers
- validate AWS identity, region, quotas, and basic permissions
- create low-cost staging skeleton resources
- log missing external input in the first-deploy judgment format
- avoid production resources unless the operator explicitly asks for production

## Hygiene Rules

- Do not ask the operator to choose routine implementation details.
- Do not ask the operator to create resources manually when credentials allow
  Codex to create them through scripts/IaC.
- Do not print or store real secret values in docs, logs, or examples.
- Treat budget thresholds, account IDs, domains, and provider account names as
  setup values, not architecture changes.
- If the operator rejects a default, record the override in the touched deploy
  doc and continue with the new explicit value.
