Auth Architecture v0 Spec

Goal

Define the v0 authentication and authorization architecture for the HomeSignal control plane.

This spec covers human/application authorization only. It intentionally does not define AWS IoT Core device authentication, MQTT topic policies, telemetry ingestion, or device certificate lifecycle.

Terminology

Account is the canonical HomeSignal authority container for customers, dealers, integrators, and support providers.

Earlier or generic architecture language may use organization, account, or customer inconsistently. In this auth architecture:

* account is the implementation term
* customer record is a lightweight real-world contact/site record
* customer login account is a future optional concept, not a v0 requirement
* organization should not be introduced as a separate v0 auth concept

Core Principles

* Cognito owns human authentication.
* The application owns authorization.
* Account is the business and authority container.
* Site is the durable data container.
* Customer records exist for sites; customer login accounts are out of scope for v0.
* Accounts relate to sites through explicit relationships.
* Users inherit permissions through account membership and active site relationships.
* Site relationships are core auth infrastructure, not a general sharing feature.
* Roles are configurable permission bundles, seeded with system defaults.
* V0/V1 does not include customer-defined role editing in the UI.
* Permissions are additive.
* No explicit deny rules in v0.
* No user impersonation.
* No hidden vendor god-mode in the cloud.
* Remote/host control requires cloud permission, site policy, local app policy, and app/device approval.
* API keys later authenticate service accounts, not themselves.
* Buildings and zones are first-class resources and always seeded.
* Orphaned states are remediation/error states, not normal product states.
* Auth-sensitive actions must be audited.

Authentication Boundary

Human authentication is delegated to Cognito.

Cognito owns:

* signup
* login
* password reset
* email verification
* MFA/passkeys later
* JWT issuance
* refresh/session mechanics

The application receives a Cognito JWT and maps it to a local user.

Cognito JWT
  -> cognito_sub
  -> users.cognito_sub
  -> local user subject

The app must not hand-roll passwords, password reset, MFA, or token crypto.

Token acceptance still belongs to the application boundary. The API must verify:

* Cognito JWT signature against the expected JWKS
* issuer
* audience/client id
* token use
* expiration
* subject mapping to users.cognito_sub
* local user status

A valid Cognito token does not grant access for a disabled, removed, or deleted local user.

Authorization Boundary

Authorization is local and Postgres-backed.

Postgres stores:

* users
* accounts
* account memberships
* customer records
* sites
* resources
* site relationships
* roles
* role permissions
* access grants
* audit events

Application code owns the permission evaluator.

All routes must authorize through a central interface:

AuthorizationService.can(subject, action, resource, context)

Route handlers must not implement ad hoc role checks.

Subject Model

A subject is an actor that can receive permissions.

Supported subject types:

user
account
service_account

Future API keys authenticate service accounts.

Do not grant permissions directly to raw API keys.

api_key -> authenticates -> service_account subject

Account, Customer, And Site Model

Account / Site Service owns account records, customer records, sites, buildings, zones, and site relationships. Authorization owns users, account memberships, roles, permissions, access grants, and permission evaluation.

Site is the durable data container.

Data belongs to the site, not directly to a dealer, integrator, customer, or support provider.

Every site has exactly one active owner account.

The owner account may be:

* dealer/integrator account
* customer account
* internal/admin account during remediation only

Normal portal flow should create or attach a lightweight customer record. A site may exist without a customer record only for remediation, import, testing, or admin workflows. The customer record is not the same thing as a login account.

customer_records

id
display_name
email nullable
phone nullable
service_address nullable
notes nullable
created_at
updated_at
archived_at

Browser geolocation is out of scope for v0 and must not be treated as site authority.

A dealer-created site may be dealer-owned while still having a customer record. A customer-owned site may later grant a dealer manager/support relationship. Ownership can transfer without moving site history.

Telemetry/history/topology should attach to the site hierarchy:

site
  building
    zone
      device

Changing integrators is a relationship change, not a data transfer.

end old managing relationship
start new managing relationship
preserve site history
audit the change

Site Relationships

Accounts may relate to a site in different ways:

owner
manager
support_provider

Site relationships are v0 operational infrastructure. They support ownership, dealer/integrator management, and explicit support access.

Site relationships are not a general-purpose sharing product in v0. Do not use them for casual external visibility such as gardeners, caregivers, neighbors, status-only dashboards, or per-sensor notifications. Those use cases should be handled later by a scoped sharing/notification/feed model.

Rules:

* each site must have exactly one active owner relationship
* manager and support_provider relationships may be added or removed
* ended relationships remove live account-derived access immediately
* historical access is not automatic after relationship end
* the current site owner controls historical visibility by default

Resource Hierarchy

Buildings and zones are first-class resources and always exist.

When a site is created, seed:

site
default building
default zone

Devices attach to zones.

The UI may collapse simple hierarchies:

Site -> Device

But the underlying model remains:

Site -> Building -> Zone -> Device

Suggested Tables

The following tables include both authorization-owned records and Account / Site facts consumed by authorization. Canonical service ownership is defined in `account-site-service.md` and this auth spec.

users

id
cognito_sub
email
display_name
status
created_at
updated_at
deleted_at

accounts

id
name
account_type: dealer | customer | internal | support_provider
status
created_at
updated_at
archived_at
deleted_at

account_memberships

id
account_id
user_id
role_id
status: active | invited | disabled | removed
created_at
updated_at
removed_at

sites

id
customer_record_id
name
status
created_at
updated_at
archived_at
deleted_at

site_relationships

id
site_id
account_id
relationship_type: owner | manager | support_provider
status: active | ended | revoked
started_at
ended_at
created_by_user_id

resources

id
site_id nullable
account_id nullable
type: account | site | building | zone | device
parent_resource_id nullable
display_name
status
created_at
updated_at
archived_at
deleted_at

Notes:

* site, building, zone, and device resources live under the site tree
* account resources represent account-level admin scope
* orphaned resources are remediation states, not normal flow

roles

id
account_id nullable
name
description
is_system
created_at
updated_at
archived_at

System roles seeded globally:

owner
admin
operator
viewer

Custom roles may be supported later. The schema should allow them, but MVP does not need role-builder UI.

Seeded roles should be represented as configurable backend records rather than hard-coded permission checks. The backend may support customer-defined role records structurally, but the v0/v1 product UI must not expose customer role creation or editing.

role_permissions

id
role_id
action
created_at

Roles are bundles of actions.

Do not hard-code role behavior except for special safety policies such as final-owner protection and high-risk local operation policies.

access_grants

id
subject_type: user | account | service_account
subject_id
resource_id
role_id
inheritance_mode: inherit | direct_only
status: active | revoked | expired
created_at
created_by_user_id
revoked_at

Permissions are additive union.

No explicit deny rules in v0.

Direct grants are exceptions, not the normal path for account/site access.

Account-to-site grants must be relationship-gated: no account subject grant on a site, building, zone, or device is effective unless that account has an active relationship to the target site.

Direct user grants may intentionally bypass account membership/relationship inheritance, but they should be rare, explicit, audited, and visible to site owners.

audit_events

id
actor_subject_type
actor_subject_id
action
target_resource_id nullable
result: success | denied | failed
request_id
ip_address nullable
user_agent nullable
metadata_json
created_at

Audit records should preserve the actor, target, action, result, and relevant relationship/grant context at time of event.

Required Constraints

The implementation should enforce these at the database level when practical:

* unique active users.cognito_sub
* one active account membership per account_id/user_id
* exactly one active owner relationship per site
* no duplicate active site relationship for site_id/account_id/relationship_type
* site-tree resources must have parents in the same site
* account resources must not be mixed into site resource trees
* resource parentage must not form cycles
* active grants must reference active roles, resources, and subjects

Permission Evaluation

All protected operations call:

can(subject, action, resource, context)

Evaluation v0:

1. Resolve subject.
2. Resolve target resource and its site/resource ancestors.
3. Find direct grants on the target resource.
4. Find inherited grants from ancestor resources.
5. Include permissions from active account memberships flowing through active site relationships.
6. Include only relationship-gated account grants for site-tree resources.
7. Expand roles into permissions.
8. Apply hard safety policies and contextual policies.
9. If any active grant or inherited membership role allows the action and all contextual policies pass, allow.
10. Otherwise deny.

Account membership role inheritance:

* a user with active membership in an account receives that membership role on active sites related to that account
* the role applies to the site and descendant building/zone/device resources
* inheritance may be limited by direct_only grants where applicable
* relationship end immediately removes account-derived access

A more specific direct grant can add permissions beyond inherited permissions.

Example:

Bob is operator through Integrator Account.
Integrator Account actively manages Site A.
Bob is explicitly admin on Site A.
Bob receives operator permissions generally for Site A and admin permissions on Site A.

There are no subtractive overrides in v0.

Contextual Authorization

Authorization may require context beyond role permissions.

Example: remote control command.

Allow only if:

user has command permission
AND account relationship/membership permits the site
AND site policy allows the command category
AND local app policy allows the command category
AND app/device policy allows the command
AND command is enabled/supported
AND target is active

Routes must not pass authoritative booleans such as remote_control_enabled: true.

The route may pass command intent:

can(subject, "device:command", deviceResource, {
  command_type: "restart_agent"
})

AuthorizationService or a trusted policy resolver loads authoritative site, app, and device policy from Postgres and device state. The local app remains the final execution authority for Home Assistant actions.

Seeded Role Recommendations

Viewer

Can:

account:view
site:view
device:view
telemetry:view
topology:view
alert:view
alert_recipient:view
audit:view limited if exposed

Cannot:

commands
invites
role changes
billing changes
site deletion
remote control
host-write operations

Operator

Can:

viewer permissions
site:refresh
device:refresh
device:diagnose
diagnostics:request
alert:acknowledge
alert_recipient:view

Cannot by default:

device:command destructive
agent:update
host:reboot
backup:trigger unless explicitly granted
backup:restore
billing:manage
member:update_role
member:manage_owner
site:delete

Admin

Can:

operator permissions
site:create
site:update
device:update
member:view
member:invite
member:remove non-owner
member:update_role below owner
role:view
billing:view optional
alert_recipient:view
alert_recipient:manage

Cannot by default:

account:delete
billing:manage
owner assignment/removal
remove final owner
agent:update high-risk unless explicitly granted
host:reboot unless explicitly granted
backup:trigger high-risk unless explicitly granted
backup:restore unless explicitly granted

Owner

Owner is seeded with all actions in role_permissions.

Owner can:

all permissions
billing:manage
account:delete
site:delete
owner management
role management

Safety constraints still apply:

cannot remove/demote final owner
cannot delete themselves if they are the final owner

Owner Management

Normal role changes use:

member:update_role

Owner assignment, owner removal, and owner demotion require:

member:manage_owner

Hard safety policies:

* non-owner cannot assign owner
* non-owner cannot remove or demote owner
* no actor can remove, demote, disable, or delete the final owner
* final-owner protection failures are audited

The evaluator should receive target-role context for membership changes:

can(subject, "member:update_role", accountResource, {
  target_user_id,
  from_role,
  to_role
})

Action Catalog v0

account:view
account:update
account:delete
member:view
member:invite
member:update_role
member:manage_owner
member:remove
role:view
role:create
role:update
role:delete
site:view
site:create
site:update
site:delete
site:archive
site:relationship_manage
customer_record:view
customer_record:create
customer_record:update
building:view
building:create
building:update
building:delete
zone:view
zone:create
zone:update
zone:delete
device:view
device:update
device:refresh
device:diagnose
device:command
device_claim_invite:view
device_claim_invite:create
device_claim_invite:cancel
device_claim_invite:replace
agent:claim
agent:release
agent:update
agent:restart
host:reboot
backup:trigger
backup:restore
diagnostics:request
remote_access:view
remote_access:update
telemetry:view
topology:view
alert:view
alert:acknowledge
alert_recipient:view
alert_recipient:manage
billing:view
billing:manage
audit:view
support:grant
support:access
service_account:view
service_account:create
service_account:update
service_account:delete
api_key:create
api_key:revoke

High-risk actions should remain separate:

agent:update
agent:restart
host:reboot
backup:trigger
backup:restore
device:command destructive
site:delete
account:delete
billing:manage
member:manage_owner
remote_access:update when it changes local/provider configuration

Do not hide dangerous operations behind one broad admin permission.

Invitations

Invitations create pending access.

invitations
- id
- email
- account_id
- resource_id nullable
- role_id
- inheritance_mode
- status: pending | accepted | expired | revoked
- invited_by_user_id
- expires_at
- accepted_at

On acceptance:

1. create or resolve local user
2. create account membership if needed
3. create access grant if site/resource-scoped
4. mark invitation accepted
5. write audit event

Site Relationship End Behavior

When an account relationship to a site ends:

- live dashboard access ends immediately
- command access ends immediately
- future telemetry visibility ends immediately
- historical telemetry is not visible by default
- account subject grants on the site tree stop being effective
- audit records remain preserved internally

Historical access can be introduced later through explicit policy or grants.

Default posture:

current site owner controls historical visibility

Support Access

No impersonation.

No break-glass in MVP.

Support access must be explicit in the model, even if not fully built.

Early coarse settings:

allow_remote_control: true | false
local_authority_profile: standard | managed_admin

Future support access modes:

implicit
request_required
disabled

Future support session model:

support_sessions
- id
- site_id
- account_id
- access_level: read | write | command
- status: active | expired | revoked
- requested_by
- approved_by
- expires_at
- created_at

For MVP, do not build hidden vendor god-mode. Managed local authority is explicit site/app configuration, not a cloud-only override.

Lifecycle Terms

archived:
  hidden from normal UI, retained historically
released:
  relationship ended; object may be re-associated through an authorized flow
deleted:
  soft-deleted; inactive but retained for audit/retention
revoked:
  credential/access forcibly invalidated
orphaned:
  object exists without required active parent/relationship; remediation state
inactive:
  valid object, but not currently operating/seen

Use soft deletion for auth-relevant entities.

Middleware Contract

Every protected endpoint follows:

requireAuth()
resolveSubject()
resolveTargetResource()
requirePermission(action, resource, context)
handler()
auditIfSensitive()

Examples:

PATCH /accounts/:accountId/members/:userId
  -> requirePermission("member:update_role", accountResource, { target_user_id, from_role, to_role })
POST /sites/:siteId/refresh
  -> requirePermission("device:refresh", siteResource)
POST /devices/:deviceId/commands/restart
  -> requirePermission("device:command", deviceResource, { command_type: "restart_agent" })

Audit Requirements

Audit at minimum:

role changes
owner assignment/removal failures and successes
invites accepted/revoked
membership changes
customer record changes
site relationship changes
site creation/archive/delete
device claim/release
remote control commands
host-write commands
support access changes
billing/admin changes
API key creation/revocation
auth failures for sensitive actions
final-owner protection failures

Audit should capture authorization context at time of event:

actor
subject type
action
target resource
result
relationship/grant used if practical
local authority profile if relevant
before/after metadata for sensitive changes
request_id
created_at

API Key Future Fit

API keys are credentials, not authorization subjects.

Model:

service_account = subject
api_key = credential for service_account

Future tables:

service_accounts
- id
- account_id
- name
- status
- created_by_user_id
- created_at
- revoked_at
api_keys
- id
- service_account_id
- key_hash
- name
- created_at
- expires_at
- revoked_at
- last_used_at

Service accounts receive grants like users.

Non-Goals v0

Do not build:

explicit deny rules
OpenFGA integration
custom role editor UI
user impersonation
break-glass vendor access
full support-session workflow
API key UI
complex conditional policies
per-field permissions
generalized cross-account site sharing UI
gardener/caregiver/scoped sensor sharing
hard-delete of auth-sensitive records

Required Tests

Add authorization unit tests early.

Minimum test cases:

viewer can view site but cannot command device
operator can refresh and request diagnostics but cannot delete site
admin can invite user but cannot remove final owner
owner has all seeded permissions subject to safety policies
final owner cannot remove/demote self
account member role flows through active managed site relationship
ended site relationship loses live account-derived access
account subject grants on site tree require active site relationship
direct user grants are additive and audited
site owner controls historical visibility by default
remote_control=false blocks command even with cloud permission
managed/admin local authority is required for host-write actions
site creation seeds default building and zone
permissions are additive across inherited and direct grants
valid Cognito token for disabled local user is denied
API route cannot bypass AuthorizationService

Final v0 Position

Build a Postgres-backed, resource-based authorization system behind a central AuthorizationService.

Use Cognito only for human authentication.

Model account as the business and authority container. Account / Site owns account records and site relationships; Authorization owns account memberships, roles, grants, and permission evaluation.

Model site as the durable data container.

Use customer records for real-world customer/site information without requiring customer login accounts.

Use site relationships for ownership, dealer/integrator management, and explicit support access, but not as a general external sharing product.

Use seeded configurable roles, additive permissions, inherited account membership roles, relationship-gated account grants, mandatory audit, and no explicit deny rules.

Keep the data model ready for site-level delegation, support access, future service-account/API-key integrations, and later scoped sharing without making broad sharing a v0 feature.
