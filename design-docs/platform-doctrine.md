# HomeSignal Platform Doctrine

This document defines the operating philosophy for HomeSignal platform work. It is not a service spec, build plan, or backlog. It explains how agents and humans should make decisions when service architecture, cross-cutting policy, and local implementation plans overlap.

## Agent Use

Read this before creating or revising service architecture, cross-cutting workstream docs, or implementation plans.

Agents should use this document to decide:

- which doc owns a decision
- whether a concern is service-local or cross-cutting
- when a local plan must reconcile with a platform invariant
- how to record a temporary implementation exception without weakening the architecture

Do not use this document as a substitute for the concrete service docs. Use it as the decision frame around them.

## Decision Order

When docs overlap, apply this order:

1. Explicit user direction in the current task.
2. Platform doctrine for decision philosophy and ownership.
3. Cross-cutting workstream docs for shared invariants.
4. Service architecture docs for service responsibility and contracts.
5. Local implementation plans for concrete sequencing and code tasks.

If a local implementation plan conflicts with a cross-cutting invariant, do not silently choose one. Either update the local plan to satisfy the invariant or record a deliberate exception with scope, reason, and follow-up.

## Core Philosophy

HomeSignal should be built as a set of small, owned services around a few strong platform invariants.

The goal is not to maximize the number of services. The goal is to keep authority, data ownership, operational behavior, and failure boundaries explicit enough that future implementation becomes routine.

Prefer:

- explicit service ownership over shared ambiguous logic
- durable product identities over transport/session identifiers
- narrow contracts over broad catch-all integration points
- production-shaped environments over hand-built special cases
- local device safety over cloud convenience
- auditable authority changes over implicit side effects
- boring deployment paths over clever manual procedures

Avoid:

- treating AWS transport identity as HomeSignal product authority
- spreading authorization decisions into route handlers or service-specific ad hoc checks
- creating broad MQTT topic rules before the exact runtime contract is known
- allowing service plans to invent their own logging, secrets, deploy, or test conventions
- using the add-on as a hidden privileged remote shell
- letting future architecture claims masquerade as implemented behavior

## Architecture Versus Implementation

Docs should distinguish four states:

| State | Meaning |
| --- | --- |
| Target architecture | How the system should work when fully built. |
| Implementation default | The preferred first concrete path unless a later decision changes it. |
| Current implementation | What exists in code or deployed infrastructure now. |
| Temporary exception | A known deviation with a reason, scope, and removal condition. |

Agents should preserve this distinction. Do not rewrite target architecture merely because the first implementation is smaller. Instead, state the smaller implementation as a staged default or temporary exception.

## Services And Cross-Cutting Work

Some concerns are both a service and a platform policy. Identity and authorization, device lifecycle, deployment, and observability all have service-local implementation surfaces and platform-wide obligations.

For v0, logical services are primarily ownership boundaries, not a mandate to deploy many microservices. The control plane should be implemented as one API/domain monolith unless a service spec explicitly says otherwise. Telemetry Ingest is the v0 exception: it should be a separately deployable service because it sits on the runtime device-message path and has different scaling, failure, and AWS IoT integration concerns. Non-functional deployment readiness is tracked by `workstreams/deployment-readiness-matrix.md`; it is an architecture checklist, not evidence that CI/CD or infrastructure already exists.

When a concern spans services:

- keep the cross-cutting policy separate from local plans
- define invariants once
- have each affected service plan state how it satisfies those invariants
- reconcile conflicts explicitly before implementation

Cross-cutting work should define what must be true. Service plans should define what will be built locally.

## Reconciliation Standard

Before implementing work that touches a service boundary, durable identity, secrets, deployment, topics, observability, tests, or local/cloud authority, agents must answer:

- Which cross-cutting workstreams apply?
- Which service owns the durable state or authority?
- Which contracts are affected?
- Does the local plan satisfy the shared invariants?
- Is this target architecture, implementation default, current behavior, or a temporary exception?
- What verification proves the work did not break the invariant?

For deployment, CI/CD, staging, production, secrets/config, alarms, smoke tests,
or launch runbooks, reconcile against
`workstreams/deployment-readiness-matrix.md` before adding local rules.

If the answer is unclear, update the docs before or alongside the implementation.

## Acceptance Criteria

- New service plans identify applicable cross-cutting workstreams.
- Cross-cutting docs define invariants, affected surfaces, and reconciliation checks.
- Service docs stay concrete and do not duplicate platform policy unless the local behavior is service-specific.
- Temporary exceptions include a removal condition.
- Agents can determine which docs to read before making implementation changes.
