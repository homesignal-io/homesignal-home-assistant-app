package main

import "time"

const (
	deviceRecordSchemaVersion = 2

	defaultHomeSignalAPIBaseURL          = "https://api.homesignal.io"
	defaultAWSRegion                     = "us-east-1"
	defaultFleetProvisioningTemplateName = "homesignal-device-enrollment"

	defaultPairingPollInterval = 10 * time.Second
	enrollmentHTTPTimeout      = 10 * time.Second
)

type ClaimState string

const (
	ClaimStateUnclaimed      ClaimState = "UNCLAIMED"
	ClaimStatePairingPending ClaimState = "PAIRING_PENDING"
	ClaimStateClaimed        ClaimState = "CLAIMED"
	ClaimStateRevoked        ClaimState = "REVOKED"
)

type DeviceRecord struct {
	SchemaVersion  int        `json:"schema_version"`
	InstallationID string     `json:"installation_id"`
	CreatedAt      time.Time  `json:"created_at"`
	ClaimState     ClaimState `json:"claim_state"`

	DeviceID        string     `json:"device_id,omitempty"`
	IoTThingName    string     `json:"iot_thing_name,omitempty"`
	CertificatePath string     `json:"certificate_path,omitempty"`
	PrivateKeyPath  string     `json:"private_key_path,omitempty"`
	ClaimedAt       *time.Time `json:"claimed_at,omitempty"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`

	PairingSessionID      string     `json:"pairing_session_id,omitempty"`
	PairingCodeExpiry     *time.Time `json:"pairing_code_expiry,omitempty"`
	PollTokenPath         string     `json:"poll_token_path,omitempty"`
	PollTokenExpiry       *time.Time `json:"poll_token_expiry,omitempty"`
	PollIntervalSeconds   int        `json:"poll_interval_seconds,omitempty"`
	PendingDeviceID       string     `json:"pending_device_id,omitempty"`
	PendingIoTThingName   string     `json:"pending_iot_thing_name,omitempty"`
	PendingCertificateID  string     `json:"pending_certificate_id,omitempty"`
	PendingCertificateARN string     `json:"pending_certificate_arn,omitempty"`
}

type EnrollmentConfig struct {
	HomeSignalAPIBaseURL          string `json:"homesignal_api_base_url"`
	AWSIoTEndpoint                string `json:"aws_iot_endpoint"`
	AWSRegion                     string `json:"aws_region"`
	FleetProvisioningTemplateName string `json:"fleet_provisioning_template_name"`
	HomeSignalAPISource           string `json:"homesignal_api_source"`
	AWSIoTEndpointSource          string `json:"aws_iot_endpoint_source"`
}

func (c EnrollmentConfig) homeSignalConfigured() bool {
	return c.HomeSignalAPIBaseURL != ""
}

func (c EnrollmentConfig) awsIoTConfigured() bool {
	return c.AWSIoTEndpoint != "" && c.AWSRegion != "" && c.FleetProvisioningTemplateName != ""
}

type EnrollmentSnapshot struct {
	InstallationID          string
	ClaimState              ClaimState
	PairingCode             string
	PairingCodeExpiry       *time.Time
	Version                 string
	DeviceID                string
	IoTThingName            string
	CertificatePath         string
	PrivateKeyPath          string
	EndpointConfig          EnrollmentConfig
	HomeSignalConfigured    bool
	AWSIoTConfigured        bool
	DegradedReasons         []string
	EnrollmentStatusMessage string
}

type statusResponse struct {
	InstallationID       string     `json:"installation_id"`
	ClaimState           string     `json:"claim_state"`
	PairingCodeExpiry    *time.Time `json:"pairing_code_expiry,omitempty"`
	Version              string     `json:"version"`
	DeviceID             string     `json:"device_id,omitempty"`
	IoTThingName         string     `json:"iot_thing_name,omitempty"`
	CertificatePath      string     `json:"certificate_path,omitempty"`
	PrivateKeyPath       string     `json:"private_key_path,omitempty"`
	HomeSignalConfigured bool       `json:"homesignal_configured"`
	AWSIoTConfigured     bool       `json:"aws_iot_configured"`
	HomeSignalAPIBaseURL string     `json:"homesignal_api_base_url,omitempty"`
	AWSIoTEndpoint       string     `json:"aws_iot_endpoint,omitempty"`
	AWSRegion            string     `json:"aws_region,omitempty"`
	DegradedReasons      []string   `json:"degraded_reasons,omitempty"`
}

func newStatusResponse(snapshot EnrollmentSnapshot) statusResponse {
	return statusResponse{
		InstallationID:       snapshot.InstallationID,
		ClaimState:           string(snapshot.ClaimState),
		PairingCodeExpiry:    snapshot.PairingCodeExpiry,
		Version:              version,
		DeviceID:             snapshot.DeviceID,
		IoTThingName:         snapshot.IoTThingName,
		CertificatePath:      snapshot.CertificatePath,
		PrivateKeyPath:       snapshot.PrivateKeyPath,
		HomeSignalConfigured: snapshot.HomeSignalConfigured,
		AWSIoTConfigured:     snapshot.AWSIoTConfigured,
		HomeSignalAPIBaseURL: snapshot.EndpointConfig.HomeSignalAPIBaseURL,
		AWSIoTEndpoint:       snapshot.EndpointConfig.AWSIoTEndpoint,
		AWSRegion:            snapshot.EndpointConfig.AWSRegion,
		DegradedReasons:      snapshot.DegradedReasons,
	}
}
