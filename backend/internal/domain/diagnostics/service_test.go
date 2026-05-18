package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/artifacts"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/commands"
)

func TestServiceStartsDebugSessionWithDefaultTTLAndAudit(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	commandCreator := &fakeCommandCreator{}
	audit := &fakeAuditRecorder{}
	service := newTestService(repo, commandCreator, audit, now)

	session, command, err := service.StartDebugSession(context.Background(), StartDebugSessionRequest{
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		RequestedBy: Actor{Type: "support", ID: "usr_support"},
	})
	if err != nil {
		t.Fatalf("start debug session: %v", err)
	}
	if session.DebugSessionID != "dbg_123" {
		t.Fatalf("unexpected session id %q", session.DebugSessionID)
	}
	if got := session.ExpiresAt.Sub(now); got != time.Hour {
		t.Fatalf("expected default 1h TTL, got %s", got)
	}
	if session.RedactionProfile != "default_v1" || session.MaxBytes != DefaultMaxBytes {
		t.Fatalf("unexpected defaults %#v", session)
	}
	if command.CommandType != commands.TypeEnableDebugCapture {
		t.Fatalf("expected enable_debug_capture command, got %s", command.CommandType)
	}
	if !strings.Contains(string(commandCreator.requests[0].Payload), `"debug_session_id":"dbg_123"`) {
		t.Fatalf("expected debug session id in command payload: %s", commandCreator.requests[0].Payload)
	}
	if len(repo.sessions) != 1 {
		t.Fatalf("expected one stored session, got %d", len(repo.sessions))
	}
	if len(audit.events) != 1 || audit.events[0].Action != "debug_session.start" {
		t.Fatalf("expected start audit event, got %#v", audit.events)
	}
}

func TestServiceRejectsCustomerDebugSessionStart(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	service := newTestService(&fakeRepository{}, &fakeCommandCreator{}, &fakeAuditRecorder{}, now)

	_, _, err := service.StartDebugSession(context.Background(), StartDebugSessionRequest{
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		RequestedBy: Actor{Type: "customer", ID: "usr_123"},
	})
	if err == nil || !strings.Contains(err.Error(), "internal/support only") {
		t.Fatalf("expected customer actor rejection, got %v", err)
	}
}

func TestServiceRejectsTooLongDebugSessionTTL(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	service := newTestService(&fakeRepository{}, &fakeCommandCreator{}, &fakeAuditRecorder{}, now)

	_, _, err := service.StartDebugSession(context.Background(), StartDebugSessionRequest{
		AccountID:   "acct_123",
		SiteID:      "site_123",
		DeviceID:    "dev_123",
		RequestedBy: Actor{Type: "internal", ID: "svc_support"},
		TTL:         25 * time.Hour,
	})
	if err == nil || !strings.Contains(err.Error(), "TTL") {
		t.Fatalf("expected TTL rejection, got %v", err)
	}
}

func TestServiceCreatesDiagnosticBundleMetadata(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	service := newTestService(repo, &fakeCommandCreator{}, &fakeAuditRecorder{}, now)

	bundle, err := service.CreateDiagnosticBundle(context.Background(), CreateBundleRequest{
		AccountID:        "acct_123",
		SiteID:           "site_123",
		DeviceID:         "dev_123",
		DebugSessionID:   "dbg_123",
		CommandID:        "cmd_123",
		ArtifactUploadID: "art_123",
		Purpose:          artifacts.PurposeDebugBundle,
	})
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	if bundle.DiagnosticBundleID != "diag_123" {
		t.Fatalf("unexpected bundle id %q", bundle.DiagnosticBundleID)
	}
	if bundle.Status != BundleStatusRequested || bundle.RedactionProfile != DefaultRedaction {
		t.Fatalf("unexpected bundle defaults %#v", bundle)
	}
	if len(repo.bundles) != 1 {
		t.Fatalf("expected one stored bundle, got %d", len(repo.bundles))
	}
}

func TestServiceRequiresBundleCommandAndArtifactReferences(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	service := newTestService(&fakeRepository{}, &fakeCommandCreator{}, &fakeAuditRecorder{}, now)

	_, err := service.CreateDiagnosticBundle(context.Background(), CreateBundleRequest{
		AccountID: "acct_123",
		SiteID:    "site_123",
		DeviceID:  "dev_123",
		Purpose:   artifacts.PurposeDebugBundle,
	})
	if err == nil || !strings.Contains(err.Error(), "command_id and artifact_upload_id") {
		t.Fatalf("expected missing reference error, got %v", err)
	}
}

func newTestService(repo *fakeRepository, commandCreator *fakeCommandCreator, audit *fakeAuditRecorder, now time.Time) Service {
	return Service{
		Repository:     repo,
		CommandCreator: commandCreator,
		AuditRecorder:  audit,
		SessionIDGenerator: func() string {
			return "dbg_123"
		},
		BundleIDGenerator: func() string {
			return "diag_123"
		},
		Clock: func() time.Time {
			return now
		},
	}
}

type fakeRepository struct {
	sessions []DebugSession
	bundles  []DiagnosticBundle
}

func (r *fakeRepository) CreateDebugSession(_ context.Context, session DebugSession) error {
	r.sessions = append(r.sessions, session)
	return nil
}

func (r *fakeRepository) CreateDiagnosticBundle(_ context.Context, bundle DiagnosticBundle) error {
	r.bundles = append(r.bundles, bundle)
	return nil
}

type fakeCommandCreator struct {
	requests []commands.CreateRequest
}

func (c *fakeCommandCreator) Create(_ context.Context, req commands.CreateRequest) (commands.Command, error) {
	c.requests = append(c.requests, req)
	return commands.Command{
		CommandID:   "cmd_123",
		AccountID:   req.AccountID,
		SiteID:      req.SiteID,
		DeviceID:    req.DeviceID,
		CommandType: req.CommandType,
		Status:      commands.StatusQueued,
	}, nil
}

type fakeAuditRecorder struct {
	events []AuditEvent
}

func (r *fakeAuditRecorder) Record(_ context.Context, event AuditEvent) error {
	r.events = append(r.events, event)
	return nil
}
