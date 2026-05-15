# Secrets And Config Workstream

Secrets and config policy covers cloud runtime secrets, local add-on secret files, non-secret configuration, rotation, redaction, and cleanup.

## Agent Use

Read this when touching:

- environment variables
- runtime config loaders
- cloud-provisioned add-on policy/config
- AWS Secrets Manager or SSM Parameter Store
- local add-on secret files
- certificates or private keys
- claim invite codes or claim verification tokens
- logs around credentials
- Docker images or deployment manifests

## Current Anchors

- `enrollment-claiming-contract.md`
- `telemetry-ingest-build-plan.md`
- `service-map.md`
- `workstreams/deployment-readiness-matrix.md`
- `workstreams/local-cloud-trust-boundaries.md`
- `state-change-and-policy-propagation.md`

## Principles

- Secrets are never source code, docs examples with real values, logs, metrics labels, images, or committed files.
- Non-secret config should be explicit and environment-specific.
- Cloud runtime secrets should be injected by the platform.
- Local device secrets should be stored separately from metadata.
- Temporary credentials must expire and be removed when no longer needed.
- Rotation and revocation are normal operations, not exceptional disasters.

## Implementation Defaults

Cloud services:

- inject secrets from AWS Secrets Manager or SSM Parameter Store
- use environment variables or parameter store for non-secret config
- avoid baking secrets into images
- redact secret-like fields in structured logs
- use standard environment names: `staging` and `production`
- use AWS secret paths shaped as
  `/homesignal/{environment}/{service}/{secret_name}`
- use non-secret parameter paths shaped as
  `/homesignal/{environment}/{service}/config/{config_name}`
- use runtime environment variables shaped as `HOMESIGNAL_<NAME>` for shared
  app config and `HOMESIGNAL_<SERVICE>_<NAME>` only when the setting is
  service-specific
- include `HOMESIGNAL_ENV`, `HOMESIGNAL_SERVICE_NAME`, and
  `HOMESIGNAL_VERSION` in each service runtime
- use `deployment-readiness-matrix.md` for the v0 service-level secret/config
  inventory before creating service-specific names
- prefer AWS IAM/service identity over manually rotated static service
  credentials

Home Assistant add-on:

- store metadata in `/config/device.json`
- store secret material in separate `0600` files
- never send the local private key to HomeSignal
- remove temporary claim verification material after final provisioning succeeds or expires
- avoid displaying claim verification tokens or durable secrets in the local UI
- HomeSignal does not own backup/restore of the local device private key.
  Losing local key material is handled through repair/reconnect/credential
  replacement flows, not a HomeSignal-managed secret backup.

Rotation defaults:

- Broad automatic service-credential rotation is not required in v0.
- Neon/Postgres database credentials are the day-zero exception because they are
  core infrastructure and painful to retrofit later.
- Neon/Postgres rotation must have a tested runbook or script before production
  launch, including staging rotation, service reload/restart behavior, and
  rollback/restore expectations.
- AWS-managed/IAM credentials should rely on AWS role/session mechanics rather
  than HomeSignal-built rotation.

## Required Local Plan Checks

Every affected service plan should state:

- secret names or classes, without real values
- non-secret config keys
- injection mechanism
- local file permissions, if applicable
- rotation/revocation behavior
- redaction behavior
- cleanup behavior for temporary material
- test fixture strategy without real secrets

## V0 Decisions (Closed)

V0 broad secrets/config policy is settled in this workstream.

## Device Credential Replacement Default

Device certificate replacement uses a primary/secondary credential slot model.
During an explicit repair or rotation flow, both credentials may be accepted for
the same `device_id` for a short overlap window. Default overlap is a few hours;
24 hours is the maximum without an explicit operational exception. Revoked,
expired, unknown, or unbound certificates must be rejected by backend credential
status even when their issuing CA remains trusted by the HTTPS edge.

## Acceptance Criteria

- No service requires a secret baked into an artifact.
- Local private keys remain local.
- Temporary poll/session material has an expiry and cleanup path.
- Logs and fixtures do not expose secret material.
