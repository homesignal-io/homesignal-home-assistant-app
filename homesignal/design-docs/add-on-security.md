# HomeSignal Add-On Security And Local Authority

## Goal

Define how the HomeSignal Manager add-on should treat local Home Assistant authority and how that authority relates to HomeSignal cloud authorization.

This document covers add-on security posture, local authority profiles, Home Assistant user visibility, and command execution boundaries. It does not define Cognito auth, account/site RBAC, AWS IoT device authentication, MQTT topic policy, or telemetry ingestion.

## Core Principles

* The add-on is the local execution authority for Home Assistant actions.
* HomeSignal cloud authorization alone is never enough to mutate a Home Assistant host.
* Local authority is explicit site/add-on configuration.
* Dealer-managed installs may reasonably run with elevated local authority.
* Self-managed installs should make elevated local authority a clear consent decision.
* The add-on must reject unsupported, unsafe, or disallowed commands even when cloud authorization succeeds.
* No arbitrary shell execution in MVP.
* No unrestricted file read/upload.
* No LAN scanning by default.
* All sensitive local actions must be audited through HomeSignal.

## Authority Profiles

HomeSignal should support two local authority profiles conceptually.

standard

Standard mode is the narrow default posture for enrollment, status, telemetry, and safe operations.

Standard mode should support:

* enrollment and claim state
* non-secret local status
* health/readiness
* telemetry and compact edge reported state through AWS IoT paths
* safe refresh/diagnostic metadata where supported
* command refusal for host-write operations

managed_admin

Managed/Admin mode is an optional elevated posture for dealer-managed sites or self-managed sites whose Home Assistant admin explicitly grants HomeSignal management authority.

Managed/Admin mode may support:

* richer Home Assistant/Supervisor context
* Home Assistant user visibility when the platform allows it
* backup trigger
* restore flows when explicitly supported later
* add-on/plugin restart
* host or Supervisor operations exposed by supported APIs
* managed update or local supervisor workflows

Managed/Admin mode is not a hidden vendor override. It is a local capability granted by the installer/admin and still bounded by HomeSignal cloud RBAC, site policy, add-on policy, command allowlists, risk tiers, and current device/plugin state.

## Dealer-Managed Versus Self-Managed

Dealer-managed installs are a first-class product posture.

In a dealer-managed install, the dealer is acting professionally on behalf of the customer. The customer may not be the Home Assistant operator, and the customer may not have a HomeSignal login account yet. For this path, Managed/Admin mode can be a normal operating choice.

Self-managed installs are different. The Home Assistant admin and the customer/operator may be the same person. For this path, elevated local authority should be explicit, understandable, and easy to leave off.

The product should eventually make this distinction clear in onboarding:

* Dealer Managed
* Self Managed
* Co-managed or transferred later

The same underlying agent may support both, but the trust ceremony and defaults can differ by path.

## Home Assistant Permission Surface

Home Assistant add-on permissions are primarily declared in add-on configuration, not dynamically requested like browser permissions.

Implementation must decide whether elevated authority is delivered by:

* one add-on that declares broad permissions up front and gates behavior with an internal HomeSignal mode switch
* separate Standard and Managed add-on profiles/packages
* a later migration from Standard to Managed that requires reinstall/reconfigure

This document does not choose the packaging shape. It defines the security behavior either way.

Broad permission requests may affect user trust at install time. That is acceptable for a supervisory/dealer-managed product when presented clearly, but it should not be described as a passive monitoring add-on.

## Local Command Gate

For any cloud-requested local action, execution requires all relevant gates:

1. Cloud user/service subject is authenticated.
2. HomeSignal AuthorizationService permits the action.
3. Site relationship and membership rules permit the target site.
4. Site policy permits the operation category.
5. Add-on local authority profile permits the operation category.
6. Plugin/device policy permits the specific command.
7. Command type is known and allowlisted.
8. Current device/plugin state allows safe execution.
9. Command is audited before and after execution.

The add-on must treat cloud requests as instructions to evaluate, not as commands to blindly execute.

## Host-Write And High-Risk Operations

The following operations require Managed/Admin mode and specific cloud permissions:

* backup trigger
* backup restore
* host reboot
* HomeSignal add-on restart
* agent/plugin update
* remote access provider/local configuration update
* any write to Home Assistant, Supervisor, add-on, host, or local managed component state

High-risk operations should remain separate in the cloud action catalog. Do not hide them behind one broad command permission.

Backup trigger is high risk because repeated or poorly timed backups can fill disk or degrade the host.

Remote access provider/local configuration update is high risk because it can change how operators reach or expose a site. Plain cloud-only notes or URL metadata may use a lower-risk cloud permission, but changing local/provider configuration must be treated as managed/admin authority.

## Home Assistant User Handling

The add-on may use current Home Assistant ingress user context when available. This is useful for attribution, local UX, and debugging.

Full Home Assistant user inventory is not required for v0 onboarding.

If HomeSignal later pulls a list of Home Assistant users, it must be:

* limited to Managed/Admin mode
* explicit and opt-in
* treated as local context, not HomeSignal identity authority
* stored minimally
* never used to grant HomeSignal cloud permissions automatically

HomeSignal identities come from Cognito and local HomeSignal users. Home Assistant usernames can help identify who is present on the local system, but they do not replace HomeSignal auth.

## Customer And Site Onboarding

HomeSignal cloud onboarding should capture a lightweight customer record for every site, even when the dealer owns the site account relationship.

The add-on may help collect local context, but it should not require Home Assistant user inventory to create a customer record.

Customer record fields may include:

* customer/site display name
* email
* phone
* service address
* notes
* optional browser location with consent

Browser location is optional and skippable. It is onboarding metadata, not an authorization signal.

## Audit Requirements

Audit at minimum:

* local authority profile changes
* cloud-requested command accepted/denied
* command delivered to local execution
* command succeeded/failed/refused
* backup trigger/restore
* host reboot/restart/update operations
* Home Assistant user inventory import if ever enabled
* remote access metadata or provider/local configuration changes
* local policy refusal for safety reasons

Audit records should preserve:

* HomeSignal actor
* site and device
* command/action
* local authority profile
* local policy decision
* result
* relevant before/after metadata
* timestamps

## Non-Goals v0

Do not build:

* arbitrary shell execution
* unrestricted file browsing/upload
* LAN scanning by default
* subnet routing by default
* automatic Home Assistant user import
* cloud-side override of local add-on refusal
* hidden support/vendor execution channel
* broad external sharing through local Home Assistant users

## Required Tests

Add tests before shipping managed local actions:

* standard mode refuses host-write operations
* managed_admin mode still requires cloud permission
* unknown command types are refused locally
* backup trigger requires specific permission and managed_admin mode
* local refusal is audited
* cloud permission without local policy fails closed
* current HA user context does not grant HomeSignal permissions
* full HA user inventory is unavailable unless managed/admin capability is enabled

## Final Position

HomeSignal may be a supervisory product with elevated local authority, especially for dealer-managed installs. That authority must be explicit, locally granted, allowlisted, audited, and subordinate to both cloud RBAC and local add-on safety checks.

The add-on should be powerful when the site owner/dealer chooses managed operation, but it must never become an unbounded remote execution channel.
