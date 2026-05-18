# Device Lifecycle Workstream

This workstream defines how HomeSignal devices exist, change, operate, and leave the system over time. It uses the original provisioning and enrollment work as the seed, but the scope is broader: provisioning is one phase in the lifecycle, not the whole lifecycle.

The document does not own every mechanism a device can perform. Update architecture owns update mechanics. Backup owns backup mechanics. Telemetry Ingest owns telemetry mechanics. This workstream owns the lifecycle states, transition rules, and trust/authority expectations those services must preserve.

## Guiding Principle: Trust, Authority, And Fidelity

A HomeSignal device has a lifecycle. Trust and authority are the rules of the road for managing that lifecycle without losing identity, history, ownership, or operational fidelity.

HomeSignal should be able to answer, at every point in the lifecycle:

- which durable HomeSignal device this is
- which site/account currently has authority over it
- which credential or session is proving transport identity right now
- which state transition is being attempted
- which service owns that transition
- which claims are evidence only, not authority
- whether history should remain attached to the same device/site record

Trust is created during provisioning and enrollment, maintained during claimed runtime operation, changed during rotation/transfer/release, and ended during revocation/removal. Each lifecycle phase must make the trust mechanics explicit rather than relying on local assumptions.

## Agent Use

Read this when touching:

- local installation identity
- claim invites
- claim verifications
- device registry records
- SaaS claim flows
- app enrollment state
- AWS IoT CSR signing/provisioning adapter behavior
- final claim confirmation
- AWS IoT credential mapping
- post-claim device HTTPS authentication
- runtime message identity resolution
- device command authority
- device/site/account binding
- credential rotation
- release, transfer, revocation, or removal flows

## Current Anchors

- `enrollment-claiming-contract.md`
- `api-facade.md`
- `service-map.md`
- `aws-iot-routing-contract.md`
- `telemetry-ingest-architecture.md`
- `telemetry-ingest-build-plan.md`
- `command-lifecycle.md`
- `edge-state-adapter.md`
- `workstreams/local-cloud-trust-boundaries.md`

## Lifecycle Overview

HomeSignal device lifecycle states:

```text
local installation
  -> unclaimed app
  -> pairing/provisioning pending
  -> AWS IoT credential issued
  -> HomeSignal claim finalized
  -> claimed runtime operation
  -> credential rotation or repair, as needed
  -> release, transfer, revoke, or removal
```

The exact implementation can be staged, but service plans must not weaken the lifecycle model to match a shortcut.

## Device Model

Durable product identity:

- `device_id`
- issued by HomeSignal
- created during claim finalization, after HomeSignal has authorized the site/account claim intent and before claimed runtime operation begins
- stable across app restarts and AWS IoT reconnects
- remains the continuity key for product history and time-series/read-model continuity
- may survive credential rotation or re-pairing when the same product device should remain continuous

Local installation identity:

- generated and stored by the app before claim
- helps correlate pairing/provisioning attempts
- does not become cloud authority by itself
- must not be silently rebound to another site after claim
- may be lost if the user wipes app config

AWS IoT transport identity:

- Thing name
- certificate ID/ARN
- certificate fingerprint
- certificate serial
- certificate issuer
- principal identifier
- IoT endpoint/region
- AWS IoT Thing name equals HomeSignal `device_id` for claimed devices
- MQTT client ID must equal AWS IoT Thing name for claimed runtime connections
- certificate/principal material is replaceable transport credential material
- private key is generated and retained locally by the app; HomeSignal must not receive or store it
- certificate PEM may pass through HomeSignal during claim but does not need to be persisted after derived certificate identity fields are stored
- AWS IoT Core owns transport authentication, policy enforcement, and transport provenance
- not product ownership, site authority, or history authority

Runtime session provenance:

- `clientId`
- `sessionIdentifier`
- lifecycle `versionNumber`
- event timestamp
- not a HomeSignal product identity primitive
- not used by HomeSignal services to resolve product identity during normal runtime ingestion

Mutable claims:

- topic `device_id`
- payload `device_id`
- browser state
- local mutable files
- user-entered codes after expiry

These are evidence or annotations. They are never the sole authority for product state changes.

## Trust And Authority Rules

- HomeSignal `device_id` is the durable product device identity.
- For claimed devices, HomeSignal `device_id`, AWS IoT Thing name, and required MQTT client ID are the same durable identifier.
- Postgres/HomeSignal domain state is the source of product authority for account, site, claim, release, transfer, revoke, and audit state.
- HomeSignal is not irrevocable MDM for a Home Assistant installation. A local Home Assistant administrator can remove the app or reset HomeSignal local identity.
- Local reset breaks the local administrative link, but it does not grant authority to mutate prior cloud account records.
- AWS IoT Core authenticates and routes transport, enforces certificate/client/Thing policy, and owns transport provenance. It does not decide HomeSignal account/site ownership.
- AWS IoT credentials are recorded under the HomeSignal `device_id`; they are not a separate runtime join key for product identity.
- Post-claim `/agent/*` HTTPS calls use the same AWS IoT-signed device certificate identity: the edge validates the certificate chain and HomeSignal authorizes by exact stored fingerprint/serial before deriving `device_id -> site_id -> org_id`.
- Runtime ingestion uses the AWS IoT Rule-enriched durable `device_id`/Thing name and must not perform a session-to-device identity lookup.
- Topic and payload device IDs are consistency checks and routing hints, not authority.
- Time-series, latest-state, event history, command history, and audit references should preserve continuity through `device_id`.
- Credential rotation creates a new credential record under the same `device_id` when product identity should continue.
- Release/revoke changes cloud authority immediately; local cleanup/convergence is a separate transition.
- Transfer changes account/site authority through HomeSignal product state; it must not create accidental new device identity or orphan history.
- Claimed-device runtime control and wake-up traffic should use AWS IoT. Approved `/agent/*` HTTPS flows may be used after claim for command detail retrieval, artifact upload negotiation, command results, and other explicitly documented device API flows.
- Local hardware, Home Assistant, Supervisor, hostname, version, installation, and environment signals may support recognition, repair, risk scoring, or support workflows, but they are advisory only and never authorize attaching to a prior `device_id` by themselves.

## Lifecycle States And Trust Mechanics

### Local Installation

The app exists locally but is not yet trusted by HomeSignal for runtime operation.

Trust mechanics:

- app may create local installation identity and local key material
- cloud has no claimed device authority yet
- browser/local UI can display local status but cannot create durable cloud authority alone
- no operational commands are accepted

Owning docs/services:

- app implementation
- app security docs

### Unclaimed App

The app can start enrollment but is not yet a claimed runtime device.

Trust mechanics:

- direct app-to-cloud API is allowed only for enrollment/bootstrap
- claim invite codes expire, are single-use, and are verified before confirmation
- claim verification tokens are short-lived
- no claimed-device MQTT runtime traffic is accepted
- local cert files alone must not imply claimed state

Owning docs/services:

- API Facade
- Enrollment / Device Registry
- app enrollment state

### Pairing And Provisioning Pending

A SaaS user/account/site has expressed claim intent, and the app is trying to become a claimed device.

Trust mechanics:

- authenticated SaaS user authorizes site/account claim intent
- HomeSignal backend is the business authority for claim authorization
- the app generates a private key locally and submits a CSR through the HomeSignal claim flow
- HomeSignal coordinates AWS IoT certificate signing through `CreateCertificateFromCsr`
- AWS IoT returns `certificatePem`, `certificateId`, and `certificateArn`
- HomeSignal returns the certificate PEM to the app and stores AWS certificate identifiers plus derived fingerprint, serial, and issuer metadata
- HomeSignal must not receive or store the device private key
- first valid claim wins
- claim attempts and outcomes are audited
- partial success must have remediation

Default flow:

```text
web portal creates site-bound claim invite
  -> unclaimed app verifies invite code and displays invite context
  -> local user confirms the invite details
  -> HomeSignal API enrollment endpoints over HTTPS under /api/v1
  -> local app CSR
  -> HomeSignal-authorized AWS IoT CreateCertificateFromCsr
  -> AWS IoT certificate/policy/Thing binding via provisioning adapter
  -> HomeSignal finalization
```

Owning docs/services:

- `enrollment-claiming-contract.md`
- Enrollment / Device Registry
- AWS IoT provisioning adapter

### Claim Finalized

HomeSignal has confirmed the claim, created durable product identity, and recorded credential metadata.

Trust mechanics:

- device enters `CLAIMED` only after HomeSignal finalization confirms the claim
- `device_id` becomes the durable product identity
- AWS IoT Thing name is created or bound using the same value as `device_id`
- claimed runtime MQTT client ID must use the same value as the AWS IoT Thing name and `device_id`
- AWS IoT credential metadata is recorded as replaceable transport credential material under `device_id`
- certificate fingerprint/serial/issuer are recorded for future `/agent/*` mTLS authorization
- account/site/device binding is stored in HomeSignal product state
- claim finalization is audited

Owning docs/services:

- Enrollment / Device Registry
- Audit
- Account / Site, for site binding context

### Claimed Runtime Operation

The device is trusted for allowed runtime behavior under its current site/account/device authority.

Trust mechanics:

- routine telemetry/events enter through the mTLS Agent HTTPS API in v0
- AWS IoT Core authenticates the connection and enforces that claimed-device runtime connections use the authorized Thing/client ID
- Telemetry Ingest stores runtime facts by the certificate-resolved durable `device_id`
- Telemetry Ingest does not resolve MQTT sessions into product identity during normal runtime ingestion
- approved `/agent/*` HTTPS calls authenticate with the claimed device certificate, then resolve exact certificate fingerprint/serial to `device_id`
- runtime writes require active claimed device and current authority
- payload device IDs are checked for drift but not trusted alone
- commands require cloud authorization plus local execution policy checks
- desired/reported edge state flows through Edge State Adapter and AWS IoT named shadows

Owning docs/services:

- Telemetry Ingest
- AWS IoT routing contract
- Command Service
- Edge State Adapter
- Local/cloud trust boundaries

### Credential Rotation Or Repair

The device may receive new transport credentials while remaining the same product device.

Trust mechanics:

- create a new `device_credentials` record
- preserve the same `device_id` when product identity continues
- preserve the same AWS IoT Thing name when product identity continues
- revoke or mark old credentials according to rotation policy
- do not break time-series or product history continuity

Owning docs/services:

- Enrollment / Device Registry
- AWS IoT provisioning/credential management
- Telemetry Ingest, for runtime identity resolution

### Release

Release removes or changes the device's current operational relationship without pretending historical facts never existed.

Trust mechanics:

- cloud authority changes immediately
- device is no longer authorized for normal claimed runtime operation under the released site/account
- AWS IoT certificate/policy binding is disabled, revoked, or otherwise made unusable for claimed runtime traffic according to the release/revoke flow
- local app cleanup removes cloud runtime credentials and claimed-device config when the release command converges locally
- historical product data remains attached to the prior `device_id`

Owning docs/services:

- Device Registry
- Command Service, for local convergence
- AWS IoT provisioning/credential management

### Re-Pairing And Repair

Re-pairing is a new enrollment attempt from an app that may or may not represent a previously claimed HomeSignal device.

Trust mechanics:

- an app must not self-assert an old `device_id` during re-pairing
- if local claimed config is intact, repair/credential rotation may preserve the same `device_id`
- if local claimed config is missing or wiped, the app behaves as an unclaimed installation
- reattaching to an existing `device_id` requires explicit cloud-authorized repair/reconnect behavior
- otherwise, successful enrollment creates a new `device_id`
- recognition signals may inform the repair/reconnect UX or support workflow, but they do not authorize identity continuity by themselves

Owning docs/services:

- Enrollment / Device Registry
- API Facade
- Audit

### Local Config Wipe

A local config wipe means the app no longer has its cached claimed-device configuration or credential bundle.

Trust mechanics:

- cloud does not treat a config wipe as release by itself
- the prior cloud device remains in its last known cloud lifecycle state until release, revoke, repair, or timeout/remediation policy changes it
- the old device may appear offline or unhealthy
- the local app returns to unclaimed behavior and must enroll again
- local files or user-entered values cannot restore claimed state without cloud authorization
- a local Home Assistant administrator is allowed to intentionally reset HomeSignal identity and return the app to unclaimed behavior
- local reset does not delete, transfer, or mutate prior cloud account records by itself

Owning docs/services:

- app implementation
- Enrollment / Device Registry
- Device Health

### Recognition Signals

Recognition signals are non-authoritative hints collected during enrollment or repair to help identify whether an app resembles a prior installation.

Initial candidate signals:

- app `installation_id`, if present
- Home Assistant instance ID, if available
- Supervisor or host installation/machine identifier, if available and appropriate
- hostname
- Home Assistant version
- Supervisor version
- app version
- operating system or environment type
- CSR hash for the current enrollment attempt

Trust mechanics:

- recognition signals are advisory only
- recognition signals may support repair prompts, fraud/risk checks, support workflows, or audit context
- recognition signals must not silently attach an app to an existing `device_id`
- browser location is not used as a recognition signal

### Claim Context Resolution

Claim context resolution starts in the web/SaaS portal when an authorized user creates a site-bound claim invite, then continues locally when the app verifies the invite code and presents the attached integrator/site/customer context for confirmation.

Trust mechanics:

- the web UI creates the claim invite because it has the authenticated user, account, site, customer, and permission context
- the app supplies the claim invite code, local installation data, recognition signals, and CSR hash for verification
- the local app UI owns the final human confirmation after showing the invite's integrator, creator, site, and customer details
- same-account or same-site matches may offer repair/reconnect of the existing HomeSignal device
- different-account or different-integrator matches must not expose or mutate the prior account's records unless the logged-in user has authority over that account
- a logged-in user without authority over the prior account may make a fresh claim for the local installation when local admin control has reset or re-paired the app
- a fresh claim creates a new `device_id` and new AWS IoT credentials under the claiming account/site
- the old cloud record remains protected by its original account authority and should appear disconnected when its old credentials stop reporting
- do not use `stale`, `superseded`, or `conflicted` as fresh-claim lifecycle end states
- the old account/site must not be told that the local installation was claimed elsewhere
- a fresh claim does not migrate history from the old `device_id` to the new `device_id`
- if the same account later reconnects/repairs the original `device_id`, historical continuity for that `device_id` resumes
- history remains under the account/site authority that owned the device record
  when the history was produced; a different account/integrator does not gain
  old history through a fresh claim
- a future explicit history-transfer/copy feature may be designed later, but it
  is not a v0 product feature and must not be inferred from a fresh claim
- HomeSignal should avoid two active runtime-authorized connections for the same recognized Home Assistant installation when conflict can be detected, but uniqueness enforcement must not grant cross-account mutation authority
- user-facing copy must not identify another account/integrator unless policy permits it
- the local verification response may identify only the invite's own account/site/customer/creator context and must not reveal unrelated prior-account recognition matches

Owning docs/services:

- Enrollment / Device Registry
- API Facade
- Web UI
- Authorization
- Audit

### Transfer

Transfer changes who has authority over the device/site relationship while preserving continuity where the product model requires it.

Trust mechanics:

- transfer is a HomeSignal product-state transition, not an AWS IoT metadata edit
- account/site relationship changes are explicit and audited
- product history remains attached according to site/device ownership rules
- transport credentials may rotate if risk or policy requires it, but rotation does not by itself create a new product device

Owning docs/services:

- Account / Site
- Authorization
- Enrollment / Device Registry
- Audit

### Revocation Or Removal

Revocation ends trust in the current credential/device relationship. Removal/archive controls product visibility and retention.

Trust mechanics:

- revoked credentials cannot authorize runtime product writes
- offline devices lose cloud authority immediately even if local cleanup waits
- product telemetry history and cold archives are deleted after a 7-day
  operational grace period when the device is deleted
- audit, security, billing, and authority records remain governed separately
  with minimized payloads
- removal/archive must not silently erase security history

Owning docs/services:

- Enrollment / Device Registry
- Authorization, where user access changes
- Audit
- Account / Site

## Variance Model

Can change without becoming a new product device, when an owning service explicitly performs the transition:

- AWS IoT certificate
- AWS IoT Thing name, if rotation/migration requires it
- principal identifier
- MQTT client session
- local software version
- reported health/status
- site manager/support relationship
- owner relationship through an approved transfer
- publish policy version
- desired/reported edge projection

Must not change silently:

- `device_id`
- installation/site binding after claim
- account/site authority
- claim state
- credential status
- product history ownership
- local private key custody rules

## Required Local Plan Checks

Every affected local plan should state:

- which lifecycle state or transition it touches
- which actor initiates the flow
- which service owns the state transition
- which durable identity is used
- which transport credential/session is involved
- which token/code/certificate is used
- expiry and replay behavior
- whether topic/payload IDs are claims, checks, or ignored
- how account/site/device authority is resolved
- audit events
- local app state changes
- AWS IoT side effects
- finalization and conflict behavior
- rollback/remediation path for partial success
- whether history continuity is preserved or intentionally broken

## V0 Decisions (Closed)

- V0 resolution for expired/conflicted claim invites and verifications is explicit in `enrollment-claiming-contract.md`:
  - Expired claim invites fail with `CLAIM_INVITE_EXPIRED` and require the integrator to create a fresh invite.
  - Expired claim verifications fail with `CLAIM_VERIFICATION_EXPIRED` and require the app to verify the invite again if the invite is still open.
  - Conflicted matches present only policy-permitted actions; same-account/site may include `repair_existing_device`, cross-account matches expose only `fresh_claim` when local reset/re-pairing is present.
  - No cross-account mutation is allowed.
- V0 post-claim direct app API exception: none. `POST /api/v1` surface is limited to enrollment/bootstrap. All additional post-claim app traffic uses approved `/agent/*` HTTPS flows.

## Acceptance Criteria

- Local and cloud state machines agree on lifecycle state names and transitions.
- Device identity, AWS IoT Thing name, and claimed MQTT client ID are the same durable identifier.
- No claimed state is inferred from local cert files alone.
- Runtime services use AWS IoT-authenticated, rule-enriched durable `device_id` before product writes.
- Topic and payload device IDs are not used as sole authority.
- Partial provisioning failures have a documented remediation path before production use.
- Claimed-device runtime control and compact state use AWS IoT. Approved `/agent/*` HTTPS flows may be used for artifact negotiation, command detail retrieval, command results, and other explicitly documented device API flows.
- Release, transfer, revocation, and removal preserve audit/history rules deliberately.
