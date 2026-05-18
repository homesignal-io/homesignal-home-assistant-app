package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/homesignal-io/homesignal-home-assistant-app/telemetry-ingest/internal/pipeline"
	"github.com/jackc/pgx/v5"
)

func (w Writer) ResolveDevice(ctx context.Context, credential pipeline.TransportCredential) (pipeline.AuthenticatedDeviceContext, error) {
	if w.Pool == nil {
		return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("postgres pool is required")
	}
	fingerprint := strings.TrimSpace(credential.CertificateFingerprint)
	serial := strings.TrimSpace(credential.CertificateSerial)
	if fingerprint == "" || serial == "" {
		return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("%w: certificate fingerprint and serial are required", pipeline.ErrMissingTransportCredential)
	}

	var record struct {
		DeviceID              string
		AccountID             string
		SiteID                string
		CredentialFingerprint string
		CredentialSerial      string
		CredentialStatus      string
		DeviceClaimState      string
		DeviceRevoked         bool
	}
	err := w.Pool.QueryRow(ctx, `
SELECT
  d.device_id,
  d.account_id,
  COALESCE(d.site_id, ''),
  dc.certificate_fingerprint,
  COALESCE(dc.certificate_serial, ''),
  dc.status,
  d.claim_state,
  d.revoked_at IS NOT NULL
FROM device_credentials dc
JOIN devices d ON d.device_id = dc.device_id
WHERE dc.certificate_fingerprint = $1
LIMIT 1
`, fingerprint).Scan(
		&record.DeviceID,
		&record.AccountID,
		&record.SiteID,
		&record.CredentialFingerprint,
		&record.CredentialSerial,
		&record.CredentialStatus,
		&record.DeviceClaimState,
		&record.DeviceRevoked,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("%w: unknown certificate fingerprint", pipeline.ErrIdentityDrift)
		}
		return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("lookup device credential: %w", err)
	}
	if !strings.EqualFold(record.CredentialStatus, "active") {
		return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("%w: credential is %s", pipeline.ErrIdentityDrift, record.CredentialStatus)
	}
	if record.CredentialSerial != serial {
		return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("%w: certificate serial mismatch", pipeline.ErrIdentityDrift)
	}
	if record.DeviceClaimState != "CLAIMED" || record.DeviceRevoked {
		return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("%w: device is not claimed", pipeline.ErrIdentityDrift)
	}

	if _, err := w.Pool.Exec(ctx, `
UPDATE device_credentials
SET last_seen_at = now(),
    updated_at = now()
WHERE certificate_fingerprint = $1
`, fingerprint); err != nil {
		return pipeline.AuthenticatedDeviceContext{}, fmt.Errorf("update credential last seen: %w", err)
	}

	return pipeline.AuthenticatedDeviceContext{
		DeviceID:               record.DeviceID,
		SiteID:                 record.SiteID,
		OrgID:                  record.AccountID,
		CertificateFingerprint: record.CredentialFingerprint,
		CertificateSerial:      record.CredentialSerial,
	}, nil
}
