# Platform Health / Monitoring Service

This is a future logical service boundary for detecting unhealthy platform and cross-device behavior across HomeSignal. It is not part of v0.

This does not mean v0 ignores device health. V0 should collect and show bounded device/add-on health through Telemetry Ingest, latest-state projections, and API reads. The future Platform Health / Monitoring service is for slower cross-service interpretation, correlation, suppression, and remediation recommendations.

V0 platform-health posture:

- Platform Health / Monitoring findings are internal/support-only in v0.
- Product UI may show simple device health derived from telemetry/latest-state projections, but not platform-health findings.
- V0 should emit and retain enough summarized signals for future platform-health correlation.
- Store minimal per-device latest counters/projections needed for support; emit metrics/events for broader analysis rather than building a full platform-health store in v0.

## Scaffold Plan

Build Platform Health as a future service by scaffolding its inputs first, not
by building a rule engine in v0.

V0 should create the source facts that a later monitor can consume:

- Telemetry Ingest records bounded reject/drop/quarantine counters.
- Command Service records ACK absence, result timeouts, and command-family
  outcomes.
- Artifact Upload Broker records upload failures and suppression counters.
- Agent HTTPS auth records certificate reject/revocation/unknown-credential
  counts.
- Edge State Adapter records compact policy/update convergence projection.
- Observability emits coarse CloudWatch metrics and short-retention structured
  logs.

Do not put Platform Health on the hot ingest path. The first real implementation
should be a slow evaluator that reads summarized DB facts, metrics, and support
records on a windowed cadence, then creates internal findings with suppression
and recommended owner/action.

Recommended first shape:

1. Define source fact names while implementing each owning service.
2. Store latest per-device/support projections in App DB where product/support
   reads need them.
3. Emit coarse metrics for fleet trends without high-cardinality dimensions.
4. Later add a scheduled evaluator that creates `platform_health_findings` from
   summarized facts.
5. Keep remediation routed through the owning service; Platform Health
   recommends or requests, but does not directly mutate device lifecycle,
   credentials, policy, backup, update, or artifact state.

Required v0 source signals:

- stale telemetry / connected-but-telemetry-stale
- command no-ACK and command timeout
- policy drift, policy apply failure, stale/expired local policy, and conservative fallback
- artifact upload failure and upload-failure suppression counters
- over-budget/drop/quarantine counts by device/account/family/type
- auth/cert rejection counts for Agent HTTPS
- schema rejection and identity-drift counts

Observability provides raw signals: logs, metrics, traces, counters, health checks, and audit events. Platform Health / Monitoring interprets those signals into internal findings, suppression windows, escalation candidates, and remediation recommendations.

Temporary debug mode is an observability/diagnostics escape hatch, not a
platform-health finding by itself. Platform Health / Monitoring may recommend
that an operator start a debug session, but it must not enable debug mode
directly in v0. Debug mode is routed through Diagnostics, Authorization,
Command Service, Agent HTTPS, and Artifact Upload Broker.

## Scope

In scope:

- platform health findings
- monitor rule definitions and thresholds
- correlation across telemetry ingest, AWS IoT, commands, artifact uploads, device registry, and deployment signals
- runaway device messaging detection
- internal operator-facing incidents or findings
- suppression, cooldown, and dedupe behavior for noisy conditions
- recommendation of remediation actions such as policy refresh, event-family disablement, credential review, or operator investigation

Out of scope:

- customer-visible alert lifecycle
- email/in-app/webhook notification delivery
- raw metrics/log storage
- audit authority
- device lifecycle mutation without an owning service
- cloud authorization decisions
- direct add-on command execution

## Ownership

Platform Health / Monitoring owns the interpretation of cross-platform operational patterns.

It does not own the underlying facts. Source-of-truth facts remain with:

- Telemetry Ingest for accepted/rejected/quarantined runtime messages
- AWS IoT metrics and lifecycle events for transport behavior
- Device Registry for device lifecycle and credential authority
- Publish Policy for allowed device publish behavior
- Command Service for command state and ACK/result lifecycle
- Artifact Upload Broker for artifact intent/upload status
- Deployment for release and environment state
- Observability backend for raw metric/log retention

## Runaway IoT Device Messaging

Initial future monitor case:

```text
device publishes far more messages than its resolved publish policy, historical baseline, or platform safety threshold allows
```

Inputs:

- AWS IoT publish/rule invocation counts by topic family, device, account, and time window
- Telemetry Ingest accepted, rejected, dropped, and quarantined message counts
- `applied_publish_policy_version`, `message_type`, `schema_type`, and
  `schema_version`
- publish-policy violation counters
- sustained over-budget signals from Telemetry Ingest
- `agent_alarm` events such as `potential_abuse_detected`
- AWS IoT lifecycle churn and reconnect rate
- command outcomes for `refresh_publish_policy`
- device registry lifecycle state, including active/suspended/revoked

Candidate detection rules:

- device message rate exceeds policy budget by a configured multiple
- device sends disabled event categories, especially live `ha_event`, after policy refresh
- device emits repeated malformed MQTT5 metadata or unsupported schema versions
- device publishes with stale or unknown `applied_publish_policy_version` outside the convergence window
- accepted-to-dropped ratio indicates local add-on enforcement is failing
- repeated reconnect/publish bursts suggest unstable or abusive runtime behavior
- a single account/site shows correlated spikes across multiple devices

Default response ladder:

1. Record an internal platform health finding.
2. Collapse duplicates by account, site, device, topic family, condition, and time window.
3. Notify internal operations, not customers, unless a product alert policy later says otherwise.
4. Ask Telemetry Ingest or Command Service to request scoped `refresh_publish_policy` when appropriate.
5. If the pattern continues, recommend event-family disablement, publish policy tightening, credential review, or device suspension through the owning service.

Platform Health / Monitoring should not directly revoke credentials or mutate device lifecycle. It recommends or requests through Device Registry, Command Service, Publish Policy, or an operator workflow.

## Other Future Monitor Families

Likely future families:

- IoT lifecycle churn: repeated connect/disconnect/connect_failed patterns
- telemetry ingest lag or receiver saturation
- schema rejection spikes by agent version
- identity drift or topic/device mismatch
- repeated command ACK absence while device appears connected
- artifact upload failure loops
- signed URL misuse or expired-upload spikes
- policy convergence stuck or stale local policy population
- enrollment failure spikes by template, region, or agent version
- release/update failure clusters by version or channel
- backup/diagnostic workflow failure clusters
- DB write latency or dedupe suppression anomalies
- suspicious auth/service-auth failures

## Data Model Sketch

Future table families:

- `platform_monitor_rules`
- `platform_health_findings`
- `platform_health_observations`
- `platform_health_suppressions`
- `platform_health_actions`

A finding should include:

- finding ID
- condition type
- severity
- account/site/device scope, when applicable
- first seen / last seen
- occurrence count
- source signal references
- current state: open, suppressed, monitoring, resolved
- recommended action
- owning service for remediation
- runbook link or operator note

## Boundaries With Alerting

Alerting is product/customer-facing. Platform Health / Monitoring is internal platform protection and operations.

A platform health finding may later create an alert candidate, but only when a product rule explicitly says the customer should see it. Runaway device messaging, policy drift, and artifact recursion are internal by default.

## Boundaries With Telemetry Ingest

Telemetry Ingest owns inline enforcement: validate, drop, count, quarantine, and write current product state. It may trigger small immediate remediations such as rate-limited `refresh_publish_policy`.

Platform Health / Monitoring owns slower correlation across windows, accounts, services, versions, and repeated incidents. It should consume summarized counters and events, not sit on the hot ingest path for v0.

## Acceptance Criteria For Future Implementation

- Platform health findings are separate from customer alerts.
- Runaway device messaging can be detected without reading raw MQTT payload bodies.
- Monitor rules have cooldown/suppression behavior to avoid creating their own storm.
- Findings cite source signals and owning remediation service.
- Automatic remediations are bounded and routed through the owning service.
- Operators can distinguish "device is unhealthy" from "platform is unhealthy" and "customer should be notified."
