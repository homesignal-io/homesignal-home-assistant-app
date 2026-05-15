# Contract Fixtures

Fixtures under this directory are shared boundary examples. They should use
fixed timestamps, stable opaque IDs, and no secrets.

Runtime telemetry/event fixtures live under `runtime/` and are expected to pass
the telemetry-ingest schema catalog before any product-state write happens.
