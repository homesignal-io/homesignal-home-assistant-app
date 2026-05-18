-- +goose Up
CREATE TABLE debug_sessions (
  debug_session_id text PRIMARY KEY,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  device_id text NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
  command_id text REFERENCES commands(command_id) ON DELETE SET NULL,
  status text NOT NULL DEFAULT 'active',
  requested_by_subject_type text NOT NULL,
  requested_by_subject_id text NOT NULL,
  redaction_profile text NOT NULL,
  max_bytes bigint NOT NULL,
  categories text[] NOT NULL DEFAULT '{}',
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  ended_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT debug_sessions_status_check CHECK (status IN ('active', 'stopped', 'expired')),
  CONSTRAINT debug_sessions_actor_type_check CHECK (requested_by_subject_type IN ('internal', 'support')),
  CONSTRAINT debug_sessions_max_bytes_check CHECK (max_bytes > 0),
  CONSTRAINT debug_sessions_expiry_check CHECK (expires_at > started_at)
);

CREATE INDEX debug_sessions_device_status_idx ON debug_sessions(device_id, status);
CREATE INDEX debug_sessions_site_created_idx ON debug_sessions(site_id, created_at DESC);
CREATE INDEX debug_sessions_command_idx ON debug_sessions(command_id) WHERE command_id IS NOT NULL;

CREATE TABLE diagnostic_bundles (
  diagnostic_bundle_id text PRIMARY KEY,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  device_id text NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
  debug_session_id text REFERENCES debug_sessions(debug_session_id) ON DELETE SET NULL,
  command_id text REFERENCES commands(command_id) ON DELETE SET NULL,
  artifact_upload_id text REFERENCES artifact_uploads(upload_id) ON DELETE SET NULL,
  status text NOT NULL DEFAULT 'requested',
  purpose text NOT NULL,
  redaction_profile text NOT NULL,
  reason_code text,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  requested_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT diagnostic_bundles_status_check CHECK (status IN ('requested', 'uploaded', 'failed', 'canceled')),
  CONSTRAINT diagnostic_bundles_purpose_check CHECK (purpose IN ('diagnostic_bundle', 'debug_bundle', 'error_log_bundle'))
);

CREATE INDEX diagnostic_bundles_device_requested_idx ON diagnostic_bundles(device_id, requested_at DESC);
CREATE INDEX diagnostic_bundles_debug_session_idx ON diagnostic_bundles(debug_session_id) WHERE debug_session_id IS NOT NULL;
CREATE INDEX diagnostic_bundles_command_idx ON diagnostic_bundles(command_id) WHERE command_id IS NOT NULL;
CREATE INDEX diagnostic_bundles_artifact_upload_idx ON diagnostic_bundles(artifact_upload_id) WHERE artifact_upload_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS diagnostic_bundles_artifact_upload_idx;
DROP INDEX IF EXISTS diagnostic_bundles_command_idx;
DROP INDEX IF EXISTS diagnostic_bundles_debug_session_idx;
DROP INDEX IF EXISTS diagnostic_bundles_device_requested_idx;
DROP TABLE IF EXISTS diagnostic_bundles;
DROP INDEX IF EXISTS debug_sessions_command_idx;
DROP INDEX IF EXISTS debug_sessions_site_created_idx;
DROP INDEX IF EXISTS debug_sessions_device_status_idx;
DROP TABLE IF EXISTS debug_sessions;
