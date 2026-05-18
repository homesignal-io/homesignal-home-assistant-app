# Enrollment And Claiming Contract

This document defines the cloud contract expected by the HomeSignal Manager app enrollment implementation. It is not a SaaS implementation plan for this repository. The current repository implements the Home Assistant app side and mock-tested client interfaces only.

Canonical device lifecycle, trust, and authority rules live in `workstreams/device-lifecycle.md`. This contract defines the enrollment/claiming phase of that lifecycle.

## Scope

Feature 2 establishes device enrollment and durable device identity bootstrap. It does not implement MQTT runtime messaging, telemetry, topology discovery, backups, command lifecycle, update orchestration, release execution, or multi-region disaster recovery.

The HomeSignal backend is the source of truth for account, site, user, claim invite, claim verification, claim status, audit, and release/revocation state. HomeSignal issues the durable product `device_id` during claim finalization. For claimed devices, AWS IoT Thing name and required MQTT client ID use that same `device_id`; AWS IoT Core issues and enforces the replaceable transport credentials after HomeSignal authorizes the claim.

## Actors

- HomeSignal Manager app: installed on a Home Assistant site. It has a stable `installation_id` before claim.
- HomeSignal SaaS user: authenticated integrator user claiming a device to an account/site.
- HomeSignal backend: business authority for claim authorization and audit.
- AWS IoT Core: replaceable device credential signer/issuer and device transport endpoint.
- AWS IoT provisioning adapter: HomeSignal infrastructure boundary that coordinates Thing creation/binding, IoT policy attachment, CSR signing, and AWS certificate references after HomeSignal authorizes the claim.

## App Local State

`/config/device.json` is metadata only:

```json
{
  "schema_version": 2,
  "installation_id": "3ef0b5a2-f7d4-45b0-9c43-7fa9dce51a29",
  "created_at": "2026-05-11T12:00:00Z",
  "claim_state": "PAIRING_PENDING",
  "device_id": "",
  "iot_thing_name": "",
  "certificate_path": "/config/iot/device.crt",
  "private_key_path": "/config/iot/device.key",
  "claim_invite_expires_at": "2026-05-14T12:00:00Z",
  "claim_verification_id": "cv_123",
  "claim_verification_token_path": "/config/secrets/claim_verification_token",
  "claim_verification_token_expiry": "2026-05-11T12:15:00Z"
}
```

Secret material lives in separate `0600` files under app-owned `/config` subdirectories:

- `/config/secrets/claim_verification_token`
- `/config/iot/device.key`
- `/config/iot/device.crt`

Temporary claim verification material must be removed after final provisioning succeeds or when it expires. The durable device private key is generated locally and stored under `/config/iot`; HomeSignal must not receive or store it.

## State Machine

Allowed app states:

- `UNCLAIMED`: no durable HomeSignal-confirmed claim exists.
- `PAIRING_PENDING`: a claim invite has been verified locally, claim confirmation is in progress, or AWS provisioning succeeded but HomeSignal has not confirmed finalization.
- `CLAIMED`: HomeSignal confirmed the device, and local cert/key files exist.
- `REVOKED`: HomeSignal reported the credential/device as revoked. Full release cleanup is future work.

The app must never boot as `CLAIMED` merely because local cert/key files exist. HomeSignal-confirmed claimed metadata is required.

## Cloud-Side Data Models

### `device_claim_invites`

Created by an authenticated SaaS user from the web portal for a selected
account/site/customer context. The raw claim code is a high-entropy GUID-style
clean code returned only at creation time and, when requested, delivered by
HomeSignal email.

Required fields:

- `id`
- `account_id`
- `site_id`
- `customer_contact_id`
- `created_by_user_id`
- `created_by_membership_id`
- `recipient_email_normalized`
- `recipient_email_hash`
- `claim_code_hash`
- `claim_code_fingerprint`
- `display_snapshot_json`
- `display_snapshot_hash`
- `status`: `OPEN`, `USED`, `EXPIRED`, `CANCELLED`, `FAILED`
- `intended_action`: `fresh_claim`, `repair_existing_device`, or `reconnect_existing_device`
- `email_delivery_status`: `NOT_REQUESTED`, `QUEUED`, `SENT`, `FAILED`
- `email_last_attempt_at`
- `email_last_provider_message_id`
- `email_failure_reason`
- `verification_attempt_count`
- `confirmed_verification_id`
- `used_by_device_id`
- `expires_at`
- `used_at`
- `cancelled_at`
- `cancelled_by_user_id`
- `replaced_by_claim_invite_id`
- `created_at`
- `updated_at`

Rules:

- Default expiry: 72 hours.
- One-time use.
- Claim code is stored hashed server-side and must not be logged.
- Scoped to the creating user's account/site authorization at creation time.
- Creation user's account/site authorization must still be valid at claim
  confirmation time, otherwise the invite fails closed.
- Site/customer display details come from canonical account/site/customer
  records, not arbitrary invite free text.
- The invite stores a bounded display snapshot generated from canonical records
  at creation. Verification returns this snapshot so emailed/customer-facing
  details cannot be changed behind the user's back.
- Does not contain durable device credentials.
- Cancelling, expiring, or using an invite prevents future confirmations.
- The raw claim code is never recoverable after the create response or initial
  email job. If the code is lost, create a replacement invite and cancel the
  old one.

### `device_claim_verifications`

Created when the app presents a claim invite code to verify before local
confirmation. Verification is a context/read step. It must not issue runtime
credentials or finalize the claim.

Required fields:

- `id`
- `claim_invite_id`
- `installation_id`
- `agent_version`
- `recognition_signals`
- `csr_sha256`
- `verification_token_hash`
- `presented_details_hash`
- `source_ip_hash`
- `user_agent_hash`
- `confirmed_device_id`
- `failure_reason`
- `status`: `PRESENTED`, `CONFIRMED`, `AWS_PROVISIONING_ISSUED`, `CLAIMED`, `EXPIRED`, `REVOKED`, `FAILED`
- `expires_at`
- `confirmed_at`
- `created_at`
- `updated_at`

Rules:

- Claim verification token default expiry: 15 minutes.
- Verification token is stored hashed server-side and returned only to the
  app that requested verification.
- Confirmation requires the same verification record, token, installation ID,
  claim invite, and presented details hash.
- The raw claim code alone cannot confirm or finalize a claim.
- The claim invite remains single-use; first valid confirmation wins.
- Creating a new verification for the same invite and installation invalidates
  any older unconfirmed verification for that pair.
- At most three active, unconfirmed verifications may exist for one invite
  across different installations before support/new-invite intervention is
  required.
- Recognition signals are advisory and cannot authorize attaching to a prior
  `device_id` by themselves.

### `device_claim_invite_email_deliveries`

Tracks delivery attempts without storing the raw claim code after the initial
email job has completed.

Required fields:

- `id`
- `claim_invite_id`
- `recipient_email_hash`
- `provider`
- `provider_message_id`
- `status`: `QUEUED`, `SENT`, `FAILED`, `BOUNCED`, `SUPPRESSED`
- `attempt_number`
- `failure_reason`
- `created_at`
- `sent_at`
- `failed_at`

Rules:

- A delivery record may include the raw claim code only in an in-memory or
  transient provider handoff. The database must not persist the raw claim code.
- "Resend" is modeled as creating a replacement invite with a new raw code, not
  as recovering the prior code.
- Provider failures do not grant extra invite creation budget. The portal can
  show the one-time code from the successful create response or let the user
  cancel/replace.

### `devices`

Created after SaaS authorization and finalized after AWS IoT provisioning succeeds.

Required fields:

- `device_id`
- `account_id`
- `site_id`
- `installation_id`
- `claim_state`: `CLAIMED`, `REVOKED`
- `iot_thing_name`
- `claimed_at`
- `revoked_at`
- `created_at`

Rules:

- `installation_id` cannot be silently rebound to another site.
- A claimed HomeSignal installation may be re-entered only through an authenticated web claim flow as `repair`, `reconnect`, or `fresh_claim`; silent re-claim is not allowed.
- `device_id` is the durable HomeSignal product identity.
- `iot_thing_name` must equal `device_id` for claimed devices.
- AWS IoT certificate, principal, and connection session identifiers are not product identity.

### `device_credentials`

Tracks AWS IoT credential metadata, not private key contents.

Credential creation uses certificate signing, not HomeSignal key custody:

- The app generates the private key locally.
- The app sends a CSR through the claim flow.
- HomeSignal coordinates AWS IoT `CreateCertificateFromCsr`.
- AWS IoT signs the CSR and returns `certificatePem`, `certificateId`, and `certificateArn`.
- HomeSignal returns the certificate PEM to the app and stores the AWS certificate identifiers plus derived certificate metadata.
- HomeSignal never receives or stores the device private key.
- HomeSignal does not need to persist the full certificate PEM after claim; it stores computed identity fields for future `/agent/*` mTLS authorization.

Required fields:

- `id`
- `device_id`
- `aws_region`
- `iot_endpoint`
- `iot_thing_name`
- `certificate_id`
- `certificate_arn`
- `certificate_fingerprint`
- `certificate_serial`
- `certificate_issuer`
- `principal_identifier`
- `status`: `ACTIVE`, `REVOKED`, `ORPHANED`
- `issued_at`
- `revoked_at`
- `created_at`

Rules:

- AWS IoT Thing name equals the durable HomeSignal `device_id`.
- AWS IoT certificate and principal identity are replaceable transport credentials.
- Rotating or re-pairing AWS IoT credentials should create a new credential record under the same HomeSignal `device_id` when product identity should be preserved.
- Runtime lifecycle/session identifiers are connection provenance and must not replace `device_id`.

## Config Wipe And Re-Pairing

If the user wipes app config, the app loses its cached claimed-device metadata and credential bundle.

Rules:

- The app must return to `UNCLAIMED` behavior locally.
- Cloud device state does not change to released merely because local config was wiped.
- The prior cloud device may appear offline or unhealthy until repaired, released, revoked, or otherwise remediated.
- The app must enroll again before claimed runtime operation.
- Reusing a prior `device_id` requires explicit cloud-authorized repair/reconnect behavior.
- Without explicit repair/reconnect, successful enrollment creates a new `device_id`.
- A local Home Assistant administrator may intentionally reset HomeSignal local identity; this returns the app to unclaimed behavior but does not delete, transfer, or mutate prior cloud account records by itself.

## Recognition Signals

Enrollment may include bounded recognition signals so HomeSignal can identify possible continuity with a prior installation.

Initial candidate signals:

- app `installation_id`, if present
- Home Assistant instance ID, if available
- Supervisor or host installation/machine identifier, if available and appropriate
- hostname
- Home Assistant version
- Supervisor version
- app version
- operating system or environment type
- CSR hash

Rules:

- Recognition signals are advisory only.
- Recognition signals may support repair prompts, support workflows, risk checks, and audit context.
- Recognition signals must not silently authorize attachment to an existing `device_id`.
- Browser location is not collected for this purpose.

## Claim Context Resolution

The app does not decide whether a recognized prior installation should repair/reconnect to an existing HomeSignal identity, transfer through an authorized product flow, or create a fresh claim. That decision happens in the authenticated web claim flow.

Flow:

```text
SaaS user creates a site claim invite in the web portal
  -> API stores hashed GUID-style claim code with account/site/customer/creator context
  -> portal shows or emails the raw code once

local app user enters claim code
  -> app sends installation_id, ha_instance_uuid, machine_id, recognition_signals, agent_version, CSR hash
  -> API validates invite, expiry, rate limits, and recognition context
  -> API returns integrator/site/customer/creator display details and a short verification token
  -> local app UI presents details for confirmation
  -> app confirms with the verification token and presented details hash
  -> API finalizes the authorized action and returns claim credentials
```

Rules:

- Same-account or same-site matches may offer repair/reconnect of the existing HomeSignal device when the logged-in user has authority.
- Different-account or different-integrator matches must not expose or mutate prior account records unless the logged-in user has authority over that account.
- A user without authority over the prior account may make a fresh claim for the local installation after local reset/re-pairing.
- A fresh claim creates a new `device_id` and new AWS IoT credentials under the claiming account/site.
- The old cloud record remains protected by its original account authority and should appear disconnected when its old credentials stop reporting.
- Do not use `stale`, `superseded`, or `conflicted` as fresh-claim lifecycle end states.
- HomeSignal should avoid two active runtime-authorized connections for the same recognized Home Assistant installation when conflict can be detected, but conflict handling must not grant cross-account mutation authority.
- User-facing copy must not identify another account/integrator unless policy permits it.
- Claim invite verification may identify only the account/site/customer/creator
  attached to the invite itself. It must not reveal unrelated prior-account
  recognition matches to the local app UI.

### `audit_events`

Every claim attempt and outcome must be recorded.

Required fields:

- `id`
- `account_id`
- `site_id`
- `device_id`, nullable until a device exists
- `installation_id`, nullable until app verification
- `claim_invite_id`, when applicable
- `claim_verification_id`, when applicable
- `recipient_email_hash`, when applicable
- `actor_user_id`, nullable for unauthenticated app verification attempts
- `actor_kind`: `user`, `app_preclaim`, `system`
- `action`: `claim_invite_created`, `claim_invite_email_send_attempted`, `claim_invite_cancelled`, `claim_invite_expired`, `device_claim_verification_attempted`, `device_claim_verification_succeeded`, `device_claim_verification_failed`, `device_claim_confirmation_attempted`, `device_claim_succeeded`, `device_claim_failed`, `aws_pre_provisioning_allowed`, `aws_pre_provisioning_denied`, `claim_finalization_conflict`
- `result`: `success`, `failure`, `denied`
- `reason`
- `source_ip`
- `created_at`

## Required API Pattern

### SaaS user creates a claim invite

`POST /api/v1/sites/{site_id}/device-claim-invites`

Headers:

```text
Authorization: Bearer <Cognito JWT>
Content-Type: application/json
Idempotency-Key: <client-generated-key>
```

Request:

```json
{
  "customer_contact_id": "contact_123",
  "recipient_email": "alex@example.com",
  "expires_in_seconds": 259200,
  "delivery": {
    "send_email": true
  },
  "intended_action": "fresh_claim"
}
```

`expires_in_seconds` defaults to 72 hours and must not exceed the configured
maximum. `intended_action` defaults to `fresh_claim`.

Creation uses idempotency because retries must not create duplicate codes or
send duplicate email. Immediate idempotent retries may return the original raw
`claim_code` from the API idempotency cache. If the idempotency cache is gone
and an open invite already exists for the same site/recipient, return
`CLAIM_INVITE_ALREADY_OPEN` with the invite ID/status but no raw code; the
portal can cancel/replace it.

Response:

```json
{
  "claim_invite_id": "ci_123",
  "claim_code": "4f8b0e7a-0f7d-45f8-8b8b-1e25f4d68a10",
  "site_id": "site_123",
  "account_id": "acct_123",
  "status": "OPEN",
  "expires_at": "2026-05-17T12:00:00Z"
}
```

The raw `claim_code` is shown or emailed once. It is a high-entropy GUID-style
clean code, not a six-digit local pairing code. HomeSignal stores only a hash
and a bounded fingerprint for support/audit correlation.

Portal management routes:

```text
GET /api/v1/sites/{site_id}/device-claim-invites
GET /api/v1/sites/{site_id}/device-claim-invites/{claim_invite_id}
POST /api/v1/sites/{site_id}/device-claim-invites/{claim_invite_id}/cancel
POST /api/v1/sites/{site_id}/device-claim-invites/{claim_invite_id}/replace
```

Portal routes are authenticated and authorized against the site/account. They
never return the raw claim code after creation. `replace` cancels the old invite
and creates a new invite/code under the normal creation limits.

Public/app error behavior:

| Situation | Public response |
| --- | --- |
| Malformed, unknown, expired, cancelled, used, or failed invite code | `CLAIM_INVITE_NOT_AVAILABLE` |
| Verification token missing, invalid, expired, or terminal | `CLAIM_VERIFICATION_NOT_AVAILABLE` |
| Presented details hash no longer matches verification | `CLAIM_DETAILS_STALE` |
| Active account/site/customer/creator authority no longer valid | `CLAIM_INVITE_NOT_AVAILABLE` |
| Rate limit exceeded | `HTTP 429` with `Retry-After` |
| Same invite confirmed first by another verification | `CLAIM_INVITE_NOT_AVAILABLE` |

Authenticated portal routes may show exact invite status to authorized users.
Unauthenticated app routes should prefer the generic unavailable class so
public callers cannot enumerate which codes ever existed.

### App verifies a claim invite

`POST /api/v1/device-enrollment/claim-invites/verify`

Headers:

```text
Content-Type: application/json
```

Request:

```json
{
  "claim_code": "4f8b0e7a-0f7d-45f8-8b8b-1e25f4d68a10",
  "installation_id": "3ef0b5a2-f7d4-45b0-9c43-7fa9dce51a29",
  "ha_instance_uuid": "ha_8f1db7b1d4fb4c12a2c4b0fcb4df8e5a",
  "machine_id": "2f8f8c8e2c7f4f89a2a1f5a9e6d3b111",
  "agent_version": "0.1.2",
  "recognition_signals": {
    "ha_instance_uuid": "ha_8f1db7b1d4fb4c12a2c4b0fcb4df8e5a",
    "machine_id": "2f8f8c8e2c7f4f89a2a1f5a9e6d3b111",
    "home_assistant_version": "2026.5.0",
    "supervisor_version": "2026.05.0",
    "ha_app_version": "0.1.2",
    "hostname": "homeassistant",
    "os_type": "Home Assistant OS"
  },
  "csr_sha256": "sha256:..."
}
```

`ha_instance_uuid` is required when the app can retrieve it. `machine_id` is required when Supervisor exposes it. The request must still be accepted without one of these values only when the app cannot obtain it; missing recognition fields reduce repair/reconnect confidence and may force normal first-time pairing.

Backend validation:

- Request body is strict JSON, length-bounded, and rejects unknown top-level
  fields.
- `claim_code` is normalized before hashing: trim surrounding whitespace,
  remove zero-width characters, lowercase UUID letters, and reject anything
  outside the supported GUID-style format.
- Claim code exists, is open, unexpired, and unused.
- Claim invite creator is still active and authorized for the account/site.
- Source IP, installation ID, claim code fingerprint, and account/site creation
  budgets are within rate limits.
- API evaluates recognition candidates and claim context before finalization.

Response:

```json
{
  "claim_verification_id": "cv_123",
  "verification_token": "opaque-high-entropy-token",
  "verification_token_expires_at": "2026-05-14T12:15:00Z",
  "claim_details_hash": "sha256:...",
  "claim_invite": {
    "expires_at": "2026-05-17T12:00:00Z",
    "intended_action": "fresh_claim"
  },
  "integrator": {
    "account_display_name": "Northstar Smart Homes",
    "support_email": "support@northstar.example"
  },
  "created_by": {
    "display_name": "Maya Patel",
    "email": "maya.patel@northstar.example"
  },
  "site": {
    "site_id": "site_123",
    "display_name": "Smith Residence",
    "service_address": {
      "line1": "12 Oak Street",
      "city": "Columbus",
      "region": "OH"
    }
  },
  "customer": {
    "display_name": "Alex Smith",
    "email": "alex@example.com"
  },
  "ui_copy_key": "confirm_integrator_site_claim"
}
```

Claim verification response rules:

- The response contains only invite-owned display context needed for the local
  user to decide whether to continue.
- Do not return internal notes, billing details, broad customer records, prior
  account matches, credentials, certificates, or secrets.
- `claim_details_hash` is computed over a canonical JSON form of
  `claim_invite`, `integrator`, `created_by`, `site`, `customer`, and
  `ui_copy_key`, plus `claim_invite_id` and `claim_verification_id`.
- The verification token is written to a local secret file and is never shown.
- Requesting verification must not issue runtime credentials or finalize the
  claim.

### App confirms a claim verification

`POST /api/v1/device-enrollment/claim-verifications/{claim_verification_id}/confirm`

Headers:

```text
Authorization: Bearer <verification_token>
Content-Type: application/json
Idempotency-Key: <client-generated-key>
```

Request:

```json
{
  "claim_details_hash": "sha256:...",
  "installation_id": "3ef0b5a2-f7d4-45b0-9c43-7fa9dce51a29",
  "csr_pem": "-----BEGIN CERTIFICATE REQUEST-----\n...\n-----END CERTIFICATE REQUEST-----\n",
  "local_management_policy": {
    "schema_version": 1,
    "policy_revision": 1,
    "authority_profile": "managed_admin",
    "source": "local_ha_app_pairing",
    "selected_at": "2026-05-14T12:00:00Z",
    "permissions": [
      { "key": "ha_status_read", "enabled": true },
      { "key": "ha_backup_trigger", "enabled": true }
    ]
  }
}
```

Backend validation:

- Request body is strict JSON, length-bounded, and rejects unknown top-level
  fields.
- Verification token hash matches the verification record and is unexpired.
- Verification record is bound to the same claim invite, installation ID, and
  presented details hash returned by verification.
- Claim invite is still open, unexpired, unused, and not cancelled.
- Claim invite creator is still active and authorized for the account/site.
- Account, site, customer contact, and creator membership are still active. If
  any are disabled, removed, archived, or suspended, confirmation fails closed.
- CSR hash matches the hash presented during verification.
- If the invite display snapshot hash no longer matches the verification's
  presented details hash, confirmation fails with `CLAIM_DETAILS_STALE` and the
  app must verify again.
- AWS region, IoT endpoint, fleet/provisioning template, and policy attachment
  are selected by backend configuration, not by app request fields.
- Optional `local_management_policy` is stored as claim metadata and initial
  local policy context. It does not authorize the claim by itself.
- Any recognized repair/reconnect/fresh-claim decision is allowed by account,
  site, user, and local reset context.
- Confirmation locks the invite row, atomically marks the invite `USED`, binds
  `confirmed_verification_id` and `used_by_device_id`, and creates the device
  and credential records. Any duplicate/racing confirmation fails safely or
  returns the idempotent prior response for the same idempotency scope.

Response:

```json
{
  "status": "claimed",
  "claim_verification_id": "cv_123",
  "claim_invite_id": "ci_123",
  "selected_action": "fresh_claim",
  "device_id": "dev_123",
  "iot_thing_name": "dev_123",
  "certificate_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
  "iot_endpoint": "a1234567890-ats.iot.us-east-1.amazonaws.com",
  "iot_region": "us-east-1",
  "claimed_at": "2026-05-14T12:02:00Z"
}
```

The app already generated and retained the private key locally before
submitting the CSR. The certificate PEM is the signed public certificate and may
be returned through the confirm response. HomeSignal must not return or store a
device private key.

### HomeSignal provisions with AWS IoT Core

The app submits a CSR through the HomeSignal claim verification confirmation
flow. After local confirmation and backend authorization, HomeSignal coordinates
AWS IoT `CreateCertificateFromCsr` through the provisioning adapter. AWS IoT
signs the CSR and returns `certificatePem`, `certificateId`, and
`certificateArn`.

Provisioning adapter validation must confirm:

- `claim_verification_id` exists.
- Claim verification is confirmed and unexpired.
- Claim invite is open, unexpired, unused, and bound to the verification.
- `installation_id` matches the confirmed claim verification.
- `device_id` matches the approved claim.
- The claim invite has not already been used by another confirmation.
- The CSR belongs to the current claim verification.
- The target AWS IoT region/account/template is selected by backend
  configuration for the environment/account/site and is expected.

If the provisioning adapter cannot validate the claim, it must deny certificate signing/provisioning. New claims may fail during provisioning/backend outages; already claimed devices are not affected by this enrollment path.

### Claim confirmation and local storage

`POST /api/v1/device-enrollment/claim-verifications/{claim_verification_id}/confirm`
is the HomeSignal claim finalization endpoint for this flow.

Confirmation must be idempotent by `Idempotency-Key`. If the app loses the
response before storing the certificate/config locally, it may retry the same
confirmation while the verification token remains valid. HomeSignal returns the
same successful response for the same idempotency scope.

The app enters `CLAIMED` only after it has persisted the returned
certificate/config and the confirm response has returned `status = claimed`. It
must not report `CLAIMED` or start future IoT runtime messaging before that
successful confirm response.

The backend must reconcile orphaned AWS certs/Things if provisioning succeeds
but final claim persistence never succeeds.

### Orphaned credential remediation (v0)

When a claim verification creates AWS material but HomeSignal cannot persist a matching owned-device/credential record, backend state is `PENDING_RECONCILIATION`:

- AWS artifact is treated as a temporary `ORPHANED` candidate.
- Backend keeps one reconciliation attempt path for the active claim verification; if the same confirmation retries and credentials are still present, HomeSignal reattaches the orphaned credential and completes claim.
- Reconciliation TTL is short (recommended 30 minutes).
- If TTL expires without successful reattachment, HomeSignal revokes certificate in AWS, marks local credential metadata `status = ORPHANED_CLEANED`, and requires a fresh claim.
- Internal telemetry records why the claim was orphaned and whether cleanup was automatic.
- Audit emits at least: `orphaned_credential_detected`, `orphaned_credential_reconciled`, `orphaned_credential_revoked`.

### Future app checks device status

`GET /api/v1/devices/{device_id}/status`

This endpoint is intentionally out of v0 scope and not part of the enrollment/bootstrap API boundary. In v0, post-claim device calls must use approved mTLS `/agent/*` routes. Any later reintroduction of a public app status or revocation polling endpoint requires explicit definition of compatibility, security, rate-limits, and degraded-mode behavior before implementation.

Response:

```json
{
  "status": "CLAIMED"
}
```

Allowed statuses:

- `CLAIMED`
- `REVOKED`
- `UNKNOWN`
- degraded/service-unavailable responses

If the backend reports `REVOKED`, the app enters safe `REVOKED` state. Full release cleanup is future work. If the backend is unavailable, the app keeps local operation safe and reports a degraded cloud status.

## Claim Invite Lifecycle And Garbage Collection

Claim invite cleanup must be explicit because these records touch public URLs,
email delivery, PII, and one-time bearer material.

Lifecycle transitions:

| From | To | Cause |
| --- | --- | --- |
| `OPEN` | `USED` | Successful claim verification confirmation. |
| `OPEN` | `EXPIRED` | `expires_at` passed before successful confirmation. |
| `OPEN` | `CANCELLED` | Authorized portal user cancels or replaces the invite. |
| `OPEN` | `FAILED` | Internal invariant/provisioning failure prevents this invite from completing safely. |

Garbage collection rules:

- A scheduled job marks expired `OPEN` invites as `EXPIRED` at least every 15
  minutes.
- Expiring or cancelling an invite expires all active verifications for that
  invite.
- Verification rows expire independently at `expires_at`; the default TTL is 15
  minutes.
- `verification_token_hash` is cleared on terminal verification states and no
  later than 24 hours after verification expiry.
- `claim_code_hash` is cleared 7 days after an invite reaches `USED`,
  `EXPIRED`, `CANCELLED`, or `FAILED`; `claim_code_fingerprint` may remain for
  support/audit correlation.
- `recipient_email_normalized` and `display_snapshot_json` are purged or
  minimized 30 days after unused terminal states. Used invite display details
  should move to device/audit records only where needed for product history.
- Rate-limit counters for invite creation, verification, and confirmation are
  retained only for their rolling window plus operational slack, recommended 48
  hours.
- Audit events remain under the audit retention policy and must not include raw
  claim codes or verification tokens.

Database constraints/indexes:

- Unique `claim_code_hash` while non-null.
- Partial unique open invite by `(site_id, recipient_email_hash)` where
  `status = 'OPEN'`.
- Index open invite expiry: `(status, expires_at)`.
- Index verification expiry: `(status, expires_at)`.
- Index verification lookup by `(claim_invite_id, installation_id, status)`.
- Confirm path must lock the invite row before checking and changing terminal
  state.

## Claim Invite Abuse Guardrails

Claim invite creation is authenticated portal behavior, but it can still be
misused for unwanted email or misleading pairing attempts. Guardrails are part
of the enrollment contract, not optional UI polish.

Initial configurable limits:

- Maximum 5 claim invite creations per creating user per rolling 24 hours.
- Maximum 10 claim invite creations per account per rolling 24 hours.
- Maximum 3 open, unexpired claim invites per site.
- Maximum 1 open invite per site/recipient email pair.
- Expired, cancelled, and failed invites still count against the rolling
  creation budget; successful use does not reset the budget.

Rate-limit posture:

- Treat invite creation as authenticated outbound-contact abuse prevention:
  protect recipients, site trust, email reputation, and customer confusion.
- Treat invite verification as public bearer-code attack prevention: protect
  against guessing, credential stuffing style replay, bot traffic, and targeted
  invite probing.
- Treat invite confirmation as authority-transition protection: protect the
  one-time claim mutation, provisioning adapter, and certificate issuance path.
- Use layered budgets rather than one global number. A request is allowed only
  when every applicable bucket has remaining capacity.
- Start with strict defaults, then tune by plan/account trust tier, verified
  email/domain reputation, fraud signals, and internal support overrides.
- Prefer short retry windows for honest mistakes and hard daily ceilings for
  outbound-contact abuse.
- Track claim conversion as a trust signal: successful claims divided by open
  claim invites, evaluated per creator, account, and site over a rolling window.
  A low successful-claim-to-open-claim ratio indicates invite spray, stale site
  workflow, bad recipient data, or confusing UX and should reduce creation
  headroom before it becomes an email or trust problem.

Suggested v0 buckets:

| Flow | Bucket | Starting posture |
| --- | --- | --- |
| Create invite | creator user | 5/day, burst 2/hour |
| Create invite | account | 10/day, burst 4/hour |
| Create invite | site | 3 open invites, 5/day |
| Create invite | recipient email hash | 1 open per site, 3/day globally |
| Create invite | recipient domain | monitor-only at first, block obvious disposable/abuse domains later |
| Create invite | successful claims / open invites | monitor initially; throttle or require review when sustained ratio is poor |
| Email send | account and provider result | pause sends on repeated bounces/suppression |
| Verify invite | source IP or network | low burst with progressive backoff |
| Verify invite | installation ID | low burst with progressive backoff |
| Verify invite | claim code fingerprint | very low failure budget; lock invite on repeated wrong-context attempts |
| Confirm verification | verification ID | very low retry budget, idempotent success replay only |
| Confirm verification | installation ID | low burst to protect provisioning/cert issuance |

Failure posture:

- Failed verification attempts should use progressive delay/backoff before a
  hard lockout. Honest users paste bad codes; bots repeat at scale.
- Repeated failures for one claim code fingerprint should make that invite
  require replacement or support review rather than letting it be probed until
  expiry.
- Repeated failures from one installation ID should temporarily pause
  verification for that installation without revealing whether any target code
  exists.
- Sustained poor claim conversion should not immediately block a legitimate
  integrator, but it should lower invite creation limits and surface a portal
  warning with cleanup actions for stale open invites.
- Rate-limit telemetry should be structured and alertable, but labels must use
  bounded fingerprints and typed IDs, never raw invite codes, email addresses,
  or verification tokens.

Additional rules:

- Only authenticated users with site/account claim authority may create invites.
  Portal list/create/cancel/replace use `device_claim_invite:*` permissions,
  and confirmation still evaluates `agent:claim`/device claim authority.
- Create/cancel/replace portal routes require authorization and CSRF protection
  when browser sessions use cookies.
- Email delivery must use HomeSignal-controlled sender identity and canonical
  account/site/customer display fields.
- Do not allow free-form sender names, arbitrary integrator names, or unbounded
  custom email body text in v0.
- Public verify/confirm routes must return `Cache-Control: no-store` and must
  not expose claim codes in URLs, redirects, referrers, logs, analytics events,
  or browser storage.
- The unauthenticated verify route returns invite display details only for a
  valid, open, unexpired claim code. Invalid, cancelled, used, malformed, and
  unknown codes return the same `CLAIM_INVITE_NOT_AVAILABLE` class response.
- Verification failures are rate-limited by source IP, installation ID, and
  claim code fingerprint.
- Confirmation is rate-limited by source IP, installation ID, and claim
  verification ID.
- Public invite rate limits may start in-process for a single-instance v0, but
  must use shared storage before horizontally scaled or production email-enabled
  deployment.
- Invalid, expired, cancelled, and already-used invite responses must be
  bounded and must not reveal unrelated account/site data.
- Audit records invite creation, email send attempt, verification attempt,
  verification success/failure, confirmation success/failure, cancellation, and
  expiry.

User experience requirements:

- The portal creation screen previews the exact integrator, creator, site,
  customer, recipient, expiry, and support contact details that the app will
  show during verification.
- The create-success screen makes clear that the raw code is shown once. After
  navigation/refresh, the user can cancel or replace the invite, not reveal the
  old code.
- The app verification screen shows the invite display details before the
  final confirm button. The confirm copy should tell the local admin to continue
  only if they expected that integrator/site/customer.
- Expired, cancelled, already-used, and malformed code states should be
  actionable but not revealing: ask the user to request a new invite from the
  integrator.

## Security Rules

- Claim invite codes are high-entropy bearer invites, not durable credentials.
- Claim invite codes default to 72-hour expiry and are single-use.
- Claim invite codes are at least 128 bits of randomness, rendered as a
  GUID-style clean code. Do not derive them from account, site, user, time, or
  sequence data.
- Claim code hashes use a server-side pepper/HMAC or equivalent keyed hash so a
  database leak does not enable offline invite-code verification.
- Claim invite codes, verification tokens, and code fingerprints must not be
  logged or exposed in metrics labels.
- Claim invite codes are submitted in JSON bodies, not URL paths or query
  strings.
- Claim verification tokens are high-entropy, short-lived, stored in secret
  files, and never shown in UI.
- Verification token hashes use keyed hashing equivalent to invite code hashes.
- The raw claim code alone cannot confirm a claim; confirmation requires a prior
  verification token and matching presented details hash.
- Claim/session material is short-lived and removed after use/expiry.
- Device private keys are generated locally and never leave the app.
- Real device certificate contents and private key contents are never exposed in `/status`, `/readyz`, `/ui`, browser storage, logs, or static docs.
- HomeSignal final confirmation is required before the app can consider itself `CLAIMED`.
- A claimed device cannot be silently claimed again; any re-entry must go through authenticated web-flow repair/reconnect/fresh-claim handling.
- First valid claim wins; races fail safely.
- Single primary AWS IoT region is used for now. Multi-region failover and load sharing are out of scope.

## App Acceptance Expectations

- Legacy `device.json` with only `installation_id` migrates to schema v2.
- Fresh install with reachable fake HomeSignal backend accepts a GUID-style claim invite code in `/ui`.
- Valid claim invite verification displays integrator, creator, site, and customer details before confirmation.
- Claim confirmation is impossible unless the app has first requested and accepted claim invite verification details.
- `/status` exposes state and non-secret metadata only.
- Pending claim verification survives restart until expiry.
- Expired claim invites or verification tokens are not reused.
- Fake successful SaaS claim plus fake AWS IoT CSR signing/provisioning plus fake HomeSignal finalization transitions to `CLAIMED`.
- Existing cert/key files without HomeSignal-confirmed metadata do not boot as `CLAIMED`.
- AWS-provisioned-but-not-finalized state remains degraded `PAIRING_PENDING`.
- Future backend status/revocation checks, if reintroduced, enter safe `REVOKED`.
