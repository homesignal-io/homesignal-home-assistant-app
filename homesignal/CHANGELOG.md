# Changelog

## 0.1.3

- Serve the Web UI at the ingress root to avoid Home Assistant ingress redirects to `/ui`.

## 0.1.2

- Adjust startup log message for update validation.

## 0.1.1

- Fix Home Assistant runtime storage writes to `/config/device.json`.

## 0.1.0

- Add initial local-build Home Assistant app skeleton.
- Add Go agent with health, readiness, version, and UI placeholder routes.
- Add persistent installation identity at `/config/device.json`.
- Add optional `/data/options.json` parsing.
- Add narrow Supervisor/Core API permissions and `addon_config:rw` storage.
