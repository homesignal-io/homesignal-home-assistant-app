HomeSignal Edge Device Management Requirements

Canonical note:
This is a broad requirements sketch. When it conflicts with current service
architecture, the canonical docs win: `service-map.md`,
`workstreams/device-lifecycle.md`, `api-facade.md`,
`aws-iot-routing-contract.md`, `telemetry-ingest-architecture.md`,
`edge-state-adapter.md`, `command-lifecycle.md`, and
`artifact-upload-broker.md`.

1. Core Architecture

System must use an outbound-only agent model.

Home Assistant host runs HomeSignal Manager app.

Claimed-device runtime uses AWS IoT Core for command notices, notifications,
lifecycle presence, and compact edge state. Agent HTTPS with mTLS handles
telemetry/events, command ACK/results, artifact negotiation, and other approved
post-claim request/response flows. WebSocket/WSS is not a v0 default transport
unless a later spec explicitly introduces it.

No inbound ports required.

No customer LAN access required.

Remote access is optional and separately configured.

Postgres is the canonical source of truth.

If a WebSocket gateway is introduced later, it is transport only, not authority.


2. Device Identity

Each installation must have a stable HomeSignal device ID.

Device identity must survive agent restarts.

Device identity must not depend only on hostname, IP, MAC address, or HA instance name.

Device must generate local installation identity at first run.

Cloud must bind device identity to:
integrator account
customer
site
Home Assistant instance
agent installation


3. Enrollment / Claiming

Unclaimed devices must not receive operational commands.

Claiming requires:
valid site-bound claim invite
logged-in authorized integrator
local app verification of the claim invite details
local app confirmation after verification
device-reported unclaimed state

Claim invites must expire.

Claim invite codes must be single-use.

Backend must prevent already-claimed devices from being silently claimed again.

Claiming must create durable credentials.

Temporary claim credentials must be discarded after enrollment.

Claim flow must support stock Home Assistant installs without preloaded hardware.


4. Release / Transfer / Fresh Claim

System must support release of a device from a site.

Release must revoke cloud credentials.

Release must not break Home Assistant itself.

Released device returns to unclaimed state.

Offline release must still revoke cloud access.

Transfer must be separate from release and fresh claim.

A fresh claim creates a new HomeSignal `device_id` and does not migrate history.
The old cloud record remains under the original account/site authority and should
appear disconnected when old credentials stop reporting. Any future transfer
feature that changes controlling account/site ownership while preserving or
copying history must be an explicit product flow, not an implication of claim,
release, or local reset.


5. Edge State Adapter / Desired vs Reported State

System must maintain compact desired and reported edge state.

Use `edge-state-adapter.md`, `workstreams/state-change-and-policy-propagation.md`, and `command-lifecycle.md` as the canonical model. V0 uses AWS IoT named shadows through the Edge State Adapter for compact desired/reported edge state. Durable intended state and bounded commands are separate concepts. Commands should be generated from desired-state changes only where local convergence requires a bounded attempt or repair.

Desired state examples:
required agent version
backup policy
telemetry interval
enabled checks
update policy
managed apps
remote access provider metadata

Reported state examples:
HA version
Supervisor version
agent version
hardware type
storage status
backup status
update status
last heartbeat
integration health
remote access configured/not configured

System should compute convergence/drift where desired-state convergence matters:
desired != reported

Drift/convergence details are internal/support-visible in v0. Customer-facing
UI may later show simple health/status, but detailed degraded/convergence UX is
not productized until a UI/product spec defines it.

HomeSignal database stores product truth and compact edge-state projections. It must not mirror whole AWS IoT shadow documents by default. A future HomeSignal Device Twin service can replace the Edge State Adapter later if AWS shadows stop fitting the product.


6. Heartbeat / Presence

Agent must send periodic heartbeat.

Routine health/presence facts include:
device ID
agent version
HA version summary
health summary
timestamp
connection/session ID

Connection/session identifiers are operational provenance only. Product joins
use durable `device_id`, which equals AWS IoT Thing name for claimed devices.

Backend must track:
last_seen_at
online/offline
degraded
stale
released/revoked

Online state must be derived from AWS IoT lifecycle/presence and recent
telemetry or health facts, not from payload claims alone.

System must tolerate missed heartbeats without immediate false alarms.

Reconnects must use backoff and jitter.


7. Command Lifecycle

Commands follow `command-lifecycle.md` as the canonical lifecycle:
queued
sent
ack_accepted
ack_rejected
ack_timed_out
running/progress, when command-specific
succeeded
failed
timed_out
canceled

Every command must have:
command_id
device_id
requested_by
created_at
expires_at
type
payload
status
result
audit metadata

Agent must ACK accepted or rejected within the command ACK window, not merely receipt. See `command-lifecycle.md`.

Agent must report command result.

Commands must be idempotent where possible.

Commands must be scoped and allowlisted.

Agent must reject unknown command types.

Dangerous commands require stronger confirmation.


8. Supported MVP Command Families

request bounded HomeSignal diagnostics
trigger backup
refresh publish policy, when a bounded repair/acceleration attempt is required
request update/apply-update status through the update architecture
report backup status through telemetry/result paths
release/revoke local credential cleanup where explicitly supported
test remote access metadata, future

Later:
apply template
future local-supervisor stage update
future local-supervisor execute update
restore backup
install managed app


9. Security Posture

All device-cloud communication must use TLS.

Device credentials must be high-entropy, per-device, scoped, and revocable.

Device tokens must never be exposed in UI.

Device tokens must not be stored in browser localStorage.

Claim codes must not become permanent credentials.

Backend must authorize every command against:
user
integrator account
site
device
role
device state

Agent must enforce local allowlist of permitted operations.

No arbitrary shell execution in MVP.

No unrestricted file read/upload.

No LAN scanning by default.

No subnet routing by default.

No required VPN.

All sensitive actions must be audit logged.


10. Credential Lifecycle

Enrollment issues durable device credential.

Credential can be revoked immediately.

Credential rotation must be supported.

Device must detect revoked credential and enter safe state.

Lost credential recovery requires re-pairing.

V0 credential direction:
device-generated private key, CSR submitted through HomeSignal claim flow, AWS
IoT-signed device certificate returned to the app, and mTLS Agent HTTPS
authorization by exact stored certificate fingerprint/serial.


11. Access Control

Roles:
Owner
Admin
Technician
Read-only

Permissions must cover:
create site
claim device
release device
transfer device
view health
run commands
manage backups
manage team
configure remote access
view audit log

Technicians should only access assigned sites where possible.


12. Remote Access

Remote access is optional.

HomeSignal must support storing remote access metadata:
provider
URL
node name
notes
last verified
configured_by

Supported providers:
manual URL
Tailscale
Nabu Casa
Cloudflare Tunnel
VPN/custom

Remote access link must be separate from agent health.

Browser reachability check is convenience only, not authoritative health.

HomeSignal must not require access to customer private networks.


13. Backup Management

Agent must report backup status.

System must track:
last successful backup
last failed backup
backup location metadata
backup policy
retention policy
backup size
failure reason

V0 supports local HA backup status/trigger flows and may store offsite Home
Assistant backup bytes through the approved Artifact Upload Broker path under
Backup Service ownership.


14. Update Management

Agent must report:
HA Core update availability
HAOS update availability
Supervisor update availability
agent update availability
managed app update availability

System must support HomeSignal app update intent/status policy:
manual
notify only
approved window
blocked version
staged rollout

For v0, HomeSignal does not initiate Home Assistant, Supervisor, OS, database,
or arbitrary host updates. HomeSignal app installation remains governed by
the Home Assistant Supervisor/app release path and local policy.


15. Diagnostics

Agent must collect safe diagnostic bundle:
versions
health summary
logs from HomeSignal app
Supervisor status
recent command results
backup status
storage summary

Diagnostics upload must be explicit and audited.

Sensitive secrets must be redacted.

Large diagnostics must use HTTPS upload, not WebSocket.


16. Audit Logging

Audit log must capture:
login
site creation
device claim
device release
device transfer
command requested
command completed
credential revoked
remote URL changed
team/user permission changes

Audit entries include:
actor
timestamp
account
site
device
action
result
source IP if available


17. Failure Handling

If cloud is unreachable:
agent continues local HA operation
agent queues non-dangerous reports locally within limit
agent reconnects with backoff

If agent is offline:
cloud marks device offline/stale
commands remain queued until expiry

If device is released while offline:
cloud revokes credential immediately
device is denied on next reconnect

If duplicate device identity appears:
backend must block or quarantine session

If pairing race occurs:
first valid claim wins
all others fail


18. Data Model Minimum

accounts
users
roles
customers
sites
devices
device_credentials
device_claim_invites
device_claim_verifications
device_latest_state
device_desired_state
commands
command_results
heartbeats
alerts
backups
remote_access_links
audit_events


19. Alerts

Initial alert/status candidates:
device offline
heartbeat stale
backup failed
backup overdue
agent version outdated
HA update available
storage high
credential revoked
claim failed
command failed

Product/customer alerting is not automatically implied by every candidate.
For v0, Alerting Service owns customer-facing alert lifecycle for promoted
product rules such as disconnected devices, backup failed/overdue, and
app/update attention. Notification Service owns email delivery through the
provider adapter. Platform Health findings remain internal/support-only in v0
unless a product alert rule explicitly promotes a condition. Product alerts
support:
severity
acknowledge
resolve
snooze later


20. Non-Goals for MVP

No custom VPN.

No subnet routing.

No arbitrary remote shell.

No full HA fork.

No custom HAOS image required.

No mandatory Tailscale.

AWS IoT Core is the planned claimed-device transport and edge-state surface.

No high-frequency telemetry pipeline.

No customer LAN inventory scanning.


21. MVP Success Criteria

Integrator can:
create site
install HomeSignal app
claim HA device
see online/offline status
see basic HA health
trigger diagnostics
trigger backup
see backup status
store remote access URL
release device safely

System can:
securely enroll device
prevent stale/replayed claims
revoke device access
track desired/reported edge state
durably track commands through the command lifecycle
audit all sensitive operations
operate without inbound network access
