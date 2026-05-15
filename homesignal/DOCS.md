# HomeSignal Add-On Notes

## Runtime

The agent is a single Go HTTP server listening on port `8099` by default. Home Assistant ingress is wired to `/ui`.

The container does not request privileged mode, host networking, Docker access, full access, or broad host filesystem mounts. It currently runs the agent as the image default user because Home Assistant owns the mounted `/config` add-on storage path, and the agent must be able to create `/config/device.json` on first boot.

## Identity

On startup, the agent ensures `/config/device.json` exists. If the file is missing, it writes a generated UUIDv4-style `installation_id` and schema-v2 enrollment metadata. If the file exists, the existing ID is reused and legacy identity-only files are migrated.

The add-on uses only the `addon_config:rw` mapping for persistent add-on-owned files. There is no fallback to broad Home Assistant config mounts.

Enrollment secrets are kept outside `device.json` as `0600` files under add-on-owned `/config` subdirectories:

```text
/config/secrets/poll_token
/config/secrets/aws_claim.crt
/config/secrets/aws_claim.key
/config/iot/device.key
/config/iot/device.crt
```

`device.json` stores only metadata and file paths. Secret contents are not returned by `/status`, `/readyz`, or `/ui`.

## Options

The agent attempts to read `/data/options.json`. A missing file is accepted and treated as empty configuration. Invalid JSON is an initialization error because it means Supervisor provided malformed options.

## Supervisor And Core API

The add-on requests `hassio_api` and `homeassistant_api` permissions. Home Assistant Supervisor injects `SUPERVISOR_TOKEN` when those APIs are available.

Feature 1 only detects whether the token is present and prepares a placeholder Core API client for:

```text
http://supervisor/core/api/
```

The token is never displayed, persisted, or required for local boot. Missing token produces degraded readiness so local tests and development remain possible.

## Readiness

`/healthz` reports process liveness.

`/readyz` reports initialized local state, enrollment state, and Supervisor/Core API availability. Missing `SUPERVISOR_TOKEN` returns HTTP 200 with `degraded: true`; local storage or identity initialization failures prevent startup.

`/status` reports local enrollment state and non-secret metadata such as `installation_id`, `claim_state`, pairing-code expiry, configured cloud endpoint status, `device_id`, and IoT Thing name. It never returns pairing codes, poll tokens, private keys, certificate contents, or temporary AWS claim material.
