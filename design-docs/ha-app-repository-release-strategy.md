# Home Assistant App Repository And Release Strategy

Status: V0 architecture guidance.

This document defines how HomeSignal publishes the Home Assistant app, how
staging/non-prod differs from production release tracks, and how cloud rollout
intent relates to Home Assistant Supervisor update behavior.

## Home Assistant Constraints

Home Assistant apps are distributed through app repositories that users add to
Supervisor by repository URL. A repository can contain one or more apps, each in
its own folder, and the repository root contains `repository.yaml`.

Relevant Home Assistant documentation:

- App repository shape and repository URL install:
  https://developers.home-assistant.io/docs/apps/repository/
- App `config.yaml`, `slug`, `version`, `image`, `stage`, and permissions:
  https://developers.home-assistant.io/docs/apps/configuration/
- Publishing pre-built app containers and multi-arch images:
  https://developers.home-assistant.io/docs/apps/publishing/
- Supervisor store/install/update endpoints:
  https://developers.home-assistant.io/docs/api/supervisor/endpoints/

Architecture consequences:

- Public app repositories and public app config cannot be treated as secret.
- Staging or candidate URLs embedded in app metadata are non-secret routing
  hints, not security boundaries.
- Durable authority lives in HomeSignal backend claim/session validation,
  device credentials, audit, authorization, rate limits, and local app policy.
- Home Assistant Supervisor owns local app install/update execution. HomeSignal
  can publish artifacts, expose desired version/channel intent, observe reported
  state, and request bounded update/status operations only where local policy
  and Supervisor permissions allow it.
- Home Assistant still exposes compatibility literals that use historical
  add-on names, including `addon_config:rw`, My Home Assistant redirect ids,
  ingress paths, and some Supervisor API paths. Keep those exact strings where
  required by the platform; use "app" for HomeSignal product copy and internal
  schema naming.

## Source Repo Vs Distribution Repos

HomeSignal has one product source repository:

```text
home-signal
```

That source repo owns:

- backend services
- the Home Assistant app source under `homesignal/`
- design docs and contracts
- build scripts
- release metadata generation

Home Assistant app distribution repositories are packaging outputs, not product
source-of-truth repositories. They should be thin public repos generated or
updated by CI from the source repo.

Preferred public distribution repos:

| Repo | Audience | Cloud environment | Release track | Notes |
| --- | --- | --- | --- | --- |
| `homesignal-home-assistant-app` | Normal customers/integrators | production | stable | Default public install path. |
| `homesignal-home-assistant-app-candidate` | Opt-in production test cohort | production | candidate | Production data path, ahead of stable. |
| `homesignal-home-assistant-app-staging` | Internal/non-prod testing | staging | staging/dev | Points at staging cloud only. |

The candidate repo is still a production repo because it pairs to production
HomeSignal, uses production account/site/device authority, and handles real
customer data. Its difference is release risk, not environment.

## App Identity Defaults

Each track should use a distinct app slug and image name so Supervisor can
reason about installed versions without ambiguity.

| Track | App name | Suggested slug | Suggested image |
| --- | --- | --- | --- |
| stable | `HomeSignal Manager` | `homesignal_manager` | `ghcr.io/homesignal-io/homesignal-manager` |
| candidate | `HomeSignal Manager Candidate` | `homesignal_manager_candidate` | `ghcr.io/homesignal-io/homesignal-manager-candidate` |
| staging | `HomeSignal Manager Staging` | `homesignal_manager_staging` | `ghcr.io/homesignal-io/homesignal-manager-staging` |

The production stable and production candidate apps both point at the production
HomeSignal cloud profile. The staging app points at the staging profile. None of
these profiles contains secrets; all real authority is created during claim.

Do not expose a freeform cloud URL field in the local app. Environment/profile
selection is a build/package/channel concern, not an end-user setting.

## CI/CD Build Matrix

CI should build the Home Assistant app as a matrix from the source repo:

| Matrix dimension | Values |
| --- | --- |
| `cloud_environment` | `staging`, `production` |
| `release_track` | `stable`, `candidate`, `staging` |
| `ha_app_slug` | track-specific slug |
| `ha_app_name` | track-specific display name |
| `cloud_profile` | `staging` or `production` |
| `image_name` | track-specific GHCR image |

Rules:

- Build pre-built multi-arch images for public distribution.
- `config.yaml` `version` must match the published image tag for that app
  package.
- The same source commit may produce more than one package, but the package
  metadata must make the track obvious.
- CI updates the thin distribution repo after image publish succeeds.
- Distribution repo commits should be generated and traceable to source commit,
  image digest, version, environment, and track.
- Staging/non-prod packages are allowed to point at staging domains only after
  Stage 1 domain-backed staging exists.

Suggested first CI flow:

```text
source tag/commit
  -> test/build Home Assistant app
  -> build multi-arch image(s)
  -> publish image(s) to GHCR
  -> render track-specific config.yaml/repository.yaml/README
  -> push generated package contents to distribution repo
  -> record release metadata in HomeSignal release catalog
```

AWS CodeBuild remains the preferred runner for AWS-heavy backend deploys. GitHub
Actions may be used for Home Assistant app image publishing when it is the
lowest-friction way to use the Home Assistant builder actions, but deployment
authority for HomeSignal cloud environments remains script-first and
AWS-oriented.

## Rollout And Cohort Control

HomeSignal cannot silently force an arbitrary percentage of existing Home
Assistant installs onto a different repository. Supervisor updates the app that
is installed from the repository/channel the local Home Assistant already has,
subject to local update settings and policy.

Use these mechanisms instead:

### New Installs

The public pairing/invite flow chooses which app repository link to show:

- default customers receive the stable production repo
- opt-in test customers or dogfood integrators receive the candidate production
  repo
- internal/non-prod users receive the staging repo

This is the primary way to steer cohorts at install time.

### Existing Installs

For an already installed app:

- publishing a version to the stable repo makes that version available to the
  stable population
- publishing a version to the candidate repo makes that version available to the
  candidate population
- moving a site from stable to candidate is a local installation/channel change,
  not a silent cloud flip
- the app reports installed slug, repository/source when available, track,
  version, update status, and auto-update posture

Cloud may recommend a track change, show instructions, or issue a bounded local
update/status command where allowed. It must not rely on secret repository URLs
or hidden app URL overrides for cohort assignment.

### Percent Rollout

A true "20% of production" binary rollout is not safe from a single public
stable repository, because once a version is published there, every auto-updated
stable install may see it according to Supervisor behavior.

Production rollout defaults:

1. publish to staging package for non-prod validation
2. publish to production candidate package for opt-in production canaries
3. observe health, update success, crash/disconnect rates, and support signals
4. publish to production stable package only when ready for the stable fleet

Cloud feature flags can progressively enable server-side behavior after a
binary is broadly installed, but feature flags do not replace candidate packages
for testing app binary installation/update safety.

## Supervisor Update Ownership

Home Assistant Supervisor is the local installer/updater. HomeSignal owns:

- artifact publication
- release catalog
- desired version/channel intent
- rollout cohort records
- update readiness policy
- update status interpretation
- user/integrator UX
- audit trail

The local app owns:

- reporting installed version, repository/source when available, auto-update
  posture, boot/watchdog posture, and update availability
- showing update warnings/action-required states
- optionally requesting Supervisor update only through supported Supervisor
  APIs, local permission policy, and explicit product rules

V0 should treat command-driven app update execution as optional and conservative.
If `/store/addons/self/update` is unavailable, denied, or unsafe in staging
validation, the product falls back to clear UI instructions and Home Assistant
settings links rather than pretending cloud can force the update.

## Product State To Track

Track these fields once release management exists:

| Field | Purpose |
| --- | --- |
| `ha_app_slug` | Distinguishes stable/candidate/staging installed package. |
| `ha_app_repository_source` | Detects expected repository/channel when Supervisor exposes it. |
| `cloud_environment_profile` | `production` or `staging`; must match claim environment. |
| `release_track` | `stable`, `candidate`, or `staging`. |
| `installed_version` | Reported by the app/Supervisor. |
| `latest_available_version` | From Supervisor/store or HomeSignal release catalog. |
| `desired_version` | Cloud rollout intent for this device/cohort. |
| `auto_update` | Local Supervisor setting when readable. |
| `update_available` | Local Supervisor/store state when readable. |
| `last_update_status` | Current, pending, blocked, failed, or rolled back. |

Claim validation should reject staging app packages against production claims
and production app packages against staging claims unless an explicit local-dev
fixture is being used.

## Open Implementation Details

- Exact GitHub organization/repo names for generated candidate and staging
  distribution repositories.
- Whether generated distribution repos are pushed by GitHub Actions, CodeBuild,
  or a hybrid pipeline.
- Whether candidate and staging repos are public but unadvertised, or public and
  clearly documented for opt-in users.
- Whether v0 app can safely call Supervisor update for itself in the target Home
  Assistant versions. Validate in staging before productizing remote update
  commands.
