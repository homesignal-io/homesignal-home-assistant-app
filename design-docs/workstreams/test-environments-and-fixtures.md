# Test Environments And Fixtures Workstream

Test environments and fixtures define how HomeSignal proves service behavior locally, in staging, and before production changes affect customer sites.

In this doc, a fixture means reusable test input or a controlled test asset. It can be a JSON/MQTT message example, a fake adapter response, seed data, or a real temporary Home Assistant instance used for staging validation. Fixtures are not production customer data.

## Agent Use

Read this when touching:

- service tests
- fake adapters
- contract fixtures
- staging environments
- CI checks
- AWS integration tests
- add-on enrollment tests
- telemetry schema tests
- deployment smoke tests

## Current Anchors

- `telemetry-ingest-build-plan.md`
- `telemetry-ingest-architecture.md`
- `enrollment-claiming-contract.md`
- `aws-iot-routing-contract.md`
- `workstreams/deployment-readiness-matrix.md`
- `homesignal/README.md`

## Principles

- Every important boundary should have a fixture or contract test.
- Local tests should not require live AWS for core business logic.
- Staging should exercise production-shaped infrastructure before production, including real AWS IoT Core and the Agent HTTPS mTLS path.
- Fake adapters should preserve real contracts, not invent easier behavior.
- Test data must not contain real secrets or real customer data.
- Fixtures should encode both happy paths and failure paths.
- Verification belongs to the agent making the change.

## Implementation Defaults

- Use unit tests for pure policy and state-machine behavior.
- Use contract-example tests for Agent HTTPS runtime envelopes: route, envelope fields, payload, authenticated device context, and expected ingest/command result.
- Use fixture-driven tests for enrollment contracts and local fake adapters.
- Keep AWS-specific code behind adapters so local tests can use fakes.
- Maintain a simulator device for local/dev validation of Agent HTTPS envelopes, policy behavior, command ACK/result, shadow convergence, and ingest expectations.
- Run simulator contract tests in CI before merge for changed runtime contracts.
- Maintain at least one long-lived staging canary device as a real Home Assistant/add-on instance paired through staging AWS IoT Core.
- Use ephemeral Home Assistant/add-on instances for enrollment, claim, repair/reconnect, and fresh-claim lifecycle tests.
- Maintain golden examples for supported telemetry schemas.
- Add smoke checks for deployed service health, readiness, and one core workflow.
- Use `deployment-readiness-matrix.md` for required staging live tests,
  production smoke checks, and staging fixture cleanup expectations.
- Prefer deterministic IDs and timestamps in fixtures.
- Mark live cloud tests separately from default local tests.

## Required Local Plan Checks

Every affected service plan should state:

- unit test coverage
- contract fixtures
- fake adapter behavior
- staging validation
- live cloud test requirements, if any
- smoke checks after deploy
- failure cases covered
- fixture update requirements for contract changes

## V0 Decisions (Closed)

- Contract fixtures live under service-local `testdata/fixtures/...` paths when
  a package owns the test, or under a repo-level fixture path only when multiple
  packages share the same contract examples.
- Fixture names should include boundary, family, case, and version where useful,
  such as `agent_https_telemetry_health_v1_valid.json` or
  `enrollment_claim_context_cross_account_v1.json`.
- Live integration tests are marked separately from default local tests. Prefer
  an explicit integration marker/build tag plus names beginning with
  `TestLive...` or `TestStaging...`.
- Staging seed data must be non-customer data, deterministic where practical,
  and safe to recreate. Seed identities should be visibly staging-scoped.
- Ephemeral Home Assistant containers and paired AWS IoT test devices should use
  names like `hs-stg-<purpose>-<yyyymmdd>-<shortid>`, carry cleanup metadata, and
  be cleaned by an explicit staging cleanup script or scheduled job.

## Acceptance Criteria

- Contract changes include fixture updates.
- Business logic can be tested without live cloud credentials.
- Deployment plans include smoke checks.
- Staging tests use non-customer data and production-shaped infrastructure.
