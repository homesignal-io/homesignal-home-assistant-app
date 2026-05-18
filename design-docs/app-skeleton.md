# HomeSignal Home Assistant App Skeleton

## Summary
Create a local-build Home Assistant app under `apps/homesignal` named **HomeSignal** with slug `homesignal`. It will run a small Go agent with health/status/version/UI routes, persistent local identity in `/config/device.json`, optional options parsing from `/data/options.json`, and narrow Home Assistant Supervisor/Core API permissions.

This is only the HA app skeleton. It will not implement enrollment, cloud auth, telemetry, topology discovery, backup actions, update orchestration, IoT Core, or command execution.

## Key Changes
- Add app package files: `config.yaml`, `Dockerfile`, `README.md`, `DOCS.md`, `CHANGELOG.md`, `translations/en.yaml`, `Makefile`, and `cmd/agent/main.go`.
- `config.yaml` will define:
  - `name: HomeSignal`
  - `slug: homesignal`
  - `version: 0.1.0`
  - `description`
  - `arch: [amd64, aarch64]`
  - `init: false`
  - `startup: services`
  - `boot: auto`
  - `hassio_api: true`
  - `homeassistant_api: true`
  - `map: [addon_config:rw]`
  - ingress enabled on the Go server port.
- Do not include a production `image` field yet; Home Assistant local install should build from the folder/Dockerfile.
- Do not request `privileged`, `host_network`, `docker_api`, `full_access`, Docker socket access, or broad host/Home Assistant filesystem mounts.
- Implement Go routes:
  - `/healthz`: process liveness.
  - `/readyz`: identity/config readiness plus degraded Supervisor/Core API status.
  - `/version`: `version`, `commit`, `buildTime`.
  - `/ui`: plain HTML status/pairing placeholder.
- On first boot, create `/config/device.json` with generated `installation_id`; reuse it on restart.
- Read `/data/options.json` if present and tolerate it being absent.
- Detect `SUPERVISOR_TOKEN`; do not fail boot if missing. Use it only to configure a placeholder client for `http://supervisor/core/api/`.
- Dockerfile uses Go builder to minimal non-root runtime, with ldflag injection for version metadata.

## Test Plan
- Add Go tests for:
  - missing options file succeeds.
  - valid options JSON parses.
  - `device.json` is created on first boot.
  - restart reuses the same `installation_id`.
  - `/healthz` succeeds.
  - `/version` returns version metadata.
  - `/readyz` reports degraded status when `SUPERVISOR_TOKEN` is missing.
- `make test` should work with host Go when available and through Docker when host Go is missing.
- `make build`, `make docker-build`, and `make run-agent` are local/dev targets only.

## Assumptions
- No production registry image or publish pipeline in Feature 1.
- No storage fallback: require `addon_config:rw` and `/config`.
- Missing `SUPERVISOR_TOKEN` means degraded API access, not failed startup.
- Current Home Assistant app config behavior should continue to follow the official Home Assistant package configuration and testing guidance.
