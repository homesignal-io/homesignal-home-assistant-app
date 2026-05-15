# HA Plugin User Journey Report

Date: 2026-05-14

Scope: browser walkthrough of the HomeSignal Home Assistant add-on mock only, including the add-on status page, status states, pairing flow, claim invite verification conditions, paired success view, permissions page, and advanced page.

Audience lens: technical Home Assistant users and integrators. These users are comfortable with Home Assistant, add-ons, switches, status panels, versions, and operational troubleshooting. The UI should be precise and practical, not overly simplified.

## Summary

The HA plugin experience is moving in the right direction. The strongest decision is reducing the product surface to four local add-on surfaces:

- `Status`
- `Pairing`
- `Permissions`
- `Advanced`

The state-specific mock controls are useful for development, but the product itself should feel like one status page with different states, one pairing flow with different conditions inside each step, one local management policy page, and one advanced maintenance page.

The add-on now has a good tone: light Home Assistant styling, clear pairing state, clear “managed by” identity, explicit update warnings, and visible but not theatrical trust details.

## What Works

- The page model is right: `Status`, `Pairing`, `Permissions`, and `Advanced` are the real local add-on concepts.
- The status page states now share one frame:
  - fresh install / not paired
  - healthy / paired
  - disconnected
  - out of date
- The top status line is useful and should stay:
  - `Not paired with HomeSignal`
  - `Paired with HomeSignal cloud`
  - `Disconnected from HomeSignal cloud`
- The `Managed by` card is important. It answers the security/trust question quickly.
- The pairing flow has the right sequence:
  - setup
  - enter claim invite code
  - verify integrator/site/customer details
  - paired
- The claim invite verification step should be modeled as one view with conditions:
  - verifying
  - valid details ready for confirmation
  - rate limited
- The paired success screen feels close: clear success, clear managing organization/site, claimed-by details, return link, portal link, and unpair action.
- `Open HomeSignal portal` as an underlined outbound link feels right. It should not be a local action button.
- `Go to add-on settings` as the settings button label is clearer than `Open add-on settings`.

## What Is Not Working Yet

- The visible mock controls make the add-on feel artificial during review. They are useful for development, but should be hidden or clearly isolated for product screenshots and implementation handoff.
- `Organization` is acceptable for now, but may need a more general label later if the managed entity can be an individual account.
- `Retry pairing` in the disconnected state is understandable but may become `Repair pairing`, `Reconnect`, or `Try again` once the actual recovery behavior is known.
- The remote management permission block is dense by nature. Technical users can handle it, but the default “full remote management” option needs careful wording because it is the highest-trust moment.
- The prior `HomeSignal Portal` helper link from the generated-code flow is no longer the primary path. The local UI should instead accept a portal-created GUID claim invite code.

## Status Page Review

### Fresh Install / Not Paired

Works:

- `Not paired with HomeSignal` is a good neutral state. It does not imply a failure.
- The primary action, `Pair with HomeSignal`, is obvious.
- Health status below gives useful context without blocking the pairing flow.

Recommendation:

- Keep pairing as the primary first-run action.
- Show only first-run relevant health checks unless there is a real blocker.

### Healthy / Paired

Works:

- The top status line, `Paired with HomeSignal cloud`, is clear.
- `Managed by` near the top is the right trust anchor.
- Health status is readable and useful for technical users.
- Update posture and remote management sections are appropriate for a healthy paired status page.

Recommendation:

- Keep this as the baseline status composition.
- Use this frame for every paired status variation.

### Disconnected

Works:

- `Disconnected from HomeSignal cloud` is clear and should remain a top status.
- Keeping `Managed by` visible is important because the add-on is still associated even while disconnected.
- The latest/last connected timestamp is useful.

Needs work:

- Recovery wording is not fully settled. `Retry pairing` may imply a full re-pair when the actual fix might be reconnect, credential refresh, or repair.
- The disconnected alert and health status both say similar things. That is acceptable, but the alert should focus on what the user should do.

Recommendation:

- Keep the warning alert, but make the action match the actual future behavior once known.
- Possible final labels:
  - `Try reconnecting`
  - `Repair pairing`
  - `Start repair flow`

### Out Of Date

Works:

- The add-on remains `Paired with HomeSignal cloud`; the update condition is a warning, not a pairing state. This is correct.
- The severe auto-update alert is well placed above managed identity and health.
- `Go to add-on settings` is the right CTA.

Needs work:

- The warning title should stay visually consistent with the rest of the HA-style UI. It should feel serious, but not like a different design system.
- The copy should avoid implying pairing is degraded unless the version is below minimum supported.

Recommendation:

- Keep update warnings separate from pairing state.
- Later add a distinct state for `Below minimum supported` if cloud management features become disabled.

## Pairing Flow Review

### Setup Step

Works:

- `Pairing setup` is clear.
- The permission review belongs before the claim invite step.
- The full-control default is reasonable for a managed installation, as long as the wording stays explicit.

Needs work:

- Permission labels are dense, but acceptable for a technical Home Assistant audience.
- The “full remote management” text should be reviewed carefully for trust and legal clarity.

Recommendation:

- Keep switches and permission chips.
- Avoid overly friendly security language.
- Make clear that permissions are enforced locally by the add-on.

### Claim Invite Step

Works:

- A single `Claim invite code` frame with conditions is the right model.
- Reserving helper text space avoids layout shift.
- Verification loading is appropriately vague: users only need to know they are waiting.
- Rate limit handling is necessary and understandable.

Needs work:

- Replace generated-code refresh behavior with local claim invite entry.
- Add the confirmation state that shows integrator, creator, site, and customer details before commit.
- Consider whether rate-limited state needs a timestamp or countdown later.

Recommendation:

- Keep these conditions:
  - verifying
  - valid details ready for confirmation
  - rate limited
- Add later:
  - expired
  - already used
  - cloud unavailable

### Paired Success Step

Works:

- The success check and centered heading give the right small amount of ceremony.
- `Managed by` is clear.
- `Claimed by`, email, and device ID provide the right security receipt.
- `Return to add-on status page` is clear.
- `Open HomeSignal portal` as a link is correct.
- `Unpair from HomeSignal` is visible but appropriately muted.

Needs work:

- The organization name has been stepped down visually, which is good. Keep it from becoming another headline.
- If the managing entity can be a person, `Organization` in status and success may need different wording.

Recommendation:

- Keep success as a terminal confirmation view rather than forcing it into the setup/code page template.
- Keep the managed entity unmistakable, but not oversized.

## Recommended Next Pass

1. Hide or isolate mock controls from the product frame.
2. Finalize disconnected recovery language once the real repair behavior is known.
3. Review full remote management copy for trust clarity.
4. Add future code-step conditions:
   - expired code
   - already paired
   - cloud unavailable
5. Decide whether `Organization` should become `HomeSignal account`, `Managed account`, or stay as-is.

## Product Direction

The HA plugin should feel like commissioning equipment inside Home Assistant:

- precise
- calm
- local-first
- clear about who manages the add-on
- explicit about pairing and unpairing
- practical about warnings

The current direction supports that. The main remaining risk is not complexity. Technical users can handle complexity. The risk is mixing dev/mock controls, future implementation notes, or inconsistent state vocabulary into the product surface.
