-- +goose Up
CREATE TABLE artifact_uploads (
  upload_id text PRIMARY KEY,
  account_id text NOT NULL REFERENCES accounts(account_id) ON DELETE RESTRICT,
  site_id text NOT NULL REFERENCES sites(site_id) ON DELETE RESTRICT,
  device_id text NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
  command_id text REFERENCES commands(command_id) ON DELETE SET NULL,
  purpose text NOT NULL,
  status text NOT NULL DEFAULT 'pending_upload',
  requested_by_subject_type text,
  requested_by_subject_id text,
  object_bucket text NOT NULL,
  object_key text NOT NULL,
  content_type text NOT NULL,
  max_size_bytes bigint NOT NULL,
  expected_size_bytes bigint,
  checksum_sha256 text,
  local_artifact_ref text,
  redaction_profile text,
  manifest_object_key text,
  manifest_sha256 text,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  expires_at timestamptz NOT NULL,
  completed_at timestamptz,
  validated_at timestamptz,
  rejected_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT artifact_uploads_purpose_check CHECK (purpose IN ('error_log_bundle', 'diagnostic_bundle', 'debug_bundle', 'backup_artifact')),
  CONSTRAINT artifact_uploads_status_check CHECK (status IN ('pending_upload', 'uploaded', 'validated', 'rejected', 'expired', 'canceled')),
  CONSTRAINT artifact_uploads_max_size_check CHECK (max_size_bytes > 0),
  CONSTRAINT artifact_uploads_expected_size_check CHECK (expected_size_bytes IS NULL OR (expected_size_bytes >= 0 AND expected_size_bytes <= max_size_bytes)),
  CONSTRAINT artifact_uploads_expiry_check CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX artifact_uploads_object_unique_idx ON artifact_uploads(object_bucket, object_key);
CREATE INDEX artifact_uploads_device_status_idx ON artifact_uploads(device_id, status);
CREATE INDEX artifact_uploads_command_idx ON artifact_uploads(command_id) WHERE command_id IS NOT NULL;
CREATE INDEX artifact_uploads_site_created_idx ON artifact_uploads(site_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS artifact_uploads_site_created_idx;
DROP INDEX IF EXISTS artifact_uploads_command_idx;
DROP INDEX IF EXISTS artifact_uploads_device_status_idx;
DROP INDEX IF EXISTS artifact_uploads_object_unique_idx;
DROP TABLE IF EXISTS artifact_uploads;
