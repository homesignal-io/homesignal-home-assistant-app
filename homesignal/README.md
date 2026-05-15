# HomeSignal

HomeSignal is a local-build Home Assistant add-on skeleton for the HomeSignal agent. This version provides local identity, enrollment state, status endpoints, an ingress UI, and Supervisor/Core API permission wiring.

## Install Locally

1. Copy or clone this repository into a Home Assistant add-on repository location.
2. In Home Assistant, go to **Settings > Add-ons > Add-on Store**.
3. Add the local repository path or refresh the local add-on repository.
4. Install the **HomeSignal** add-on.
5. Start the add-on and open its Web UI.

This skeleton intentionally omits a production `image` field in `config.yaml`, so Home Assistant can build it from this add-on folder.

## Endpoints

- `/healthz`: process liveness.
- `/readyz`: identity/config readiness and degraded Supervisor/Core API status.
- `/status`: local enrollment state and non-secret device metadata.
- `/version`: build metadata.
- `/ui`: Home Assistant ingress page showing pairing, claimed, or revoked state.

## Permissions

The add-on requests only:

- `hassio_api: true`
- `homeassistant_api: true`
- `addon_config:rw`

It does not request Docker access, host networking, privileged mode, full access, the Docker socket, or broad Home Assistant filesystem mappings.

The container currently runs with the image default user so it can write Home Assistant's mounted `/config` add-on storage path.

## Storage

The agent stores add-on-owned data in `/config`, backed by Home Assistant's `addon_config:rw` mapping. On first boot it creates:

```text
/config/device.json
```

The file contains a generated `installation_id`, enrollment state, and non-secret credential metadata. Poll tokens, private keys, certificates, and temporary AWS claim material are stored separately as `0600` files under add-on-owned `/config` subdirectories and are not exposed through JSON endpoints.

## Current Limitations

This release implements the add-on side of enrollment and documents the future cloud/AWS IoT Core contract in `design-docs/enrollment-claiming-contract.md`. It does not implement the SaaS backend, portal UI, MQTT runtime, telemetry, topology discovery, backup actions, update orchestration, full release flow, or command execution.
