# HomeSignal Staging Deployment

This is the first deployable slice for HomeSignal. It creates only the staging
control-plane runtime needed for operational smoke checks:

- Lambda custom runtime for the Go `control-plane` service.
- HTTP API routes for `GET /healthz`, `GET /readyz`, and `GET /version`.
- CloudWatch log groups with short staging retention.
- Runtime IAM role scoped to basic Lambda logging.
- Optional account-level AWS Budget when `HOMESIGNAL_BUDGET_ALERT_EMAIL` is set.

The first deploy is intentionally pinned to `us-east-1` to stay colocated with
the staging Neon region that was validated from the voice extraction API repo.

## Run

```bash
scripts/deploy.sh staging
scripts/smoke.sh staging
```

Set these environment variables before deploying when applicable:

- `AWS_PROFILE`: named staging deploy principal.
- `HOMESIGNAL_BUDGET_ALERT_EMAIL`: email recipient for the staging AWS Budget.
- `HOMESIGNAL_BUDGET_GUARDRAIL_CONFIRMED=1`: use only when the budget guardrail
  already exists outside this Terraform state.
- `HOMESIGNAL_STAGING_BUDGET_AMOUNT`: monthly budget amount, default `25`.
- `HOMESIGNAL_OWNER_TAG`: owner tag value, default `platform`.

## State

This first slice uses the Terraform or OpenTofu local state default. Before CI/CD
or production, add a remote state backend with state locking.
