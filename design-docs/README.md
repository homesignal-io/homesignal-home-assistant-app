# HomeSignal Design Docs

This folder is the current V0 architecture workspace for HomeSignal.

## Current V0 Sources

Unless a file is explicitly labeled as future or V1 draft, docs under
`design-docs/` describe the current V0 architecture and first-deploy planning
surface.

Start with:

- `architectural-decision-log.md` for settled decisions and canonical doc
  routing
- `implementation-plan.md` for V0 implementation sequencing
- boundary docs such as `api-facade.md`, `service-map.md`, and
  `workstreams/deployment.md` for the current V0 architecture

Do not pull requirements from future draft folders into V0 implementation,
first-deploy work, staging, or production.

## Future V1 Drafts Live Elsewhere

V1 architecture exploration lives in the sibling top-level folder
`../v1-draft-architecture/`, outside this V0 design-doc folder.

Docs in that folder are future-only and non-authoritative for V0. They may
reference V0 docs as a baseline, but they do not change V0 route contracts,
service ownership, deployment readiness, implementation scope, or non-goals
unless a later explicit promotion updates the owning V0 doc and
`architectural-decision-log.md`.

## Promotion Rule

To promote a V1 idea into current architecture:

1. The user must explicitly ask to promote or adopt it.
2. Update the owning V0 boundary doc instead of treating the V1 draft as
   canonical.
3. Add or update the receipt in `architectural-decision-log.md`.
4. Reconcile affected implementation-plan or workstream docs.
