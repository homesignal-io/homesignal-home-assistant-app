# UI Data Wiring Reconciliation

Last updated: 2026-05-14

This document reconciles the numeric wiring overlay in `design-mock/src/App.jsx`
against the current architecture docs. Its job is to separate three questions:

1. Does the architecture already contain the data needed for the UI value?
2. Is the API/read-model contract clear enough to implement without guessing?
3. If not, is the missing piece a real architecture gap or just a projection/rule
   that should be owned by the API facade?

The numeric IDs should stay stable while the JSX skeleton evolves. They are a
conversation aid and a future implementation checklist, not database column IDs.

## Legend

| State | Meaning |
| --- | --- |
| Backed | The needed data is already represented in the architecture. Implementation may still need exact field names. |
| Partial | The source data mostly exists, but the read model, rule, priority order, or display projection is not fully contracted. |
| Missing | The UI value depends on a concept that is not yet clearly represented in the architecture. |

Contract clarity is intentionally separate from backing data. A value can be
backed but still need a thin API response shape so the UI does not re-derive
business logic.

## Canonical Source Anchors

- Logical service ownership is in `design-docs/service-map.md:157`,
  `design-docs/service-map.md:158`, `design-docs/service-map.md:160`,
  `design-docs/service-map.md:163`, `design-docs/service-map.md:166`, and
  `design-docs/service-map.md:165`.
- Email alerting and notification ownership are in
  `design-docs/service-map.md:145`, `design-docs/service-map.md:172`,
  `design-docs/service-map.md:272`, and `design-docs/service-map.md:274`.
- Customer/site fields are in `design-docs/account-site-service.md:178`,
  `design-docs/account-site-service.md:184`, `design-docs/account-site-service.md:205`,
  `design-docs/account-site-service.md:208`, and
  `design-docs/account-site-service.md:211`.
- Telemetry/latest-state/presence backing is in
  `design-docs/telemetry-ingest-architecture.md:9`,
  `design-docs/telemetry-ingest-architecture.md:27`,
  `design-docs/telemetry-ingest-architecture.md:28`,
  `design-docs/telemetry-ingest-architecture.md:30`,
  `design-docs/telemetry-ingest-architecture.md:392`,
  `design-docs/telemetry-ingest-architecture.md:406`,
  `design-docs/telemetry-ingest-architecture.md:407`,
  `design-docs/telemetry-ingest-architecture.md:477`,
  `design-docs/telemetry-ingest-architecture.md:820`,
  `design-docs/telemetry-ingest-architecture.md:847`,
  `design-docs/telemetry-ingest-architecture.md:874`, and
  `design-docs/telemetry-ingest-architecture.md:1169`.
- Add-on update backing is in `design-docs/update-architecture.md:430`,
  `design-docs/update-architecture.md:440`, `design-docs/update-architecture.md:442`,
  and `design-docs/update-architecture.md:445`.
- Implementation-plan slices for alert recipients and notification attempts are in
  `design-docs/implementation-plan.md:1409`, `design-docs/implementation-plan.md:1411`,
  `design-docs/implementation-plan.md:1417`, `design-docs/implementation-plan.md:1421`,
  `design-docs/implementation-plan.md:1427`, and
  `design-docs/implementation-plan.md:1428`.

## Dashboard Summary Items

| ID | UI value | State | Contract clarity | Reconciliation |
| --- | --- | --- | --- | --- |
| 1 | Dashboard state label / action required | Backed | Clear | Backing fields exist in presence, backup status, add-on version drift, Home Assistant version drift, storage status, and latest state. API returns computed dashboard state from the v0 issue projection. |
| 2 | Dashboard affected-site count | Backed | Clear | Count distinct affected sites over total managed sites. This belongs in the dashboard summary response. |
| 3 | Dashboard hero copy | Backed | Clear enough | API returns a dashboard state plus primary issue summary; portal copy can render from that small enum/summary without raw-service stitching. |
| 4 | Managed site total | Backed | Clear | Count account-scoped sites. |
| 5 | Online ratio | Backed | Clear | Derived from device presence online/total. Presence is the correct source. |
| 6 | Open issue total | Backed | Clear | Count tripped v0 issue projection rows. |
| 7 | Sites with issues | Backed | Clear | Count distinct sites with one or more v0 issue projection rows. |
| 8 | Backup issues | Backed | Clear | Backup status is owned by Backup Service. V0 issue projection includes `backup_failed` and `backup_overdue`. |
| 9 | Add-on drift | Backed | Clear | Compare reported add-on version/status with desired/latest release state. |
| 10 | Email alerts | Backed | Clear | Alert recipient/preference contract is v0: verified email recipients, enabled subscriptions, optional site scope, and notification attempt metadata. |

## Managed Home Assistant Rows

| ID | UI value | State | Contract clarity | Reconciliation |
| --- | --- | --- | --- | --- |
| 11 | Attention row site name | Backed | Clear | Use account-scoped site display name. |
| 12 | Attention row customer/location | Backed | Clear enough | Customer display name and service-address fields exist. UI can choose city/region as the compact location. |
| 13 | Primary attention issue label | Backed | Clear | Use the first issue row by `sort_priority` from the API issue projection. |
| 14 | Primary attention issue detail | Backed | Clear | Use `detail` from the API issue projection, generated server-side from backing facts. |
| 15 | Attention issue count | Backed | Clear | Count issue projection rows for the site/device. |
| 16 | Expanded issue label | Backed | Clear | Use issue projection `label`. |
| 17 | Expanded issue detail | Backed | Clear | Use issue projection `detail`. |
| 18 | Device row site name | Backed | Clear | Use account-scoped site display name. |
| 19 | Device row customer/location | Backed | Clear enough | Same customer/location source as ID 12. |
| 20 | Device connection label / dot | Backed | Clear | Use `device_presence` online/offline/disconnected state. The UI label can say Connected/Disconnected. |
| 21 | Device status detail | Backed | Clear enough | Home Assistant version and backup status are in the latest-state/backup summary path. The API should expose them together on the device list row. |
| 22 | Home Assistant update status | Backed | Clear | Installed version is backed by telemetry; latest version comes from a small cloud catalog/cache and is advisory only. Hide when source is unavailable. |
| 23 | Device row action | Backed | Clear | "Review" vs "View" comes from the issue projection's primary action/rule. |
| 24 | Devices page fleet count | Backed | Clear | Count devices and sites visible to the account/user. |
| 30 | Site/home icon | Backed | Clear | `site_category` is optional presentation-only Account/Site data. UI shows inline icon before site name and falls back to the default Home Assistant/site icon. |
| 31 | Issue severity / indicator color | Backed | Clear | Use issue projection `severity`: `critical`, `warning`, or `info`. |
| 32 | Add-on update drift | Backed | Clear | Add-on desired/reported version comparison is explicitly part of update architecture. |

## Activity Timeline

| ID | UI value | State | Contract clarity | Reconciliation |
| --- | --- | --- | --- | --- |
| 25 | Activity time | Backed | Clear | Use `occurred_at` from the API activity feed row. |
| 26 | Activity action | Backed | Clear | Use normalized activity `action`. |
| 27 | Activity subject | Backed | Clear | Use subject type, subject ID, and subject label from the activity feed row. |
| 28 | Activity detail | Backed | Clear | Use server-generated or enum-driven activity detail text. |
| 29 | Activity category | Backed | Clear | Use v0 categories: alert, backup, device, update, enrollment, and account. |

## Cross-Item Contract Fixes

These product calls are now locked for v0.

1. Add a dashboard/device-list issue projection.
   - Covers IDs 1, 3, 6, 7, 13, 14, 15, 16, 17, 23, and 31.
   - Suggested shape: `issue_code`, `severity`, `label`, `detail`, `site_id`,
     `device_id`, `source_area`, `sort_priority`, `primary_action`.
   - This is API facade/read-model work, not a new domain service.
   - V0 issue codes: `device_disconnected`, `backup_failed`,
     `backup_overdue`, `addon_update_attention`, `ha_update_advisory`, and
     `storage_warning`.
   - Judgment call: calculate issue rows server-side so Dashboard and Devices
     never disagree about what needs review.

2. Add an activity feed projection.
   - Covers IDs 25, 26, 27, 28, and 29.
   - Suggested shape: `occurred_at`, `category`, `action`, `subject_type`,
     `subject_id`, `subject_label`, `detail`, `severity`, `actor_label`.
   - It can adapt from audit, lifecycle, backup, update, alert, and telemetry
     events without declaring one new canonical event store.
   - Judgment call: public activity is a product timeline; internal
     platform-health/debug/provider noise stays out unless intentionally exposed
     in internal admin.

3. Name the alert recipient/preferences contract.
   - Covers ID 10.
   - Suggested minimum: recipient email, display label, verification/status,
     channel, enabled flag, subscribed alert families, created/updated stamps,
     and last notification attempt summary.
   - Judgment call: recipient verification is required before delivery unless
     the address is the authenticated user's already-verified email.

4. Add the Home Assistant latest-version source.
   - Covers ID 22.
   - Installed version is already backed. A small cloud catalog/cache supplies
     latest stable version for advisory display.
   - Judgment call: hide the advisory when source data is missing/stale instead
     of warning from weak data.

5. Keep site icon variation presentation-only.
   - Covers ID 30.
   - Optional `site_category` may drive an inline icon, but absence is fine and
     defaults to the standard Home Assistant/site icon.
   - Judgment call: do not let icon category become an authorization, billing,
     lifecycle, or device-placement concept.

## Implementation Guidance

The UI should not stitch these values from many service endpoints. For v0, the
API facade should return dashboard, device-list, and activity-feed read models
that are already scoped to the authenticated account/user. Those read models can
be backed by Postgres projections owned by the appropriate logical services.

Do not reopen settled architecture because a wiring item is yellow. Yellow means
the contract needs a concise response shape or rule list; it does not imply a new
service, a new transport, or a new authority boundary.
