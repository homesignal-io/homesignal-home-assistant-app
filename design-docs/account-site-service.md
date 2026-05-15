# Account / Site Service Spec

The Account / Site Service owns HomeSignal's durable business containers: accounts, customer records, sites, buildings, zones, site relationships, and site lifecycle. It is a logical service boundary. In v0 it should live inside the control-plane monolith with the API Facade and other domain services. Telemetry Ingest is the separate v0 deployable service.

This service owns business structure. Authorization owns membership and permission evaluation. Enrollment / Device Registry owns device identity and claim lifecycle.

## Agent Use

Read this before adding or changing account, customer, site, building, zone, site relationship, site lifecycle, or device placement behavior.

Also read:

- `api-facade.md` for route shape and API behavior
- `auth-service.md` and `workstreams/identity-and-authorization.md` for membership, roles, permissions, and authorization
- `enrollment-claiming-contract.md` for device claim and registry ownership
- `platform-doctrine.md` before changing service boundaries

Do not put membership, role evaluation, device identity, or topology artifact behavior into Account / Site.

## Owns

Account / Site owns:

- account records and account status
- account type/classification
- customer records
- site records and site status
- site service address fields entered manually
- buildings
- zones
- site relationships between accounts and sites
- site archive/deactivation lifecycle
- customer record archive/deactivation lifecycle
- default building/default zone seeding
- device placement rules within a site hierarchy
- normal site creation workflow

## Does Not Own

Account / Site does not own:

- human authentication
- users
- account memberships
- membership invitations
- roles
- permissions
- access grants
- authorization decisions
- device identity
- device claim lifecycle
- AWS IoT credential metadata
- telemetry ingest
- edge state projections, desired/reported state, or future custom twin behavior
- topology snapshots or topology artifacts
- command lifecycle
- billing/subscription state in v0
- installer/technician/staff assignment workflows
- self-managed customer onboarding in v0

## Core Model

`account` is the business and authority container.

`site` is the durable installation/data/history container.

`customer_record` is an account-scoped business/contact record. It is not a login account, user, or account membership.

Every normal site has exactly one active owner account. Ownerless sites are remediation/admin-only states, not normal product states.

Dealer, integrator, and support access is modeled through explicit site relationships. Changing a manager or support provider is a relationship change, not a data transfer.

## Service Boundary With Authorization

Authorization Service owns:

- users
- account memberships
- membership invitations
- roles
- permissions
- access grants
- permission evaluation

Account / Site owns:

- accounts
- customer records
- sites
- buildings
- zones
- site relationships

Authorization consumes Account / Site facts such as account status, site status, and active site relationships when evaluating permissions.

Account type does not grant permissions by itself. Account type may drive defaults, UX, onboarding, and validation, but Authorization roles/grants decide what a subject can do.

## Service Boundary With Enrollment / Device Registry

Enrollment / Device Registry owns:

- claim invites
- claim verifications
- claim invite display snapshots generated from Account / Site canonical facts
- durable HomeSignal `device_id`
- device claim state
- AWS IoT credential metadata
- release/transfer/rotation lifecycle when designed

Account / Site owns:

- canonical account/integrator display name and support contact
- canonical customer contact display name/email
- canonical site display name and service address
- where devices belong in the site/building/zone hierarchy
- placement rules and same-site zone moves

Device placement workflows coordinate with Enrollment / Device Registry. Cross-site device moves/reassignment are deferred until release/transfer semantics are designed.

## Service Boundary With Topology And Artifacts

Account / Site owns durable hierarchy containers:

```text
site
  building
    zone
```

It does not own topology snapshots, topology uploads, diagnostics bundles, backup artifacts, or raw S3 object state. Future object-transfer capability belongs behind `artifact-upload-broker.md`. Future queryable topology product state should be owned by a dedicated topology/domain service unless a later architecture decision explicitly creates a broader HomeSignal device-state service.

No topology upload feature exists in v0.

## v0 Product Stance

v0 is dealer/integrator-led.

Normal v0 sites are dealer/integrator-owned with lightweight customer records. The data model may support customer-owned sites, but self-managed customer account onboarding is out of scope.

Customers do not need login accounts in v0. Customer involvement is represented by account-scoped customer records, not user membership.

## Account Types

Account / Site owns lightweight account classification:

- dealer
- customer
- internal
- support_provider

Internal/admin-owned sites are exceptional remediation states and require admin-only authorization and audit.

Account type is not an authorization grant.

## Site Relationships

Supported v0 relationship types:

- owner
- manager
- support_provider

Rules:

- each normal site has exactly one active owner relationship
- multiple active manager relationships are allowed
- multiple active support_provider relationships are allowed
- ended relationships remove live relationship-derived access immediately
- site relationships are not general-purpose sharing
- neighbor/caregiver/view-only external sharing is out of scope for v0

Rich support workflows are deferred even though `support_provider` is modeled.

## Roles And Permissions

Roles are configurable permission bundles seeded with system defaults. They are not hard-coded in Account / Site.

Account / Site may know that an account has a relationship to a site. It must not decide whether a user can perform an action. Route handlers and workflows must use Authorization Service for permission decisions.

## Customer Records

Customer records are account-scoped.

A customer record may be associated with multiple sites within the same account.

v0 customer record fields should include:

- typed customer ID
- owning account ID
- display name
- email, optional
- phone, optional
- notes, optional
- status
- created_at
- updated_at
- archived_at

No browser geolocation is stored in v0.

Customer records may be archived/deactivated independently from site archive.

## Sites

Normal site creation requires an owner account.

A site may exist without a customer record only for explicit remediation, import, testing, or admin workflows. Normal portal flow should create or attach a customer record.

Site fields should include:

- typed site ID
- name/display name
- optional `site_category` for presentation only: `residential`, `business`, or
  `other`; if absent, UI uses the default Home Assistant/site icon
- owner relationship
- customer_record_id, optional
- manually entered service address fields, optional
- status
- created_at
- updated_at
- archived_at

Site lifecycle is distinct from device operational status.

`site_category` must not drive authorization, billing, lifecycle, or device
placement behavior in v0. It exists only so the portal can show a compact inline
icon before a site name when the category is known.

## Buildings And Zones

Buildings and zones are first-class resources in the model.

Every normal site creation seeds:

- one default building
- one default zone

Devices attach to zones. The UI may collapse simple sites into a Site -> Device view, but the underlying model remains:

```text
site
  building
    zone
      device placement
```

v0 API/UI can keep building and zone management minimal.

## Normal Site Creation

Normal v0 site creation is a single Account / Site domain transaction:

```text
create or attach customer record
create site
create active owner relationship
seed default building
seed default zone
```

Provisioning remains a separate Enrollment workflow. Site creation does not automatically create a claim invite unless an API/read-model workflow explicitly composes those steps later.

Dealer/integrator users may create and manage sites without customer login involvement in v0.

## Archive And Deactivation

Account / Site owns:

- account archive/deactivation
- site archive/deactivation
- customer record archive/deactivation

v0 uses archive/deactivate semantics for accounts, sites, and customer records. No hard delete for accounts or sites in v0.

Customer record hard delete is a future/admin edge case only when no linked site/history exists.

Site archive must coordinate with affected services and be authorized/audited. Archive is not a device release, command cancel, or alert resolution by itself.

## Route Families

The Account / Site spec defines route families only. Full public route contracts belong in OpenAPI under the API Facade spec.

Expected public route families:

- accounts
- account-scoped customer records
- sites
- site relationships
- buildings
- zones
- site overview/read models

Membership, invitations, roles, and grants are Authorization route families, not Account / Site route families.

## Audit Events

Domain-owned audit-worthy actions include:

- account creation
- account archive/deactivation
- customer record creation/update/archive
- site creation
- site archive/deactivation
- owner relationship creation/failure
- manager relationship add/end
- support_provider relationship add/end
- building/zone structural changes
- device placement changes when implemented
- admin/remediation owner assignment

Account / Site emits domain audit events through Audit Service. Operational API request logs are not audit.

## Observability

Account / Site should expose domain-level counters and logs for:

- site creation success/failure
- relationship invariant violations
- archive/deactivation actions
- default hierarchy seeding failures
- same-site device placement changes

Logs must include request/correlation context from the API Facade where available and must not include secrets.

## Out Of Scope v0

- billing/subscription state
- self-managed customer account onboarding
- customer login portal
- dealer-to-customer ownership transfer
- full site ownership transfer workflow
- dealer-to-dealer manager replacement except admin/manual correction
- global customer identity graph
- browser geolocation
- generic external sharing
- custom role editor UI
- staff/installer/technician assignment
- scheduling, dispatch, territory, or operations workflow
- topology snapshots/uploads
- cross-site device moves/reassignment
- hard delete for accounts/sites

Deferred features should be enumerated in implementation plans when relevant. Do not implement them as side effects of v0 site/account work.

## Acceptance Criteria

- Account / Site does not own users, memberships, roles, permissions, or grants.
- Normal site creation creates or attaches customer record, creates site, creates owner relationship, and seeds default building/zone.
- Exactly one active owner relationship per normal site is enforced as a hard invariant.
- Customer records are account-scoped and are not login accounts.
- No browser geolocation fields are introduced in v0.
- Buildings and zones are first-class model resources, even when hidden in the UI.
- Devices attach to zones; device identity remains owned by Enrollment / Device Registry.
- Site archive/deactivation is distinct from device release/revocation.
- Provisioning remains a separate Enrollment workflow.
- Route families are represented through API Facade/OpenAPI before public implementation.
