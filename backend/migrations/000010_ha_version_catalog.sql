-- +goose Up
CREATE TABLE ha_core_version_catalog (
  catalog_key text PRIMARY KEY,
  latest_version text,
  source_url text,
  status text NOT NULL DEFAULT 'unavailable',
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  fetched_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ha_core_version_catalog_key_check CHECK (catalog_key IN ('stable')),
  CONSTRAINT ha_core_version_catalog_status_check CHECK (status IN ('active', 'stale', 'unavailable'))
);

CREATE TABLE device_ha_version_advisories (
  device_id text PRIMARY KEY REFERENCES devices(device_id) ON DELETE CASCADE,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  installed_version text,
  latest_version text,
  advisory_status text NOT NULL DEFAULT 'hidden',
  reason_code text NOT NULL,
  catalog_fetched_at timestamptz,
  observed_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT device_ha_version_advisories_status_check CHECK (advisory_status IN ('current', 'update_available', 'hidden'))
);

CREATE INDEX device_ha_version_advisories_site_status_idx ON device_ha_version_advisories(site_id, advisory_status, updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS device_ha_version_advisories_site_status_idx;
DROP TABLE IF EXISTS device_ha_version_advisories;
DROP TABLE IF EXISTS ha_core_version_catalog;
