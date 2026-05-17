package ports

import (
	"context"
	"testing"
	"time"
)

func TestFakeDeviceRegistryRepositoryResolvesCredential(t *testing.T) {
	repo := NewFakeDeviceRegistryRepository()
	identity := CredentialIdentity{
		CertificateFingerprint: "SHA256:abc",
		CertificateSerial:      "serial",
		CertificateIssuer:      "issuer",
	}
	repo.Authorities[identity] = DeviceAuthority{
		DeviceID:         "dev_123",
		AccountID:        "acct_123",
		SiteID:           "site_123",
		CredentialStatus: "ACTIVE",
	}

	authority, err := repo.ResolveCredential(context.Background(), identity)
	if err != nil {
		t.Fatalf("ResolveCredential returned error: %v", err)
	}
	if authority.DeviceID != "dev_123" {
		t.Fatalf("expected dev_123, got %s", authority.DeviceID)
	}
}

func TestFakeTelemetryRepositoryStoresLatestAndSparseEvent(t *testing.T) {
	repo := NewFakeTelemetryRepository()
	state := LatestTelemetryState{
		DeviceID:      "dev_123",
		AccountID:     "acct_123",
		SiteID:        "site_123",
		MessageID:     "msg_123",
		MessageType:   "telemetry",
		SchemaType:    "device.health_snapshot",
		SchemaVersion: 1,
		MaterialHash:  "hash",
		ObservedAt:    time.Unix(10, 0).UTC(),
		ReceivedAt:    time.Unix(11, 0).UTC(),
	}
	if err := repo.UpsertLatestState(context.Background(), state); err != nil {
		t.Fatalf("UpsertLatestState returned error: %v", err)
	}
	if err := repo.InsertTelemetryEvent(context.Background(), TelemetryEvent{LatestTelemetryState: state}); err != nil {
		t.Fatalf("InsertTelemetryEvent returned error: %v", err)
	}

	if repo.LatestStates["dev_123"].MessageID != "msg_123" {
		t.Fatalf("expected latest state to be stored")
	}
	if len(repo.Events) != 1 {
		t.Fatalf("expected one sparse telemetry event, got %d", len(repo.Events))
	}
}

func TestFakeAuditRepositoryRecordsEvent(t *testing.T) {
	repo := &FakeAuditRepository{}
	err := repo.RecordAuditEvent(context.Background(), AuditEvent{
		ActorType:  "user",
		ActorID:    "user_123",
		Action:     "claim_invite.create",
		Resource:   "site/site_123",
		OccurredAt: time.Unix(12, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("RecordAuditEvent returned error: %v", err)
	}
	if len(repo.Events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(repo.Events))
	}
}
