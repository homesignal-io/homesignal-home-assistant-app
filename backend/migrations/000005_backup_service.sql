-- +goose Up
CREATE TABLE backup_policies (
  device_id text PRIMARY KEY REFERENCES devices(device_id) ON DELETE CASCADE,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  status text NOT NULL DEFAULT 'enabled',
  expected_cadence_seconds integer NOT NULL DEFAULT 86400,
  overdue_after_seconds integer NOT NULL DEFAULT 172800,
  offsite_enabled boolean NOT NULL DEFAULT false,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT backup_policies_status_check CHECK (status IN ('enabled', 'disabled')),
  CONSTRAINT backup_policies_cadence_check CHECK (expected_cadence_seconds > 0),
  CONSTRAINT backup_policies_overdue_check CHECK (overdue_after_seconds >= expected_cadence_seconds)
);

CREATE TABLE backup_runs (
  backup_id text PRIMARY KEY,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  device_id text NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
  command_id text REFERENCES commands(command_id) ON DELETE SET NULL,
  artifact_upload_id text REFERENCES artifact_uploads(upload_id) ON DELETE SET NULL,
  status text NOT NULL,
  requested_by_user_id text REFERENCES users(user_id) ON DELETE SET NULL,
  reason_code text,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  requested_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT backup_runs_status_check CHECK (status IN ('requested', 'running', 'succeeded', 'failed', 'timed_out', 'canceled'))
);

CREATE INDEX backup_runs_device_requested_idx ON backup_runs(device_id, requested_at DESC);
CREATE UNIQUE INDEX backup_runs_command_unique_idx ON backup_runs(command_id) WHERE command_id IS NOT NULL;
CREATE INDEX backup_runs_artifact_upload_idx ON backup_runs(artifact_upload_id) WHERE artifact_upload_id IS NOT NULL;

CREATE TABLE device_backup_status (
  device_id text PRIMARY KEY REFERENCES devices(device_id) ON DELETE CASCADE,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  current_status text NOT NULL DEFAULT 'unknown',
  last_backup_id text REFERENCES backup_runs(backup_id) ON DELETE SET NULL,
  in_progress_backup_id text REFERENCES backup_runs(backup_id) ON DELETE SET NULL,
  last_success_at timestamptz,
  last_failure_at timestamptz,
  overdue_after timestamptz,
  artifact_upload_id text REFERENCES artifact_uploads(upload_id) ON DELETE SET NULL,
  reason_code text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT device_backup_status_current_check CHECK (current_status IN ('unknown', 'requested', 'running', 'succeeded', 'failed', 'timed_out', 'canceled', 'overdue'))
);

CREATE INDEX device_backup_status_site_status_idx ON device_backup_status(site_id, current_status);

-- +goose Down
DROP INDEX IF EXISTS device_backup_status_site_status_idx;
DROP TABLE IF EXISTS device_backup_status;
DROP INDEX IF EXISTS backup_runs_artifact_upload_idx;
DROP INDEX IF EXISTS backup_runs_command_unique_idx;
DROP INDEX IF EXISTS backup_runs_device_requested_idx;
DROP TABLE IF EXISTS backup_runs;
DROP TABLE IF EXISTS backup_policies;
