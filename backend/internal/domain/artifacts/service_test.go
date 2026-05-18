package artifacts

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestServiceCreatesUploadSlotWithServerGeneratedObjectKey(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	issuer := &fakeIssuer{}
	service := newTestService(repo, issuer, now)
	expectedSize := int64(1024)

	response, err := service.CreateUpload(context.Background(), CreateUploadRequest{
		AccountID:              "acct_123",
		SiteID:                 "site_123",
		DeviceID:               "dev_123",
		CommandID:              "cmd_123",
		Purpose:                PurposeBackupArtifact,
		RequestedBySubjectType: "user",
		RequestedBySubjectID:   "usr_123",
		ContentType:            "application/gzip",
		ExpectedSizeBytes:      &expectedSize,
		LocalArtifactRef:       "../../backup.tar.gz",
		RedactionProfile:       "backup_v1",
	})
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}
	if len(repo.slots) != 1 {
		t.Fatalf("expected one stored slot, got %d", len(repo.slots))
	}
	slot := repo.slots[0]
	if slot.ObjectKey != "artifacts/accounts/acct_123/sites/site_123/devices/dev_123/backup_artifact/2026/05/18/art_123" {
		t.Fatalf("unexpected object key %q", slot.ObjectKey)
	}
	if strings.Contains(slot.ObjectKey, "backup.tar.gz") || strings.Contains(slot.ObjectKey, "..") {
		t.Fatalf("object key should be server-generated and not local-path-derived: %q", slot.ObjectKey)
	}
	if slot.Status != StatusPendingUpload {
		t.Fatalf("expected pending upload status, got %s", slot.Status)
	}
	if got := slot.ExpiresAt.Sub(now); got != 15*time.Minute {
		t.Fatalf("expected 15m TTL, got %s", got)
	}
	if response.Capability.URL == "" {
		t.Fatalf("expected signed upload capability URL")
	}
	if issuer.requests[0].ObjectKey != slot.ObjectKey {
		t.Fatalf("issuer did not receive generated object key")
	}
}

func TestServiceRejectsUnsupportedPurpose(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	service := newTestService(&fakeRepository{}, &fakeIssuer{}, now)

	_, err := service.CreateUpload(context.Background(), CreateUploadRequest{
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		Purpose:     "topology_snapshot",
		ContentType: "application/json",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported artifact purpose") {
		t.Fatalf("expected unsupported purpose error, got %v", err)
	}
}

func TestServiceRejectsOversizeAndUnsupportedContentType(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	service := newTestService(&fakeRepository{}, &fakeIssuer{}, now)
	oversize := int64(6 * 1024 * 1024)

	_, err := service.CreateUpload(context.Background(), CreateUploadRequest{
		AccountID:         "acct_123",
		SiteID:            "site_123",
		DeviceID:          "dev_123",
		Purpose:           PurposeErrorLogBundle,
		ContentType:       "text/plain",
		ExpectedSizeBytes: &oversize,
	})
	if err == nil || !strings.Contains(err.Error(), "expected size exceeds") {
		t.Fatalf("expected oversize error, got %v", err)
	}

	_, err = service.CreateUpload(context.Background(), CreateUploadRequest{
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		Purpose:     PurposeErrorLogBundle,
		ContentType: "application/octet-stream",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported content type") {
		t.Fatalf("expected content type error, got %v", err)
	}
}

func TestServiceDoesNotPersistOrPrintSignedURL(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	issuer := &fakeIssuer{url: "https://bucket.example/upload?X-Amz-Signature=supersecret"}
	service := newTestService(repo, issuer, now)

	response, err := service.CreateUpload(context.Background(), CreateUploadRequest{
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		Purpose:     PurposeDiagnosticBundle,
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}
	if strings.Contains(fmt.Sprintf("%v", response), "supersecret") {
		t.Fatalf("response string should redact signed URL: %v", response)
	}
	if strings.Contains(fmt.Sprintf("%v", response.Capability), "supersecret") {
		t.Fatalf("capability string should redact signed URL: %v", response.Capability)
	}
	if len(repo.slots) != 1 {
		t.Fatalf("expected stored slot")
	}
	if strings.Contains(fmt.Sprintf("%#v", repo.slots[0]), "supersecret") {
		t.Fatalf("stored upload slot must not contain signed URL: %#v", repo.slots[0])
	}
}

func TestServiceRejectsPathLikeAuthoritySegments(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	service := newTestService(&fakeRepository{}, &fakeIssuer{}, now)

	_, err := service.CreateUpload(context.Background(), CreateUploadRequest{
		AccountID:   "acct/123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		Purpose:     PurposeDiagnosticBundle,
		ContentType: "application/json",
	})
	if err == nil || !strings.Contains(err.Error(), "account_id is required") {
		t.Fatalf("expected invalid account segment error, got %v", err)
	}
}

func newTestService(repo *fakeRepository, issuer *fakeIssuer, now time.Time) Service {
	return Service{
		Repository: repo,
		Issuer:     issuer,
		Bucket:     "homesignal-staging-artifacts",
		IDGenerator: func() string {
			return "art_123"
		},
		Clock: func() time.Time {
			return now
		},
	}
}

type fakeRepository struct {
	slots []UploadSlot
}

func (r *fakeRepository) CreateUploadSlot(_ context.Context, slot UploadSlot) error {
	r.slots = append(r.slots, slot)
	return nil
}

type fakeIssuer struct {
	url      string
	requests []UploadURLRequest
}

func (i *fakeIssuer) IssueUploadURL(_ context.Context, req UploadURLRequest) (UploadCapability, error) {
	i.requests = append(i.requests, req)
	url := i.url
	if url == "" {
		url = "https://bucket.example/upload?X-Amz-Signature=fixture"
	}
	return UploadCapability{
		UploadID:    req.UploadID,
		Method:      req.Method,
		URL:         url,
		ExpiresAt:   req.ExpiresAt,
		ContentType: req.ContentType,
		MaxBytes:    req.MaxSizeBytes,
	}, nil
}
