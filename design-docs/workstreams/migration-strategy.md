# Migration Strategy Workstream

Migration strategy covers durable changes that must move safely across time. It is not only database migration. In HomeSignal, migrations may involve Postgres schema, local add-on state, device credentials, MQTT topics, message schemas, deployment config, and ownership boundaries.

## Agent Use

Read this when changing:

- database schemas
- local `/config` state files
- device identity or credential records
- MQTT topics
- telemetry schemas
- API contracts
- service ownership boundaries
- config keys or secret names
- rollout behavior that must support old and new versions at once

## Current Anchors

- `enrollment-claiming-contract.md`
- `telemetry-ingest-architecture.md`
- `aws-iot-routing-contract.md`
- `telemetry-ingest-build-plan.md`
- `update-architecture.md`
- `workstreams/deployment-readiness-matrix.md`

## Principles

- A migration is any durable compatibility problem, not just a SQL file.
- Prefer backward-compatible changes when devices or services may update at different times.
- Version persisted local state and message schemas.
- Make rollback limits explicit before deploy.
- Separate product identity migration from transport credential replacement.
- Validate migrated state before deleting compatibility paths.
- Avoid requiring synchronized updates across cloud services and customer devices unless unavoidable.

## Migration Surfaces

| Surface | Examples |
| --- | --- |
| Database | table additions, constraints, indexes, backfills, state transitions |
| Device local state | `/config/device.json` schema version, secret file movement |
| Topics | topic family changes, bootstrap/runtime split, command ACK topics |
| Schemas | telemetry envelope versions, state group versions |
| Credentials | certificate rotation, revoked credentials, orphaned AWS Things |
| Config/secrets | renamed environment variables, secret path changes |
| Ownership | moving authority from one service to another |

## Implementation Defaults

- Add before remove.
- Read old and new when needed.
- Write new only after readers are compatible.
- Backfill with verification.
- Keep compatibility windows explicit.
- Use schema/version fields for device and telemetry state.
- Record rollback behavior for each deploy step.
- Treat destructive cleanup as a later, separately verified step.
- Use Goose-style SQL migrations as the v0 database migration model: ordered
  SQL migration files, explicit up steps, optional down steps only when truly
  safe, scriptable execution, and a migration table in Postgres.
- Keep migration execution behind repo scripts so local development, Codex,
  CodeBuild, and production deploys run the same command surface.
- Use `deployment-readiness-matrix.md` for production migration precheck,
  postcheck, deploy-record, and rollback/forward-fix gate requirements.
- Use additive-first database migrations. New columns/tables/indexes and
  compatibility reads ship before old fields or paths are removed.
- Keep application rollback compatible across each migration window. If a
  rollback would require destructive database undo, split the rollout until the
  old and new application versions can both tolerate the schema.
- Run backfills as explicit, verifiable steps. Treat destructive cleanup as a
  later release after production has proven the new path.
- Prefer application rollback or forward-fix for failed releases; database
  rollback is reserved for narrow, rehearsed cases where no durable production
  data would be lost or reinterpreted.
- If v0 uses Neon Postgres, preserve an explicit Neon-to-RDS migration seam:
  standard PostgreSQL schemas, ordinary migration tooling, provider-neutral
  connection config, rehearsable dump/restore or replication cutover, and no
  Neon-only application behavior.

## Database Provider Posture

Neon is acceptable for v0 when cost, speed, and development ergonomics matter
more than having the database inside the HomeSignal AWS account/VPC from day
one.

Rules:

- Treat PostgreSQL as the contract, not Neon.
- Keep all database access behind repositories/storage adapters.
- Use normal migrations and portable PostgreSQL features.
- Prefer same-region placement as AWS services when practical.
- Store credentials through the same secret injection mechanism that would later
  hold an RDS connection string.
- Keep a documented RDS cutover path for production maturity: provision RDS,
  migrate schema, load data, verify, briefly freeze writes or replicate changes,
  switch connection config, and monitor.

RDS should be revisited when private networking, fixed production posture,
provider consolidation, or operational simplicity outweigh Neon economics.

## Current V0 Migration Surface

- Source-controlled SQL migrations live in `backend/migrations`.
- The local runner is `backend/cmd/migrate`; the shared command surface is
  `scripts/migrate.sh staging [plan|status|status-if-configured|up]`.
- `scripts/deploy.sh staging` always runs migration `plan` so malformed SQL or
  duplicate versions fail before deploy.
- Deploy does not apply migrations by default. Set `HOMESIGNAL_RUN_MIGRATIONS=1`
  after the Neon PostgreSQL URL is stored in AWS Secrets Manager.
- The staging database URL secret is
  `/homesignal/staging/platform/database_url`. The secret value is managed
  outside Terraform so the database password is not stored in IaC state.
- The initial v0 schema covers account/site/device identity, device
  credentials, presence, lifecycle events, latest telemetry state, sparse
  telemetry history, and ingest failures.
- Application code should use `backend/internal/platform/database` for shared
  Postgres connection/transaction behavior and repository interfaces under
  `backend/internal/domain/ports`; route handlers should not grow direct SQL.

## Required Local Plan Checks

Every affected service plan should state:

- durable state being changed
- old and new formats
- compatibility window
- rollout order
- rollback limit
- backfill or cleanup step
- fixture coverage
- production verification query or check

## V0 Decisions (Closed)

- Add-on local state migrations should be versioned, idempotent functions that
  read the current local schema version, validate required fields, transform to
  the next schema, and write atomically.
- Local migration must back up or preserve enough prior state to recover from a
  failed write, but it must not copy private keys or secrets into broad logs,
  fixtures, or world-readable files.
- The add-on should refuse unsafe local state by entering a degraded/unclaimed
  safe mode rather than silently inventing claimed identity from partial files.
- Local state migration tests should include old-version fixtures, corrupted
  files, missing optional fields, missing secret material, and already-migrated
  idempotency cases.

## Device Compatibility Default

HomeSignal supports backward-compatible add-on/cloud changes where practical, but
does not promise indefinite support for old add-on protocols. Automatic updates
through the local Home Assistant add-on/Supervisor path are the supported
operating mode. A local administrator may disable automatic updates, but the UI
should make unsupported/drifting add-on versions visible and explain that
compatibility may eventually be revoked.

V0 compatibility window:

- Support the current add-on protocol family plus one prior compatible protocol
  family.
- Normal deprecation should provide at least 30 days notice when practical.
- Security or severe abuse issues may require immediate cutoff.
- Unsupported add-on versions should be visible in the UI.
- Disabling automatic updates is allowed locally, but drift beyond the supported
  protocol window is unsupported.

## Acceptance Criteria

- Durable changes define old/new compatibility.
- Device-facing changes do not assume instant fleet update.
- Cleanup is separated from rollout when rollback would otherwise be unsafe.
- Migration verification is documented before production use.
