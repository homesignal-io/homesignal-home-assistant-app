package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"
)

type EnrollmentManagerConfig struct {
	ConfigDir        string
	DeviceRecordPath string
	Config           EnrollmentConfig
	Client           HomeSignalEnrollmentClient
	Provisioner      FleetProvisioningClient
	Now              func() time.Time
	Record           DeviceRecord
}

type EnrollmentManager struct {
	configDir        string
	deviceRecordPath string
	config           EnrollmentConfig
	client           HomeSignalEnrollmentClient
	provisioner      FleetProvisioningClient
	now              func() time.Time

	mu              sync.Mutex
	record          DeviceRecord
	pairingCode     string
	degradedReasons []string
}

func NewEnrollmentManager(config EnrollmentManagerConfig) *EnrollmentManager {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	manager := &EnrollmentManager{
		configDir:        config.ConfigDir,
		deviceRecordPath: config.DeviceRecordPath,
		config:           config.Config,
		client:           config.Client,
		provisioner:      config.Provisioner,
		now:              now,
		record:           config.Record,
	}
	manager.fillDefaultPathsLocked()
	return manager
}

func (m *EnrollmentManager) Start(ctx context.Context, logger *slog.Logger) {
	go func() {
		m.runOnceWithTimeout(ctx, logger)
		ticker := time.NewTicker(defaultPairingPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.runOnceWithTimeout(ctx, logger)
			}
		}
	}()
}

func (m *EnrollmentManager) runOnceWithTimeout(parent context.Context, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(parent, enrollmentHTTPTimeout)
	defer cancel()
	if err := m.RunOnce(ctx); err != nil && logger != nil {
		logger.Debug("enrollment reconcile degraded", "reason", err.Error())
	}
}

func (m *EnrollmentManager) RunOnce(ctx context.Context) error {
	m.lock()
	defer m.unlock()

	m.degradedReasons = nil
	now := m.now().UTC()
	m.fillDefaultPathsLocked()
	m.normalizeLocked(now)

	if m.record.ClaimState == ClaimStateClaimed {
		if !fileExists(m.record.PrivateKeyPath) || !fileExists(m.record.CertificatePath) {
			m.record.ClaimState = ClaimStateUnclaimed
			m.record.ClaimedAt = nil
			m.addDegradedLocked("claimed_credential_files_missing")
			_ = saveDeviceRecord(m.deviceRecordPath, m.record)
			return fmt.Errorf("claimed_credential_files_missing")
		}
		return m.checkClaimedStatusLocked(ctx, now)
	}

	if m.record.ClaimState == ClaimStatePairingPending && m.record.PendingCertificateID == "" && m.pendingExpiredLocked(now) {
		m.clearPendingLocked()
		m.record.ClaimState = ClaimStateUnclaimed
		_ = saveDeviceRecord(m.deviceRecordPath, m.record)
	}

	switch m.record.ClaimState {
	case ClaimStateUnclaimed:
		return m.createPairingSessionLocked(ctx, now)
	case ClaimStatePairingPending:
		if m.record.PendingCertificateID != "" {
			return m.finalizeClaimLocked(ctx, now)
		}
		return m.pollPairingSessionLocked(ctx, now)
	case ClaimStateRevoked:
		return nil
	default:
		m.record.ClaimState = ClaimStateUnclaimed
		_ = saveDeviceRecord(m.deviceRecordPath, m.record)
		return m.createPairingSessionLocked(ctx, now)
	}
}

func (m *EnrollmentManager) Snapshot() EnrollmentSnapshot {
	m.lock()
	defer m.unlock()
	reasons := append([]string{}, m.degradedReasons...)
	return EnrollmentSnapshot{
		InstallationID:          m.record.InstallationID,
		ClaimState:              m.record.ClaimState,
		PairingCode:             m.pairingCode,
		PairingCodeExpiry:       m.record.PairingCodeExpiry,
		Version:                 version,
		DeviceID:                m.record.DeviceID,
		IoTThingName:            m.record.IoTThingName,
		CertificatePath:         m.record.CertificatePath,
		PrivateKeyPath:          m.record.PrivateKeyPath,
		EndpointConfig:          m.config,
		HomeSignalConfigured:    m.config.homeSignalConfigured(),
		AWSIoTConfigured:        m.config.awsIoTConfigured(),
		DegradedReasons:         reasons,
		EnrollmentStatusMessage: m.statusMessageLocked(),
	}
}

func (m *EnrollmentManager) createPairingSessionLocked(ctx context.Context, now time.Time) error {
	if !m.config.homeSignalConfigured() || m.client == nil {
		m.addDegradedLocked("homesignal_enrollment_unconfigured")
		return fmt.Errorf("homesignal_enrollment_unconfigured")
	}

	privateKey, err := ensureDevicePrivateKey(m.record.PrivateKeyPath)
	if err != nil {
		m.addDegradedLocked("device_private_key_unavailable")
		return err
	}
	csr, err := newCSRPEM(privateKey, m.record.InstallationID)
	if err != nil {
		m.addDegradedLocked("device_csr_unavailable")
		return err
	}

	response, err := m.client.CreatePairingSession(ctx, CreatePairingSessionRequest{
		InstallationID:                    m.record.InstallationID,
		AgentVersion:                      version,
		CSRPEM:                            csr,
		AWSRegion:                         m.config.AWSRegion,
		AWSIoTEndpoint:                    m.config.AWSIoTEndpoint,
		FleetProvisioningTemplateName:     m.config.FleetProvisioningTemplateName,
		RequiresPreProvisioningHook:       true,
		RequiresHomeSignalFinalization:    true,
		SupportsCSRBasedFleetProvisioning: true,
	})
	if err != nil {
		m.addDegradedLocked("homesignal_enrollment_unavailable")
		return err
	}
	if response.PairingSessionID == "" || response.PairingCode == "" || response.PollToken == "" {
		m.addDegradedLocked("homesignal_pairing_session_invalid")
		return fmt.Errorf("homesignal_pairing_session_invalid")
	}

	pollTokenExpiry := response.PollTokenExpiresAt.UTC()
	if pollTokenExpiry.IsZero() {
		pollTokenExpiry = response.ExpiresAt.UTC()
	}
	if response.PollIntervalSeconds <= 0 {
		response.PollIntervalSeconds = int(defaultPairingPollInterval.Seconds())
	}

	if err := writeSecretFile(m.record.PollTokenPath, []byte(response.PollToken)); err != nil {
		m.addDegradedLocked("poll_token_storage_failed")
		return err
	}

	expiresAt := response.ExpiresAt.UTC()
	m.record.ClaimState = ClaimStatePairingPending
	m.record.PairingSessionID = response.PairingSessionID
	m.record.PairingCodeExpiry = &expiresAt
	m.record.PollTokenExpiry = &pollTokenExpiry
	m.record.PollIntervalSeconds = response.PollIntervalSeconds
	m.pairingCode = response.PairingCode

	if m.record.PairingCodeExpiry != nil && m.record.PairingCodeExpiry.Before(now) {
		m.addDegradedLocked("homesignal_pairing_session_expired")
		return fmt.Errorf("homesignal_pairing_session_expired")
	}

	return saveDeviceRecord(m.deviceRecordPath, m.record)
}

func (m *EnrollmentManager) pollPairingSessionLocked(ctx context.Context, now time.Time) error {
	if !m.config.homeSignalConfigured() || m.client == nil {
		m.addDegradedLocked("homesignal_enrollment_unconfigured")
		return fmt.Errorf("homesignal_enrollment_unconfigured")
	}
	pollToken, err := m.pollTokenLocked()
	if err != nil {
		m.addDegradedLocked("poll_token_unavailable")
		return err
	}

	response, err := m.client.PollPairingSession(ctx, PollPairingSessionRequest{
		PairingSessionID: m.record.PairingSessionID,
		PollToken:        pollToken,
	})
	if err != nil {
		m.addDegradedLocked("homesignal_pairing_poll_failed")
		return err
	}
	if response.PairingCode != "" {
		m.pairingCode = response.PairingCode
	}
	if !response.ExpiresAt.IsZero() {
		expiresAt := response.ExpiresAt.UTC()
		m.record.PairingCodeExpiry = &expiresAt
	}

	switch response.Status {
	case "pending", "PAIRING_PENDING", "":
		if m.pendingExpiredLocked(now) {
			m.clearPendingLocked()
			m.record.ClaimState = ClaimStateUnclaimed
		}
		return saveDeviceRecord(m.deviceRecordPath, m.record)
	case "approved", "APPROVED":
		return m.handleApprovedClaimLocked(ctx, response, pollToken, now)
	case "claimed", "CLAIMED":
		m.addDegradedLocked("homesignal_pairing_claimed_without_local_finalization")
		return fmt.Errorf("homesignal_pairing_claimed_without_local_finalization")
	case "revoked", "REVOKED":
		revokedAt := now.UTC()
		m.record.ClaimState = ClaimStateRevoked
		m.record.RevokedAt = &revokedAt
		return saveDeviceRecord(m.deviceRecordPath, m.record)
	case "expired", "EXPIRED":
		m.clearPendingLocked()
		m.record.ClaimState = ClaimStateUnclaimed
		return saveDeviceRecord(m.deviceRecordPath, m.record)
	default:
		m.addDegradedLocked("homesignal_pairing_status_unknown")
		return fmt.Errorf("homesignal_pairing_status_unknown")
	}
}

func (m *EnrollmentManager) handleApprovedClaimLocked(ctx context.Context, response PollPairingSessionResponse, pollToken string, now time.Time) error {
	if response.DeviceID == "" {
		m.addDegradedLocked("homesignal_claim_missing_device_id")
		return fmt.Errorf("homesignal_claim_missing_device_id")
	}
	if response.AWSClaimCertificatePEM == "" || response.AWSClaimPrivateKeyPEM == "" {
		m.addDegradedLocked("homesignal_claim_missing_aws_material")
		return fmt.Errorf("homesignal_claim_missing_aws_material")
	}
	if !response.AWSClaimExpiresAt.IsZero() && !response.AWSClaimExpiresAt.After(now) {
		m.addDegradedLocked("aws_claim_material_expired")
		return fmt.Errorf("aws_claim_material_expired")
	}
	if !m.config.awsIoTConfigured() {
		m.addDegradedLocked("aws_iot_endpoint_unconfigured")
		return fmt.Errorf("aws_iot_endpoint_unconfigured")
	}

	claimCertPath := filepath.Join(m.configDir, "secrets", "aws_claim.crt")
	claimKeyPath := filepath.Join(m.configDir, "secrets", "aws_claim.key")
	if err := writeSecretFile(claimCertPath, []byte(response.AWSClaimCertificatePEM)); err != nil {
		m.addDegradedLocked("aws_claim_certificate_storage_failed")
		return err
	}
	if err := writeSecretFile(claimKeyPath, []byte(response.AWSClaimPrivateKeyPEM)); err != nil {
		m.addDegradedLocked("aws_claim_key_storage_failed")
		return err
	}
	defer func() {
		_ = removeIfExists(claimCertPath)
		_ = removeIfExists(claimKeyPath)
	}()

	privateKey, err := ensureDevicePrivateKey(m.record.PrivateKeyPath)
	if err != nil {
		m.addDegradedLocked("device_private_key_unavailable")
		return err
	}
	csr, err := newCSRPEM(privateKey, m.record.InstallationID)
	if err != nil {
		m.addDegradedLocked("device_csr_unavailable")
		return err
	}
	template := response.FleetProvisioningTemplate
	if template == "" {
		template = m.config.FleetProvisioningTemplateName
	}

	provisioned, err := m.provisioner.Provision(ctx, FleetProvisioningRequest{
		AWSIoTEndpoint:                m.config.AWSIoTEndpoint,
		AWSRegion:                     m.config.AWSRegion,
		FleetProvisioningTemplateName: template,
		ClaimCertificatePath:          claimCertPath,
		ClaimPrivateKeyPath:           claimKeyPath,
		DevicePrivateKeyPath:          m.record.PrivateKeyPath,
		CSRPEM:                        csr,
		InstallationID:                m.record.InstallationID,
		PairingSessionID:              m.record.PairingSessionID,
		DeviceID:                      response.DeviceID,
		IoTThingName:                  response.IoTThingName,
		TemplateParameters:            response.FleetProvisioningParameters,
	})
	if err != nil {
		m.addDegradedLocked("aws_fleet_provisioning_failed")
		return err
	}
	if provisioned.CertificatePEM == "" || provisioned.CertificateID == "" {
		m.addDegradedLocked("aws_fleet_provisioning_response_invalid")
		return fmt.Errorf("aws_fleet_provisioning_response_invalid")
	}
	if provisioned.DeviceID == "" {
		provisioned.DeviceID = response.DeviceID
	}
	if provisioned.IoTThingName == "" {
		provisioned.IoTThingName = response.IoTThingName
	}
	if err := writeSecretFile(m.record.CertificatePath, []byte(provisioned.CertificatePEM)); err != nil {
		m.addDegradedLocked("device_certificate_storage_failed")
		return err
	}

	m.record.PendingDeviceID = provisioned.DeviceID
	m.record.PendingIoTThingName = provisioned.IoTThingName
	m.record.PendingCertificateID = provisioned.CertificateID
	m.record.PendingCertificateARN = provisioned.CertificateARN
	if err := saveDeviceRecord(m.deviceRecordPath, m.record); err != nil {
		return err
	}

	return m.finalizeClaimWithPollTokenLocked(ctx, pollToken, now)
}

func (m *EnrollmentManager) finalizeClaimLocked(ctx context.Context, now time.Time) error {
	pollToken, err := m.pollTokenLocked()
	if err != nil {
		m.addDegradedLocked("poll_token_unavailable")
		return err
	}
	return m.finalizeClaimWithPollTokenLocked(ctx, pollToken, now)
}

func (m *EnrollmentManager) finalizeClaimWithPollTokenLocked(ctx context.Context, pollToken string, now time.Time) error {
	if !m.config.homeSignalConfigured() || m.client == nil {
		m.addDegradedLocked("homesignal_enrollment_unconfigured")
		return fmt.Errorf("homesignal_enrollment_unconfigured")
	}
	response, err := m.client.FinalizeClaim(ctx, FinalizeClaimRequest{
		PairingSessionID: m.record.PairingSessionID,
		PollToken:        pollToken,
		InstallationID:   m.record.InstallationID,
		DeviceID:         m.record.PendingDeviceID,
		IoTThingName:     m.record.PendingIoTThingName,
		CertificateID:    m.record.PendingCertificateID,
		CertificateARN:   m.record.PendingCertificateARN,
		CertificatePath:  m.record.CertificatePath,
		PrivateKeyPath:   m.record.PrivateKeyPath,
	})
	if err != nil {
		m.addDegradedLocked("iot_provisioned_awaiting_homesignal_confirmation")
		_ = saveDeviceRecord(m.deviceRecordPath, m.record)
		return err
	}

	switch response.Status {
	case "claimed", "CLAIMED":
		claimedAt := response.ClaimedAt.UTC()
		if claimedAt.IsZero() {
			claimedAt = now.UTC()
		}
		m.record.ClaimState = ClaimStateClaimed
		m.record.DeviceID = firstNonEmpty(response.DeviceID, m.record.PendingDeviceID)
		m.record.IoTThingName = firstNonEmpty(response.IoTThingName, m.record.PendingIoTThingName)
		m.record.ClaimedAt = &claimedAt
		m.clearPendingLocked()
		if err := removeIfExists(m.record.PollTokenPath); err != nil {
			m.addDegradedLocked("poll_token_cleanup_failed")
			return err
		}
		return saveDeviceRecord(m.deviceRecordPath, m.record)
	case "revoked", "REVOKED", "rejected", "REJECTED":
		revokedAt := now.UTC()
		m.record.ClaimState = ClaimStateRevoked
		m.record.RevokedAt = &revokedAt
		m.clearPendingLocked()
		return saveDeviceRecord(m.deviceRecordPath, m.record)
	default:
		m.addDegradedLocked("iot_provisioned_awaiting_homesignal_confirmation")
		return fmt.Errorf("homesignal_finalization_not_confirmed")
	}
}

func (m *EnrollmentManager) checkClaimedStatusLocked(ctx context.Context, now time.Time) error {
	if !m.config.homeSignalConfigured() || m.client == nil {
		m.addDegradedLocked("homesignal_status_unavailable")
		return fmt.Errorf("homesignal_status_unavailable")
	}
	response, err := m.client.DeviceStatus(ctx, DeviceStatusRequest{DeviceID: m.record.DeviceID})
	if err != nil {
		m.addDegradedLocked("homesignal_status_unavailable")
		return err
	}
	switch response.Status {
	case "claimed", "CLAIMED", "ok", "OK":
		return nil
	case "revoked", "REVOKED":
		revokedAt := now.UTC()
		m.record.ClaimState = ClaimStateRevoked
		m.record.RevokedAt = &revokedAt
		return saveDeviceRecord(m.deviceRecordPath, m.record)
	default:
		m.addDegradedLocked("homesignal_device_status_unknown")
		return fmt.Errorf("homesignal_device_status_unknown")
	}
}

func (m *EnrollmentManager) normalizeLocked(now time.Time) {
	if m.record.SchemaVersion == 0 {
		m.record.SchemaVersion = deviceRecordSchemaVersion
	}
	if m.record.CreatedAt.IsZero() {
		m.record.CreatedAt = now
	}
	if m.record.ClaimState == "" {
		m.record.ClaimState = ClaimStateUnclaimed
	}
}

func (m *EnrollmentManager) fillDefaultPathsLocked() {
	if m.record.PrivateKeyPath == "" {
		m.record.PrivateKeyPath = filepath.Join(m.configDir, "iot", "device.key")
	}
	if m.record.CertificatePath == "" {
		m.record.CertificatePath = filepath.Join(m.configDir, "iot", "device.crt")
	}
	if m.record.PollTokenPath == "" {
		m.record.PollTokenPath = filepath.Join(m.configDir, "secrets", "poll_token")
	}
}

func (m *EnrollmentManager) pendingExpiredLocked(now time.Time) bool {
	if m.record.PairingCodeExpiry != nil && !m.record.PairingCodeExpiry.After(now) {
		return true
	}
	if m.record.PollTokenExpiry != nil && !m.record.PollTokenExpiry.After(now) {
		return true
	}
	return false
}

func (m *EnrollmentManager) clearPendingLocked() {
	m.record.PairingSessionID = ""
	m.record.PairingCodeExpiry = nil
	m.record.PollTokenExpiry = nil
	m.record.PollIntervalSeconds = 0
	m.record.PendingDeviceID = ""
	m.record.PendingIoTThingName = ""
	m.record.PendingCertificateID = ""
	m.record.PendingCertificateARN = ""
	m.pairingCode = ""
}

func (m *EnrollmentManager) pollTokenLocked() (string, error) {
	payload, err := readSecretFile(m.record.PollTokenPath)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (m *EnrollmentManager) addDegradedLocked(reason string) {
	if reason == "" {
		return
	}
	for _, existing := range m.degradedReasons {
		if existing == reason {
			return
		}
	}
	m.degradedReasons = append(m.degradedReasons, reason)
}

func (m *EnrollmentManager) statusMessageLocked() string {
	switch m.record.ClaimState {
	case ClaimStateUnclaimed:
		if len(m.degradedReasons) > 0 {
			return "Enrollment is unavailable."
		}
		return "Waiting to create a pairing session."
	case ClaimStatePairingPending:
		if m.record.PendingCertificateID != "" {
			return "AWS IoT provisioned; waiting for HomeSignal confirmation."
		}
		return "Waiting for SaaS claim approval."
	case ClaimStateClaimed:
		return "Device is claimed."
	case ClaimStateRevoked:
		return "Device credential is revoked."
	default:
		return "Enrollment state is unknown."
	}
}

func (m *EnrollmentManager) lock() {
	m.mu.Lock()
}

func (m *EnrollmentManager) unlock() {
	m.mu.Unlock()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
