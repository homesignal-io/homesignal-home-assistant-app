-- +goose Up
CREATE TABLE publish_policy_catalog (
  policy_version text PRIMARY KEY,
  display_name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  is_default boolean NOT NULL DEFAULT false,
  telemetry_cadence_seconds integer NOT NULL,
  refresh_after_seconds integer NOT NULL,
  expires_after_seconds integer NOT NULL,
  enabled_event_families text[] NOT NULL DEFAULT '{}',
  policy_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT publish_policy_catalog_status_check CHECK (status IN ('draft', 'active', 'retired')),
  CONSTRAINT publish_policy_catalog_telemetry_cadence_check CHECK (telemetry_cadence_seconds > 0),
  CONSTRAINT publish_policy_catalog_refresh_check CHECK (refresh_after_seconds > 0),
  CONSTRAINT publish_policy_catalog_expiry_check CHECK (expires_after_seconds >= refresh_after_seconds)
);

CREATE UNIQUE INDEX publish_policy_catalog_one_default_active_idx
  ON publish_policy_catalog(is_default)
  WHERE is_default AND status = 'active';

INSERT INTO publish_policy_catalog (
  policy_version,
  display_name,
  status,
  is_default,
  telemetry_cadence_seconds,
  refresh_after_seconds,
  expires_after_seconds,
  enabled_event_families,
  policy_json
)
VALUES (
  'ppv_v0_default_free',
  'V0 default free publish policy',
  'active',
  true,
  3600,
  86400,
  604800,
  ARRAY['agent_alarm'],
  '{
    "policy_version": "ppv_v0_default_free",
    "telemetry_cadence_seconds": 3600,
    "refresh_after_seconds": 86400,
    "expires_after_seconds": 604800,
    "enabled_event_families": ["agent_alarm"],
    "disabled_event_families": ["ha_event"],
    "event_budgets": {
      "agent_alarm": {
        "max_events": 12,
        "window_seconds": 3600,
        "max_payload_bytes": 5120
      },
      "ha_event": {
        "max_events": 0,
        "window_seconds": 3600,
        "max_payload_bytes": 0
      }
    },
    "observability": {
      "runtime_log_level": "warning",
      "diagnostic_excerpt_max_bytes": 5120,
      "routine_log_summary_enabled": true
    }
  }'::jsonb
);

INSERT INTO audit_events (
  actor_subject_type,
  actor_subject_id,
  action,
  result,
  metadata_json
)
VALUES (
  'system',
  'migration:000003',
  'publish_policy.seed_default',
  'success',
  '{"policy_version": "ppv_v0_default_free", "is_default": true}'::jsonb
);

-- +goose Down
DELETE FROM audit_events
WHERE actor_subject_type = 'system'
  AND actor_subject_id = 'migration:000003'
  AND action = 'publish_policy.seed_default';

DROP INDEX IF EXISTS publish_policy_catalog_one_default_active_idx;
DROP TABLE IF EXISTS publish_policy_catalog;
