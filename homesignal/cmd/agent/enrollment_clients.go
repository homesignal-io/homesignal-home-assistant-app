package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HomeSignalEnrollmentClient interface {
	CreatePairingSession(context.Context, CreatePairingSessionRequest) (CreatePairingSessionResponse, error)
	PollPairingSession(context.Context, PollPairingSessionRequest) (PollPairingSessionResponse, error)
	FinalizeClaim(context.Context, FinalizeClaimRequest) (FinalizeClaimResponse, error)
	DeviceStatus(context.Context, DeviceStatusRequest) (DeviceStatusResponse, error)
}

type FleetProvisioningClient interface {
	Provision(context.Context, FleetProvisioningRequest) (FleetProvisioningResponse, error)
}

type CreatePairingSessionRequest struct {
	InstallationID                    string `json:"installation_id"`
	AgentVersion                      string `json:"agent_version"`
	CSRPEM                            string `json:"csr_pem"`
	AWSRegion                         string `json:"aws_region"`
	AWSIoTEndpoint                    string `json:"aws_iot_endpoint"`
	FleetProvisioningTemplateName     string `json:"fleet_provisioning_template_name"`
	RequiresPreProvisioningHook       bool   `json:"requires_pre_provisioning_hook"`
	RequiresHomeSignalFinalization    bool   `json:"requires_homesignal_finalization"`
	SupportsCSRBasedFleetProvisioning bool   `json:"supports_csr_based_fleet_provisioning"`
}

type CreatePairingSessionResponse struct {
	PairingSessionID    string    `json:"pairing_session_id"`
	PairingCode         string    `json:"pairing_code"`
	ExpiresAt           time.Time `json:"expires_at"`
	PollIntervalSeconds int       `json:"poll_interval_seconds"`
	PollToken           string    `json:"poll_token"`
	PollTokenExpiresAt  time.Time `json:"poll_token_expires_at"`
}

type PollPairingSessionRequest struct {
	PairingSessionID string
	PollToken        string
}

type PollPairingSessionResponse struct {
	Status                      string            `json:"status"`
	PairingCode                 string            `json:"pairing_code,omitempty"`
	ExpiresAt                   time.Time         `json:"expires_at,omitempty"`
	DeviceID                    string            `json:"device_id,omitempty"`
	IoTThingName                string            `json:"iot_thing_name,omitempty"`
	AWSClaimCertificatePEM      string            `json:"aws_claim_certificate_pem,omitempty"`
	AWSClaimPrivateKeyPEM       string            `json:"aws_claim_private_key_pem,omitempty"`
	AWSClaimExpiresAt           time.Time         `json:"aws_claim_expires_at,omitempty"`
	FleetProvisioningTemplate   string            `json:"fleet_provisioning_template_name,omitempty"`
	FleetProvisioningParameters map[string]string `json:"fleet_provisioning_parameters,omitempty"`
}

type FinalizeClaimRequest struct {
	PairingSessionID string `json:"pairing_session_id"`
	PollToken        string `json:"-"`
	InstallationID   string `json:"installation_id"`
	DeviceID         string `json:"device_id"`
	IoTThingName     string `json:"iot_thing_name"`
	CertificateID    string `json:"certificate_id"`
	CertificateARN   string `json:"certificate_arn,omitempty"`
	CertificatePath  string `json:"certificate_path"`
	PrivateKeyPath   string `json:"private_key_path"`
}

type FinalizeClaimResponse struct {
	Status       string    `json:"status"`
	DeviceID     string    `json:"device_id,omitempty"`
	IoTThingName string    `json:"iot_thing_name,omitempty"`
	ClaimedAt    time.Time `json:"claimed_at,omitempty"`
	Reason       string    `json:"reason,omitempty"`
}

type DeviceStatusRequest struct {
	DeviceID string
}

type DeviceStatusResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type FleetProvisioningRequest struct {
	AWSIoTEndpoint                string
	AWSRegion                     string
	FleetProvisioningTemplateName string
	ClaimCertificatePath          string
	ClaimPrivateKeyPath           string
	DevicePrivateKeyPath          string
	CSRPEM                        string
	InstallationID                string
	PairingSessionID              string
	DeviceID                      string
	IoTThingName                  string
	TemplateParameters            map[string]string
}

type FleetProvisioningResponse struct {
	DeviceID       string
	IoTThingName   string
	CertificateID  string
	CertificateARN string
	CertificatePEM string
}

type UnsupportedFleetProvisioningClient struct{}

func (UnsupportedFleetProvisioningClient) Provision(context.Context, FleetProvisioningRequest) (FleetProvisioningResponse, error) {
	return FleetProvisioningResponse{}, fmt.Errorf("aws_fleet_provisioning_client_not_configured")
}

type HTTPHomeSignalClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPHomeSignalClient(baseURL string) *HTTPHomeSignalClient {
	return &HTTPHomeSignalClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: enrollmentHTTPTimeout,
		},
	}
}

func (c *HTTPHomeSignalClient) CreatePairingSession(ctx context.Context, request CreatePairingSessionRequest) (CreatePairingSessionResponse, error) {
	var response CreatePairingSessionResponse
	if err := c.postJSON(ctx, "/api/device-enrollment/pairing-sessions", "", request, &response); err != nil {
		return CreatePairingSessionResponse{}, err
	}
	return response, nil
}

func (c *HTTPHomeSignalClient) PollPairingSession(ctx context.Context, request PollPairingSessionRequest) (PollPairingSessionResponse, error) {
	var response PollPairingSessionResponse
	path := "/api/device-enrollment/pairing-sessions/" + url.PathEscape(request.PairingSessionID)
	if err := c.getJSON(ctx, path, request.PollToken, &response); err != nil {
		return PollPairingSessionResponse{}, err
	}
	return response, nil
}

func (c *HTTPHomeSignalClient) FinalizeClaim(ctx context.Context, request FinalizeClaimRequest) (FinalizeClaimResponse, error) {
	var response FinalizeClaimResponse
	path := "/api/device-enrollment/pairing-sessions/" + url.PathEscape(request.PairingSessionID) + "/finalize"
	if err := c.postJSON(ctx, path, request.PollToken, request, &response); err != nil {
		return FinalizeClaimResponse{}, err
	}
	return response, nil
}

func (c *HTTPHomeSignalClient) DeviceStatus(ctx context.Context, request DeviceStatusRequest) (DeviceStatusResponse, error) {
	var response DeviceStatusResponse
	path := "/api/devices/" + url.PathEscape(request.DeviceID) + "/status"
	if err := c.getJSON(ctx, path, "", &response); err != nil {
		return DeviceStatusResponse{}, err
	}
	return response, nil
}

func (c *HTTPHomeSignalClient) postJSON(ctx context.Context, path, bearer string, payload interface{}, target interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return c.doJSON(req, target)
}

func (c *HTTPHomeSignalClient) getJSON(ctx context.Context, path, bearer string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return c.doJSON(req, target)
}

func (c *HTTPHomeSignalClient) doJSON(req *http.Request, target interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("homesignal request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("homesignal request returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode homesignal response: %w", err)
	}
	return nil
}
