# V1 Draft Architecture

Status: Future draft, non-authoritative for V0.

Do not use this folder to implement, unblock, deploy, or expand V0. V0 has not
deployed yet, and the V0 canonical docs remain the source of truth for current
implementation and first-deploy work.

## Purpose

Use this folder to explore V1 product and architecture ideas without
contaminating the V0 deployment path.

Appropriate content:

- candidate V1 product scope
- architecture deltas from V0
- V1-only open questions
- future service boundaries or deployment changes
- notes that deliberately should not affect the current V0 build queue

Inappropriate content:

- current V0 acceptance criteria
- first-deploy requirements
- staging or production blockers for V0
- route, schema, service, or infrastructure changes that V0 implementers should
  treat as canonical

## Rules For Agents

- Treat every file in this folder as V1 draft unless it says otherwise.
- If a V1 draft conflicts with current V0 docs, follow the V0 docs for V0 work.
- Do not cite this folder as authority for V0 implementation, deployment, or
  scope decisions.
- If the user asks to promote a V1 decision, update the owning V0 boundary doc
  under `../design-docs/` and `../design-docs/architectural-decision-log.md`;
  do not leave the promoted decision only in this folder.
- Every new V1 file should include `V1 Draft` in the title or opening status.

## Starter Files

- `prioritized-exploration-backlog.md` captures the first V1 priority cut.
- `architecture-deltas.md` captures proposed changes relative to V0.
- `open-questions.md` captures unresolved V1-only decisions.
