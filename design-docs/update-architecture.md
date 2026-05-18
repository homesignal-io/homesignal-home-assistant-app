You should treat this as a safety-critical distributed systems problem, not “app deployment.” Integrators will tolerate feature gaps. They will not tolerate bricked sites, broken automations, unreachable boxes, or mystery failures after an update.

The core mistake most IoT/device-agent platforms make is treating the edge agent like a SaaS frontend. It is not. Your edge agent is closer to firmware.

A good mental model:

* Cloud = orchestration + intelligence
* Edge agent = durable appliance runtime
* Updates = controlled migrations, not deployments

Given your HA/integrator context, I would strongly recommend these principles:

⸻

1. Immutable, versioned agent releases

Never “hot mutate” a running installation.

Every release should be:

* signed
* immutable
* content-addressed
* rollbackable

Think:

* OCI image digest
* or signed package bundle
* with exact dependency versions

Never:

* latest
* floating dependencies
* runtime pip/npm installs
* self-modifying environments

Integrators hate:

* “it worked yesterday”
* “dependency upstream changed”
* “Python package broke SSL”
* “Node updated and websocket auth broke”

Your release artifact should be reproducible byte-for-byte.

⸻

2. Dual-track architecture: stable vs candidate

You need channels.

At minimum:

Channel	Audience
Stable	default production installs
Candidate	opt-in integrators
Dev	internal only

Do not let customers jump arbitrary versions casually.

You want controlled upgrade paths:

* 1.4 → 1.5
* not 1.1 → 1.9

Because migrations become impossible to reason about otherwise.

⸻

3. A/B rollback or snapshot rollback

This is non-negotiable eventually.

You need one of:

* A/B partitioning
* container snapshot rollback
* atomic image swap
* supervisor-managed previous-version restore

A failed update must:

1. fail health checks
2. auto-revert
3. preserve local state
4. preserve credentials
5. come back online automatically

The integrator should wake up to:

“Update reverted automatically due to failed health check.”

not:

“The site is offline.”

⸻

4. Health checks must reflect actual function

Most systems do fake health checks:

* process alive
* TCP port open

Worthless.

Your checks should validate:

* HA connectivity
* MQTT broker connectivity
* local DB writable
* outbound cloud auth valid
* queue drain functioning
* websocket subscriptions active
* disk pressure acceptable
* memory leak thresholds
* event processing latency

You need:

* startup health
* steady-state health
* degraded-mode health

⸻

5. Explicit degraded modes

This is critical.

The edge agent should continue operating locally even if:

* cloud unreachable
* auth expired
* update service broken
* metrics upload failing

Local-first authority is correct for your architecture.

Cloud loss should degrade:

* fleet management
* analytics
* orchestration

—not local automation collection/runtime.

Integrators absolutely need this.

⸻

6. Separate control plane from data plane

Huge design issue.

Do NOT couple:

* telemetry
* command execution
* updates
* websocket control
* local processing

into one monolithic connection.

You want:

* independent queues
* resumable operations
* durable local buffering

Otherwise one failure cascades.

⸻

7. Database migrations are the real risk

Not binaries.

Almost every catastrophic release failure is:

* schema migration
* corrupted state
* incompatible persistence model

Rules:

* migrations must be forward compatible
* ideally reversible
* old agent should survive rollback against migrated DB
* never destructive in same release

Pattern:

1. additive schema
2. dual-write
3. migrate reads
4. cleanup later

Slow boring migrations win.

⸻

8. Your update orchestrator needs “site awareness”

Home Assistant environments are messy.

You’ll encounter:

* Raspberry Pi SD corruption
* low disk
* broken DNS
* captive VLANs
* old kernels
* power instability
* integrator custom hacks
* unsupported apps
* weird MQTT topologies

So updates must evaluate:

* available disk
* memory
* CPU architecture
* dependency compatibility
* HA version compatibility
* backup success
* uptime state

before applying.

“Update available” is not enough.

⸻

9. Mandatory pre-update snapshot/backup

Before update:

* snapshot config
* snapshot local DB
* snapshot credentials
* snapshot mappings/topology

Prefer automatic.

If restore requires CLI recovery, your UX failed.

⸻

10. Treat agent commands as distributed transactions

Especially dangerous operations:

* restart HA
* firmware update
* credential rotation
* network changes
* MQTT rebinding

Need:

* idempotency
* checkpoints
* timeout recovery
* resumability
* audit trail

Because devices WILL:

* reboot mid-operation
* lose power
* lose WAN
* partially apply

You already identified this pattern elsewhere with deterministic execution. Same principle here.

⸻

11. Strong release rings

You should eventually support:

* internal
* dogfood integrators
* low-risk customers
* general rollout

Rollouts should be:

* progressive
* measurable
* haltable

You need automated rollback triggers based on:

* crash rate
* memory growth
* disconnect rate
* update failure rate
* HA API failures

⸻

12. Minimize agent responsibility

Critical architectural point.

Your edge agent should NOT:

* contain business logic explosion
* dynamically execute arbitrary AI code
* become app spaghetti
* host uncontrolled third-party extensions

Because maintenance burden explodes.

Prefer:

* deterministic runtime
* declarative configs
* signed modules
* feature flags
* constrained execution model

Your instinct about agent outputs being constrained DSL/config instead of arbitrary runtime code is correct.

⸻

13. Build an “offline serviceability” story

Integrators eventually need:

* local UI
* recovery shell
* local logs
* export bundle
* emergency disable update
* manual rollback
* local diagnostics

Because some sites will have:

* no WAN
* broken WAN
* broken DNS
* firewall changes

If recovery requires your cloud, you created a single point of failure.

⸻

14. Observability must be temporal

This matters more than dashboards.

Integrators need:

* “what changed?”
* “when did degradation begin?”
* “what version was running?”
* “what update preceded this?”

So your system should aggressively track:

* version timelines
* topology changes
* config diffs
* device churn
* latency drift
* resource drift

Most operational debugging is temporal correlation.

⸻

15. Your biggest long-term risk is version skew

You will eventually have:

* old HA versions
* old agent versions
* old app versions
* incompatible APIs
* abandoned installs

You need a compatibility policy early.

Example:

* support N-2 HA major versions
* support agent versions for 12 months
* forced minimum version for cloud APIs

Otherwise maintenance cost becomes exponential.

⸻

The important meta-point:

The integrator pain is not “updates are annoying.”

The pain is:

“I no longer trust touching this system.”

Once trust is lost:

* updates stop
* installs drift
* support costs explode
* fleet fragmentation kills velocity

So your real product is not the agent.

It is:

predictable operational behavior under failure.


## Addendum: Local Supervisor Architecture

A core architectural principle of the platform is that update orchestration and local update execution are intentionally separated.

The cloud control plane does not directly mutate devices. Instead, it publishes desired state and rollout intent through the platform's desired-state boundary. For v0 edge state, that boundary is the Edge State Adapter around AWS IoT named shadows.

For v0 Home Assistant app updates, HomeSignal does not initiate the binary update through IoT Core. Release artifacts are published through the normal app release channel, such as the Home Assistant app repository/GitHub path, and the local Home Assistant Supervisor/app update mechanism performs the local install according to local policy. HomeSignal may publish desired version/channel intent, observe reported version and health, and surface update status, but the local supervisor/runtime remains the execution authority.

The Home Assistant app repository/channel strategy is owned by
`ha-app-repository-release-strategy.md`. The summary is:

- one HomeSignal product source repo
- public, CI-generated Home Assistant app distribution repos/channels
- production stable and production candidate/test cohort both point at
  production HomeSignal
- staging/non-prod points only at staging HomeSignal
- cohort steering for new installs happens through pairing/invite install links
- existing installs cannot be silently moved from one repository/channel to
  another by cloud intent alone
- Home Assistant Supervisor remains the local install/update executor

Desired HomeSignal app version is a valid `homesignal_edge.update`
shadow target. This is rollout intent and convergence tracking, not binary
delivery. A typical release flow is:

1. CI/CD publishes a new HomeSignal app version to the normal release
   channel, such as the GitHub/Home Assistant app repository path.
2. Release / Update Orchestrator records the version, channel, compatibility,
   and rollout metadata in HomeSignal product state.
3. When the release is promoted for a cohort, the Edge State Adapter writes the
   desired app version/channel into `homesignal_edge.update`.
4. The app reports its observed installed version/status back through the
   compact shadow `reported.update` shape and/or the approved runtime status
   path.
5. HomeSignal compares desired vs reported state to see which devices took the
   update, which are pending, which are blocked by local policy, and which
   failed or rolled back.

The shadow must not contain release artifacts, download URLs, signed URLs, or
large release metadata. It should carry only compact desired/reportable facts
needed for convergence.

## Home Assistant Version Advisory

Home Assistant Core version drift is a portal advisory in v0, not a cloud-driven
update command and not an email alert by default.

HomeSignal should maintain a small version-catalog adapter/cache for the latest
stable Home Assistant Core version. The cache may be refreshed daily. Device
latest-state reports the installed Home Assistant version; API read models may
compare installed vs cached latest to show `ha_update_advisory`.

If the catalog source is unavailable, stale, or ambiguous, the portal should
hide the Home Assistant update advisory rather than warning the user from a weak
source. HomeSignal does not initiate Home Assistant Core updates in v0.

A lightweight HomeSignal local supervisor may become a later hardening layer. Until then, treat "supervisor" below as the local update authority pattern, not a separate v0 process.

If introduced later, this supervisor would be HomeSignal code and should be treated as durable infrastructure rather than application logic.

### Architectural Split

#### Cloud Control Plane
Responsible for:
- release management
- rollout cohorts/rings
- desired version assignment
- release channels (stable/candidate/dev)
- promotion of a published app version into shadow desired state
- rollout pause/resume
- fleet visibility
- telemetry aggregation
- compatibility policy
- signed artifact publication

The cloud should never directly execute arbitrary remote commands against managed sites.

#### Future Local Supervisor
A future stable local daemon would be responsible for:
- reading desired state through the Edge State Adapter / AWS IoT shadow path
- authenticating with the control plane
- verifying signed artifacts
- evaluating local update readiness
- creating snapshots/backups
- applying updates atomically
- restarting managed services
- executing health checks
- automatically rolling back failed updates
- buffering telemetry/logs while offline
- exposing local diagnostics/recovery capabilities

If introduced, the supervisor should remain intentionally conservative, deterministic, and minimal in scope.

The supervisor is not:
- an AI runtime
- an app marketplace
- a general-purpose script execution engine
- a fast-moving feature layer

Its primary responsibility is maintaining operational safety and recoverability of the local system.

#### Agent / App Layer
The higher-level application layer responsible for:
- Home Assistant integration
- topology discovery
- telemetry collection
- automation/control workflows
- user-facing functionality
- cloud synchronization

This layer is intentionally replaceable and allowed to evolve more rapidly than the supervisor.

If introduced, the supervisor must remain operational even when the agent/app layer fails and should be capable of:
- rollback
- recovery
- diagnostics
- reconnection to cloud services

### Rollback Authority

Rollback is an essential platform capability, not a support-only escape hatch.

V0 rollback policy:

- User/integrator-approved rollback is allowed for HomeSignal app releases when the rollback target is still supported.
- Automatic rollback is allowed only for HomeSignal-controlled app update attempts that fail bounded local health checks, fail startup, fail to reconnect/authenticate to HomeSignal within the update window, or report a known bad update result.
- Automatic rollback must preserve local HomeSignal identity, private keys, claim state, and local configuration.
- Automatic rollback must report the attempted version, rollback version, reason, and terminal result when connectivity returns.
- HomeSignal must not automatically roll back broader Home Assistant, Supervisor, OS, database, or unrelated host state unless a later local supervisor spec explicitly owns that surface.
- Support may advise or trigger a rollback only through the same authorization and audit path available to the appropriate customer/integrator role; support-only hidden rollback authority is not a v0 product model.

Rollback should be visible in product UI history because integrators need confidence that updates are recoverable and explainable.

### Desired State Model

The platform follows a desired-state reconciliation model.

The cloud declares intent:

```json
{
  "site_id": "site_123",
  "component": "ha_app",
  "target_version": "1.8.3",
  "release_channel": "stable",
  "artifact_digest": "sha256:...",
  "rollout_id": "rollout_456"
}
```

That full intent record is HomeSignal product state. The shadow projection is
smaller: desired version, channel, rollout/reference identifiers, and bounded
status/reason fields. Release artifacts, download URLs, signed URLs, and large
release metadata stay out of the shadow.
