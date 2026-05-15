package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)

func TestLoadOptionsMissingFile(t *testing.T) {
	options, err := loadOptions(filepath.Join(t.TempDir(), "options.json"))
	if err != nil {
		t.Fatalf("loadOptions returned error: %v", err)
	}
	if options.Present {
		t.Fatal("expected missing options file to be tolerated")
	}
}

func TestLoadOptionsParsesJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "options.json")
	if err := os.WriteFile(path, []byte(`{"log_level":"debug"}`), 0o600); err != nil {
		t.Fatalf("write options: %v", err)
	}

	options, err := loadOptions(path)
	if err != nil {
		t.Fatalf("loadOptions returned error: %v", err)
	}
	if !options.Present {
		t.Fatal("expected options file to be present")
	}
	if options.Options["log_level"] != "debug" {
		t.Fatalf("expected log_level option, got %#v", options.Options)
	}
}

func TestEnsureIdentityCreatesAndReusesInstallationID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "device.json")

	first, err := ensureIdentity(path)
	if err != nil {
		t.Fatalf("ensureIdentity first run: %v", err)
	}
	if first.InstallationID == "" {
		t.Fatal("expected generated installation_id")
	}

	second, err := ensureIdentity(path)
	if err != nil {
		t.Fatalf("ensureIdentity second run: %v", err)
	}
	if second.InstallationID != first.InstallationID {
		t.Fatalf("expected identity reuse, first=%q second=%q", first.InstallationID, second.InstallationID)
	}

	var record DeviceRecord
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read device record: %v", err)
	}
	if err := json.Unmarshal(payload, &record); err != nil {
		t.Fatalf("decode device record: %v", err)
	}
	if record.SchemaVersion != deviceRecordSchemaVersion {
		t.Fatalf("expected schema v2, got %d", record.SchemaVersion)
	}
	if record.ClaimState != ClaimStateUnclaimed {
		t.Fatalf("expected unclaimed state, got %s", record.ClaimState)
	}
}

func TestLegacyDeviceRecordMigratesToSchemaV2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "device.json")
	if err := os.WriteFile(path, []byte(`{"installation_id":"legacy-installation","created_at":"2026-05-11T12:00:00Z"}`), 0o600); err != nil {
		t.Fatalf("write legacy record: %v", err)
	}

	record, err := loadDeviceRecord(path, testNow)
	if err != nil {
		t.Fatalf("load legacy record: %v", err)
	}
	if record.InstallationID != "legacy-installation" {
		t.Fatalf("expected installation id to survive migration, got %q", record.InstallationID)
	}
	if record.SchemaVersion != deviceRecordSchemaVersion {
		t.Fatalf("expected schema v2, got %d", record.SchemaVersion)
	}
	if record.ClaimState != ClaimStateUnclaimed {
		t.Fatalf("expected unclaimed after migration, got %s", record.ClaimState)
	}
}

func TestHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	healthHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestVersionEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/version", nil)

	versionHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["version"] == "" || response["commit"] == "" || response["build_time"] == "" {
		t.Fatalf("expected version metadata, got %#v", response)
	}
}

func TestRouterServesUIAtIngressRoot(t *testing.T) {
	state := newTestRuntimeState(t, &fakeEnrollmentClient{}, fakeFleetProvisioner{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	newRouter(state).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "HomeSignal") {
		t.Fatalf("expected UI response, got %s", recorder.Body.String())
	}
}

func TestReadyEndpointDegradedWithoutSupervisorTokenIncludesClaimState(t *testing.T) {
	state := newTestRuntimeState(t, &fakeEnrollmentClient{}, fakeFleetProvisioner{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	readyHandler(state)(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var response readyResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Ready {
		t.Fatal("expected ready response")
	}
	if !response.Degraded {
		t.Fatal("expected degraded response without supervisor token")
	}
	if response.SupervisorToken {
		t.Fatal("expected supervisor token to be absent")
	}
	if response.ClaimState != string(ClaimStateUnclaimed) {
		t.Fatalf("expected claim state in readiness, got %q", response.ClaimState)
	}
}

func TestFreshInstallCreatesPairingSessionAndUIShowsCode(t *testing.T) {
	client := &fakeEnrollmentClient{
		createResponse: CreatePairingSessionResponse{
			PairingSessionID:    "ps_123",
			PairingCode:         "123456",
			ExpiresAt:           testNow.Add(10 * time.Minute),
			PollIntervalSeconds: 5,
			PollToken:           "poll-secret",
			PollTokenExpiresAt:  testNow.Add(10 * time.Minute),
		},
	}
	state := newTestRuntimeState(t, client, fakeFleetProvisioner{})

	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("run enrollment: %v", err)
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStatePairingPending {
		t.Fatalf("expected pending state, got %s", snapshot.ClaimState)
	}
	if snapshot.PairingCode != "123456" {
		t.Fatalf("expected pairing code in UI snapshot, got %q", snapshot.PairingCode)
	}

	statusRecorder := httptest.NewRecorder()
	statusHandler(state)(statusRecorder, httptest.NewRequest(http.MethodGet, "/status", nil))
	if strings.Contains(statusRecorder.Body.String(), "123456") || strings.Contains(statusRecorder.Body.String(), "poll-secret") {
		t.Fatalf("status leaked pairing code or poll token: %s", statusRecorder.Body.String())
	}

	uiRecorder := httptest.NewRecorder()
	uiHandler(state)(uiRecorder, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if !strings.Contains(uiRecorder.Body.String(), "123456") {
		t.Fatalf("expected UI to show pairing code, got %s", uiRecorder.Body.String())
	}
	if strings.Contains(uiRecorder.Body.String(), "poll-secret") {
		t.Fatalf("UI leaked poll token: %s", uiRecorder.Body.String())
	}
}

func TestBackendUnavailableBootsUnclaimedWithDegradedReason(t *testing.T) {
	state := newTestRuntimeState(t, &fakeEnrollmentClient{createErr: errors.New("down")}, fakeFleetProvisioner{})

	if err := state.Enrollment.RunOnce(context.Background()); err == nil {
		t.Fatal("expected degraded enrollment error")
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStateUnclaimed {
		t.Fatalf("expected unclaimed state, got %s", snapshot.ClaimState)
	}
	if !contains(snapshot.DegradedReasons, "homesignal_enrollment_unavailable") {
		t.Fatalf("expected degraded reason, got %#v", snapshot.DegradedReasons)
	}
}

func TestPendingSessionSurvivesRestartUntilExpiry(t *testing.T) {
	dir := t.TempDir()
	client := &fakeEnrollmentClient{
		createResponse: CreatePairingSessionResponse{
			PairingSessionID:   "ps_restart",
			PairingCode:        "777888",
			ExpiresAt:          testNow.Add(10 * time.Minute),
			PollToken:          "poll-secret",
			PollTokenExpiresAt: testNow.Add(10 * time.Minute),
		},
	}
	state := newTestRuntimeStateInDir(t, dir, client, fakeFleetProvisioner{})
	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("create pending session: %v", err)
	}

	reloaded, err := loadRuntimeStateWithClients(dir, filepath.Join(dir, "data"), "token", &fakeEnrollmentClient{
		pollResponse: PollPairingSessionResponse{
			Status:      "pending",
			PairingCode: "777888",
			ExpiresAt:   testNow.Add(9 * time.Minute),
		},
	}, fakeFleetProvisioner{}, func() time.Time { return testNow.Add(time.Minute) })
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if err := reloaded.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("poll reloaded pending session: %v", err)
	}
	snapshot := reloaded.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStatePairingPending {
		t.Fatalf("expected pending after restart, got %s", snapshot.ClaimState)
	}
}

func TestExpiredPendingSessionIsNotReused(t *testing.T) {
	dir := t.TempDir()
	expired := testNow.Add(-time.Minute)
	record := DeviceRecord{
		SchemaVersion:     deviceRecordSchemaVersion,
		InstallationID:    "install-expired",
		CreatedAt:         testNow.Add(-time.Hour),
		ClaimState:        ClaimStatePairingPending,
		PairingSessionID:  "old-session",
		PairingCodeExpiry: &expired,
		PollTokenPath:     filepath.Join(dir, "secrets", "poll_token"),
		PrivateKeyPath:    filepath.Join(dir, "iot", "device.key"),
		CertificatePath:   filepath.Join(dir, "iot", "device.crt"),
	}
	if err := saveDeviceRecord(filepath.Join(dir, "device.json"), record); err != nil {
		t.Fatalf("save record: %v", err)
	}
	if err := writeSecretFile(record.PollTokenPath, []byte("old-poll-token")); err != nil {
		t.Fatalf("write poll token: %v", err)
	}

	client := &fakeEnrollmentClient{
		createResponse: CreatePairingSessionResponse{
			PairingSessionID:   "new-session",
			PairingCode:        "999000",
			ExpiresAt:          testNow.Add(10 * time.Minute),
			PollToken:          "new-poll-token",
			PollTokenExpiresAt: testNow.Add(10 * time.Minute),
		},
	}
	state := newTestRuntimeStateInDir(t, dir, client, fakeFleetProvisioner{})
	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("refresh expired session: %v", err)
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStatePairingPending {
		t.Fatalf("expected new pending state, got %s", snapshot.ClaimState)
	}
	if snapshot.PairingCode != "999000" {
		t.Fatalf("expected new pairing code, got %q", snapshot.PairingCode)
	}
}

func TestClaimSuccessTransitionsToClaimedAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	client := &fakeEnrollmentClient{
		createResponse: CreatePairingSessionResponse{
			PairingSessionID:   "ps_claim",
			PairingCode:        "111222",
			ExpiresAt:          testNow.Add(10 * time.Minute),
			PollToken:          "poll-secret",
			PollTokenExpiresAt: testNow.Add(10 * time.Minute),
		},
		pollResponse: PollPairingSessionResponse{
			Status:                 "approved",
			DeviceID:               "dev_123",
			IoTThingName:           "homesignal-dev-123",
			AWSClaimCertificatePEM: "temporary claim cert",
			AWSClaimPrivateKeyPEM:  "temporary claim key",
		},
		finalizeResponse: FinalizeClaimResponse{
			Status:       "claimed",
			DeviceID:     "dev_123",
			IoTThingName: "homesignal-dev-123",
			ClaimedAt:    testNow.Add(2 * time.Minute),
		},
		deviceStatusResponse: DeviceStatusResponse{Status: "claimed"},
	}
	state := newTestRuntimeStateInDir(t, dir, client, fakeFleetProvisioner{
		response: FleetProvisioningResponse{
			DeviceID:       "dev_123",
			IoTThingName:   "homesignal-dev-123",
			CertificateID:  "cert_123",
			CertificateARN: "arn:aws:iot:us-east-1:123:cert/cert_123",
			CertificatePEM: "durable cert",
		},
	})
	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("create pairing: %v", err)
	}
	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("complete claim: %v", err)
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStateClaimed {
		t.Fatalf("expected claimed, got %s reasons=%#v", snapshot.ClaimState, snapshot.DegradedReasons)
	}
	if !fileExists(snapshot.PrivateKeyPath) || !fileExists(snapshot.CertificatePath) {
		t.Fatalf("expected cert/key files at %q and %q", snapshot.CertificatePath, snapshot.PrivateKeyPath)
	}
	assertFileMode(t, snapshot.PrivateKeyPath, 0o600)
	assertFileMode(t, snapshot.CertificatePath, 0o600)

	reloaded, err := loadRuntimeStateWithClients(dir, filepath.Join(dir, "data"), "token", client, fakeFleetProvisioner{}, func() time.Time { return testNow.Add(3 * time.Minute) })
	if err != nil {
		t.Fatalf("reload claimed state: %v", err)
	}
	if err := reloaded.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("check claimed status: %v", err)
	}
	if got := reloaded.Enrollment.Snapshot().ClaimState; got != ClaimStateClaimed {
		t.Fatalf("expected restart to preserve claimed, got %s", got)
	}
}

func TestAWSProvisionedButNotFinalizedStaysDegradedPending(t *testing.T) {
	client := &fakeEnrollmentClient{
		createResponse: CreatePairingSessionResponse{
			PairingSessionID:   "ps_partial",
			PairingCode:        "333444",
			ExpiresAt:          testNow.Add(10 * time.Minute),
			PollToken:          "poll-secret",
			PollTokenExpiresAt: testNow.Add(10 * time.Minute),
		},
		pollResponse: PollPairingSessionResponse{
			Status:                 "approved",
			DeviceID:               "dev_partial",
			IoTThingName:           "homesignal-dev-partial",
			AWSClaimCertificatePEM: "temporary claim cert",
			AWSClaimPrivateKeyPEM:  "temporary claim key",
		},
		finalizeErr: errors.New("homesignal finalization timeout"),
	}
	state := newTestRuntimeState(t, client, fakeFleetProvisioner{
		response: FleetProvisioningResponse{
			DeviceID:       "dev_partial",
			IoTThingName:   "homesignal-dev-partial",
			CertificateID:  "cert_partial",
			CertificatePEM: "durable cert",
		},
	})
	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("create pairing: %v", err)
	}
	if err := state.Enrollment.RunOnce(context.Background()); err == nil {
		t.Fatal("expected finalization failure")
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStatePairingPending {
		t.Fatalf("expected pending after finalization failure, got %s", snapshot.ClaimState)
	}
	if !contains(snapshot.DegradedReasons, "iot_provisioned_awaiting_homesignal_confirmation") {
		t.Fatalf("expected split-brain degraded reason, got %#v", snapshot.DegradedReasons)
	}
}

func TestPartialAWSProvisioningDoesNotExpireBackToUnclaimed(t *testing.T) {
	dir := t.TempDir()
	expired := testNow.Add(-time.Minute)
	record := DeviceRecord{
		SchemaVersion:         deviceRecordSchemaVersion,
		InstallationID:        "install-partial-expired",
		CreatedAt:             testNow.Add(-time.Hour),
		ClaimState:            ClaimStatePairingPending,
		PairingSessionID:      "ps_partial_expired",
		PairingCodeExpiry:     &expired,
		PollTokenExpiry:       &expired,
		PollTokenPath:         filepath.Join(dir, "secrets", "poll_token"),
		PrivateKeyPath:        filepath.Join(dir, "iot", "device.key"),
		CertificatePath:       filepath.Join(dir, "iot", "device.crt"),
		PendingDeviceID:       "dev_partial_expired",
		PendingIoTThingName:   "thing-partial-expired",
		PendingCertificateID:  "cert_partial_expired",
		PendingCertificateARN: "arn:cert",
	}
	if err := saveDeviceRecord(filepath.Join(dir, "device.json"), record); err != nil {
		t.Fatalf("save partial record: %v", err)
	}
	if err := writeSecretFile(record.PollTokenPath, []byte("expired-token")); err != nil {
		t.Fatalf("write poll token: %v", err)
	}
	state := newTestRuntimeStateInDir(t, dir, &fakeEnrollmentClient{finalizeErr: errors.New("expired token")}, fakeFleetProvisioner{})

	if err := state.Enrollment.RunOnce(context.Background()); err == nil {
		t.Fatal("expected finalization retry to fail")
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStatePairingPending {
		t.Fatalf("expected partial AWS state to remain pending, got %s", snapshot.ClaimState)
	}
	if !contains(snapshot.DegradedReasons, "iot_provisioned_awaiting_homesignal_confirmation") {
		t.Fatalf("expected finalization degraded reason, got %#v", snapshot.DegradedReasons)
	}
}

func TestPollClaimedWithoutLocalFinalizationDoesNotBecomeClaimed(t *testing.T) {
	client := &fakeEnrollmentClient{
		createResponse: CreatePairingSessionResponse{
			PairingSessionID:   "ps_claimed_poll",
			PairingCode:        "555666",
			ExpiresAt:          testNow.Add(10 * time.Minute),
			PollToken:          "poll-secret",
			PollTokenExpiresAt: testNow.Add(10 * time.Minute),
		},
		pollResponse: PollPairingSessionResponse{Status: "claimed"},
	}
	state := newTestRuntimeState(t, client, fakeFleetProvisioner{})
	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("create pairing: %v", err)
	}
	if err := state.Enrollment.RunOnce(context.Background()); err == nil {
		t.Fatal("expected claimed poll response without finalization to degrade")
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState == ClaimStateClaimed {
		t.Fatal("must not enter claimed without local finalization")
	}
	if !contains(snapshot.DegradedReasons, "homesignal_pairing_claimed_without_local_finalization") {
		t.Fatalf("expected claimed-without-finalization reason, got %#v", snapshot.DegradedReasons)
	}
}

func TestClaimedStateWithMissingCredentialFilesBootsSafe(t *testing.T) {
	dir := t.TempDir()
	claimedAt := testNow
	record := DeviceRecord{
		SchemaVersion:   deviceRecordSchemaVersion,
		InstallationID:  "install-claimed",
		CreatedAt:       testNow.Add(-time.Hour),
		ClaimState:      ClaimStateClaimed,
		DeviceID:        "dev_missing_files",
		IoTThingName:    "thing-missing-files",
		PrivateKeyPath:  filepath.Join(dir, "iot", "missing.key"),
		CertificatePath: filepath.Join(dir, "iot", "missing.crt"),
		ClaimedAt:       &claimedAt,
	}
	if err := saveDeviceRecord(filepath.Join(dir, "device.json"), record); err != nil {
		t.Fatalf("save claimed record: %v", err)
	}
	state := newTestRuntimeStateInDir(t, dir, &fakeEnrollmentClient{}, fakeFleetProvisioner{})

	if err := state.Enrollment.RunOnce(context.Background()); err == nil {
		t.Fatal("expected missing credential files to degrade")
	}
	snapshot := state.Enrollment.Snapshot()
	if snapshot.ClaimState != ClaimStateUnclaimed {
		t.Fatalf("expected safe unclaimed state, got %s", snapshot.ClaimState)
	}
}

func TestRevokedDeviceStatusTransitionsToRevoked(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "iot", "device.key")
	certPath := filepath.Join(dir, "iot", "device.crt")
	if _, err := ensureDevicePrivateKey(keyPath); err != nil {
		t.Fatalf("create key: %v", err)
	}
	if err := writeSecretFile(certPath, []byte("durable cert")); err != nil {
		t.Fatalf("create cert: %v", err)
	}
	claimedAt := testNow
	record := DeviceRecord{
		SchemaVersion:   deviceRecordSchemaVersion,
		InstallationID:  "install-revoked",
		CreatedAt:       testNow.Add(-time.Hour),
		ClaimState:      ClaimStateClaimed,
		DeviceID:        "dev_revoked",
		IoTThingName:    "thing-revoked",
		PrivateKeyPath:  keyPath,
		CertificatePath: certPath,
		ClaimedAt:       &claimedAt,
	}
	if err := saveDeviceRecord(filepath.Join(dir, "device.json"), record); err != nil {
		t.Fatalf("save claimed record: %v", err)
	}
	state := newTestRuntimeStateInDir(t, dir, &fakeEnrollmentClient{deviceStatusResponse: DeviceStatusResponse{Status: "revoked"}}, fakeFleetProvisioner{})

	if err := state.Enrollment.RunOnce(context.Background()); err != nil {
		t.Fatalf("run status check: %v", err)
	}
	if got := state.Enrollment.Snapshot().ClaimState; got != ClaimStateRevoked {
		t.Fatalf("expected revoked, got %s", got)
	}
}

func newTestRuntimeState(t *testing.T, client HomeSignalEnrollmentClient, provisioner FleetProvisioningClient) RuntimeState {
	t.Helper()
	dir := t.TempDir()
	return newTestRuntimeStateInDir(t, dir, client, provisioner)
}

func newTestRuntimeStateInDir(t *testing.T, dir string, client HomeSignalEnrollmentClient, provisioner FleetProvisioningClient) RuntimeState {
	t.Helper()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	options := []byte(`{"cloud_base_url":"https://cloud.test","aws_iot_endpoint":"iot.test.amazonaws.com","aws_region":"us-east-1","fleet_provisioning_template":"test-template"}`)
	if err := os.WriteFile(filepath.Join(dataDir, "options.json"), options, 0o600); err != nil {
		t.Fatalf("write options: %v", err)
	}
	state, err := loadRuntimeStateWithClients(dir, dataDir, "", client, provisioner, func() time.Time { return testNow })
	if err != nil {
		t.Fatalf("load runtime state: %v", err)
	}
	return state
}

type fakeEnrollmentClient struct {
	createResponse       CreatePairingSessionResponse
	createErr            error
	pollResponse         PollPairingSessionResponse
	pollErr              error
	finalizeResponse     FinalizeClaimResponse
	finalizeErr          error
	deviceStatusResponse DeviceStatusResponse
	deviceStatusErr      error
}

func (f *fakeEnrollmentClient) CreatePairingSession(context.Context, CreatePairingSessionRequest) (CreatePairingSessionResponse, error) {
	if f.createErr != nil {
		return CreatePairingSessionResponse{}, f.createErr
	}
	if f.createResponse.PairingSessionID == "" {
		f.createResponse = CreatePairingSessionResponse{
			PairingSessionID:   "ps_default",
			PairingCode:        "000111",
			ExpiresAt:          testNow.Add(10 * time.Minute),
			PollToken:          "poll-token",
			PollTokenExpiresAt: testNow.Add(10 * time.Minute),
		}
	}
	return f.createResponse, nil
}

func (f *fakeEnrollmentClient) PollPairingSession(context.Context, PollPairingSessionRequest) (PollPairingSessionResponse, error) {
	if f.pollErr != nil {
		return PollPairingSessionResponse{}, f.pollErr
	}
	if f.pollResponse.Status == "" {
		f.pollResponse = PollPairingSessionResponse{Status: "pending", ExpiresAt: testNow.Add(10 * time.Minute)}
	}
	return f.pollResponse, nil
}

func (f *fakeEnrollmentClient) FinalizeClaim(context.Context, FinalizeClaimRequest) (FinalizeClaimResponse, error) {
	if f.finalizeErr != nil {
		return FinalizeClaimResponse{}, f.finalizeErr
	}
	if f.finalizeResponse.Status == "" {
		f.finalizeResponse = FinalizeClaimResponse{Status: "claimed", ClaimedAt: testNow.Add(time.Minute)}
	}
	return f.finalizeResponse, nil
}

func (f *fakeEnrollmentClient) DeviceStatus(context.Context, DeviceStatusRequest) (DeviceStatusResponse, error) {
	if f.deviceStatusErr != nil {
		return DeviceStatusResponse{}, f.deviceStatusErr
	}
	if f.deviceStatusResponse.Status == "" {
		f.deviceStatusResponse = DeviceStatusResponse{Status: "claimed"}
	}
	return f.deviceStatusResponse, nil
}

type fakeFleetProvisioner struct {
	response FleetProvisioningResponse
	err      error
}

func (f fakeFleetProvisioner) Provision(context.Context, FleetProvisioningRequest) (FleetProvisioningResponse, error) {
	if f.err != nil {
		return FleetProvisioningResponse{}, f.err
	}
	if f.response.CertificateID == "" {
		f.response = FleetProvisioningResponse{
			DeviceID:       "dev_default",
			IoTThingName:   "thing-default",
			CertificateID:  "cert_default",
			CertificatePEM: "durable cert",
		}
	}
	return f.response, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func assertFileMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != expected {
		t.Fatalf("expected %s mode %v, got %v", path, expected, got)
	}
}
