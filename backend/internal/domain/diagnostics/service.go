package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/artifacts"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/commands"
)

const (
	DefaultDebugTTL  = time.Hour
	MaxDebugTTL      = 24 * time.Hour
	DefaultMaxBytes  = int64(25 * 1024 * 1024)
	DefaultRedaction = "default_v1"

	SessionStatusActive  SessionStatus = "active"
	SessionStatusStopped SessionStatus = "stopped"
	SessionStatusExpired SessionStatus = "expired"

	BundleStatusRequested BundleStatus = "requested"
	BundleStatusUploaded  BundleStatus = "uploaded"
	BundleStatusFailed    BundleStatus = "failed"
	BundleStatusCanceled  BundleStatus = "canceled"
)

type SessionStatus string
type BundleStatus string

type Actor struct {
	Type string
	ID   string
}

type DebugSession struct {
	DebugSessionID   string
	AccountID        string
	SiteID           string
	DeviceID         string
	CommandID        string
	Status           SessionStatus
	RequestedBy      Actor
	RedactionProfile string
	MaxBytes         int64
	Categories       []string
	Metadata         json.RawMessage
	StartedAt        time.Time
	ExpiresAt        time.Time
	EndedAt          *time.Time
}

type DiagnosticBundle struct {
	DiagnosticBundleID string
	AccountID          string
	SiteID             string
	DeviceID           string
	DebugSessionID     string
	CommandID          string
	ArtifactUploadID   string
	Status             BundleStatus
	Purpose            artifacts.Purpose
	RedactionProfile   string
	ReasonCode         string
	Metadata           json.RawMessage
	RequestedAt        time.Time
	CompletedAt        *time.Time
}

type StartDebugSessionRequest struct {
	AccountID        string
	SiteID           string
	DeviceID         string
	RequestedBy      Actor
	TTL              time.Duration
	RedactionProfile string
	MaxBytes         int64
	Categories       []string
	Metadata         json.RawMessage
}

type CreateBundleRequest struct {
	AccountID        string
	SiteID           string
	DeviceID         string
	DebugSessionID   string
	CommandID        string
	ArtifactUploadID string
	Purpose          artifacts.Purpose
	RedactionProfile string
	Metadata         json.RawMessage
}

type Repository interface {
	CreateDebugSession(ctx context.Context, session DebugSession) error
	CreateDiagnosticBundle(ctx context.Context, bundle DiagnosticBundle) error
}

type CommandCreator interface {
	Create(ctx context.Context, req commands.CreateRequest) (commands.Command, error)
}

type AuditRecorder interface {
	Record(ctx context.Context, event AuditEvent) error
}

type AuditEvent struct {
	Actor      Actor
	Action     string
	AccountID  string
	SiteID     string
	DeviceID   string
	Metadata   json.RawMessage
	OccurredAt time.Time
}

type IDGenerator func() string
type Clock func() time.Time

type Service struct {
	Repository         Repository
	CommandCreator     CommandCreator
	AuditRecorder      AuditRecorder
	SessionIDGenerator IDGenerator
	BundleIDGenerator  IDGenerator
	Clock              Clock
}

func (s Service) StartDebugSession(ctx context.Context, req StartDebugSessionRequest) (DebugSession, commands.Command, error) {
	if s.Repository == nil {
		return DebugSession{}, commands.Command{}, fmt.Errorf("diagnostics repository is required")
	}
	if s.CommandCreator == nil {
		return DebugSession{}, commands.Command{}, fmt.Errorf("command creator is required")
	}
	if !isInternalActor(req.RequestedBy) {
		return DebugSession{}, commands.Command{}, fmt.Errorf("debug sessions are internal/support only")
	}
	now := s.now()
	ttl := req.TTL
	if ttl == 0 {
		ttl = DefaultDebugTTL
	}
	if ttl < 0 || ttl > MaxDebugTTL {
		return DebugSession{}, commands.Command{}, fmt.Errorf("debug session TTL must be between 0 and %s", MaxDebugTTL)
	}
	sessionID, err := s.newID(s.SessionIDGenerator, "debug session id")
	if err != nil {
		return DebugSession{}, commands.Command{}, err
	}
	accountID := strings.TrimSpace(req.AccountID)
	siteID := strings.TrimSpace(req.SiteID)
	deviceID := strings.TrimSpace(req.DeviceID)
	if accountID == "" || siteID == "" || deviceID == "" {
		return DebugSession{}, commands.Command{}, fmt.Errorf("account_id, site_id, and device_id are required")
	}
	redactionProfile := strings.TrimSpace(req.RedactionProfile)
	if redactionProfile == "" {
		redactionProfile = DefaultRedaction
	}
	maxBytes := req.MaxBytes
	if maxBytes == 0 {
		maxBytes = DefaultMaxBytes
	}
	if maxBytes < 0 || maxBytes > DefaultMaxBytes {
		return DebugSession{}, commands.Command{}, fmt.Errorf("debug session max bytes exceeds default limit")
	}
	categories := normalizeCategories(req.Categories)
	if len(categories) == 0 {
		categories = []string{"app_runtime_status"}
	}
	metadata := normalizeJSON(req.Metadata)
	if !json.Valid(metadata) {
		return DebugSession{}, commands.Command{}, fmt.Errorf("metadata must be valid JSON")
	}
	expiresAt := now.Add(ttl)
	payload, err := json.Marshal(map[string]any{
		"debug_session_id":  sessionID,
		"expires_at":        expiresAt.Format(time.RFC3339),
		"redaction_profile": redactionProfile,
		"max_bytes":         maxBytes,
		"categories":        categories,
	})
	if err != nil {
		return DebugSession{}, commands.Command{}, fmt.Errorf("marshal debug command payload: %w", err)
	}
	command, err := s.CommandCreator.Create(ctx, commands.CreateRequest{
		AccountID:   accountID,
		SiteID:      siteID,
		DeviceID:    deviceID,
		CommandType: commands.TypeEnableDebugCapture,
		Payload:     payload,
	})
	if err != nil {
		return DebugSession{}, commands.Command{}, fmt.Errorf("create debug command: %w", err)
	}
	session := DebugSession{
		DebugSessionID:   sessionID,
		AccountID:        accountID,
		SiteID:           siteID,
		DeviceID:         deviceID,
		CommandID:        command.CommandID,
		Status:           SessionStatusActive,
		RequestedBy:      req.RequestedBy,
		RedactionProfile: redactionProfile,
		MaxBytes:         maxBytes,
		Categories:       categories,
		Metadata:         metadata,
		StartedAt:        now,
		ExpiresAt:        expiresAt,
	}
	if err := s.Repository.CreateDebugSession(ctx, session); err != nil {
		return DebugSession{}, commands.Command{}, fmt.Errorf("create debug session: %w", err)
	}
	if err := s.audit(ctx, AuditEvent{
		Actor:      req.RequestedBy,
		Action:     "debug_session.start",
		AccountID:  accountID,
		SiteID:     siteID,
		DeviceID:   deviceID,
		Metadata:   json.RawMessage(fmt.Sprintf(`{"debug_session_id":%q,"command_id":%q}`, session.DebugSessionID, session.CommandID)),
		OccurredAt: now,
	}); err != nil {
		return DebugSession{}, commands.Command{}, err
	}
	return session, command, nil
}

func (s Service) CreateDiagnosticBundle(ctx context.Context, req CreateBundleRequest) (DiagnosticBundle, error) {
	if s.Repository == nil {
		return DiagnosticBundle{}, fmt.Errorf("diagnostics repository is required")
	}
	bundleID, err := s.newID(s.BundleIDGenerator, "diagnostic bundle id")
	if err != nil {
		return DiagnosticBundle{}, err
	}
	accountID := strings.TrimSpace(req.AccountID)
	siteID := strings.TrimSpace(req.SiteID)
	deviceID := strings.TrimSpace(req.DeviceID)
	commandID := strings.TrimSpace(req.CommandID)
	artifactUploadID := strings.TrimSpace(req.ArtifactUploadID)
	if accountID == "" || siteID == "" || deviceID == "" {
		return DiagnosticBundle{}, fmt.Errorf("account_id, site_id, and device_id are required")
	}
	if commandID == "" || artifactUploadID == "" {
		return DiagnosticBundle{}, fmt.Errorf("command_id and artifact_upload_id are required")
	}
	if !allowedBundlePurpose(req.Purpose) {
		return DiagnosticBundle{}, fmt.Errorf("unsupported diagnostic bundle purpose %q", req.Purpose)
	}
	redactionProfile := strings.TrimSpace(req.RedactionProfile)
	if redactionProfile == "" {
		redactionProfile = DefaultRedaction
	}
	metadata := normalizeJSON(req.Metadata)
	if !json.Valid(metadata) {
		return DiagnosticBundle{}, fmt.Errorf("metadata must be valid JSON")
	}
	bundle := DiagnosticBundle{
		DiagnosticBundleID: bundleID,
		AccountID:          accountID,
		SiteID:             siteID,
		DeviceID:           deviceID,
		DebugSessionID:     strings.TrimSpace(req.DebugSessionID),
		CommandID:          commandID,
		ArtifactUploadID:   artifactUploadID,
		Status:             BundleStatusRequested,
		Purpose:            req.Purpose,
		RedactionProfile:   redactionProfile,
		Metadata:           metadata,
		RequestedAt:        s.now(),
	}
	if err := s.Repository.CreateDiagnosticBundle(ctx, bundle); err != nil {
		return DiagnosticBundle{}, fmt.Errorf("create diagnostic bundle: %w", err)
	}
	return bundle, nil
}

func (s Service) audit(ctx context.Context, event AuditEvent) error {
	if s.AuditRecorder == nil {
		return nil
	}
	if err := s.AuditRecorder.Record(ctx, event); err != nil {
		return fmt.Errorf("record diagnostics audit event: %w", err)
	}
	return nil
}

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func (s Service) newID(generator IDGenerator, name string) (string, error) {
	if generator == nil {
		return "", fmt.Errorf("%s generator is required", name)
	}
	id := strings.TrimSpace(generator())
	if id == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return id, nil
}

func isInternalActor(actor Actor) bool {
	switch strings.TrimSpace(actor.Type) {
	case "internal", "support":
		return strings.TrimSpace(actor.ID) != ""
	default:
		return false
	}
}

func normalizeCategories(categories []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" || seen[category] {
			continue
		}
		seen[category] = true
		out = append(out, category)
	}
	return out
}

func allowedBundlePurpose(purpose artifacts.Purpose) bool {
	switch purpose {
	case artifacts.PurposeDiagnosticBundle, artifacts.PurposeDebugBundle, artifacts.PurposeErrorLogBundle:
		return true
	default:
		return false
	}
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	clone := make(json.RawMessage, len(value))
	copy(clone, value)
	return clone
}
