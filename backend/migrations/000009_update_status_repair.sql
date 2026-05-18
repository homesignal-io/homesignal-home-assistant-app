-- +goose Up
CREATE TABLE device_update_status (
  device_id text PRIMARY KEY REFERENCES devices(device_id) ON DELETE CASCADE,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  channel_key text REFERENCES release_channels(channel_key) ON DELETE SET NULL,
  ha_app_slug text,
  ha_app_repository_source text,
  cloud_environment_profile text,
  release_track text,
  installed_version text,
  desired_version text,
  latest_available_version text,
  update_available boolean,
  auto_update_enabled boolean,
  status text NOT NULL DEFAULT 'unknown',
  reason_code text,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  reported_at timestamptz,
  received_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT device_update_status_status_check CHECK (status IN ('current', 'pending', 'blocked', 'failed', 'rolled_back', 'unknown')),
  CONSTRAINT device_update_status_cloud_environment_check CHECK (cloud_environment_profile IS NULL OR cloud_environment_profile IN ('production', 'staging')),
  CONSTRAINT device_update_status_release_track_check CHECK (release_track IS NULL OR release_track IN ('stable', 'candidate', 'staging', 'dev'))
);

CREATE INDEX device_update_status_site_status_idx ON device_update_status(site_id, status, updated_at DESC);
CREATE INDEX device_update_status_channel_idx ON device_update_status(channel_key);

-- +goose Down
DROP INDEX IF EXISTS device_update_status_channel_idx;
DROP INDEX IF EXISTS device_update_status_site_status_idx;
DROP TABLE IF EXISTS device_update_status;
