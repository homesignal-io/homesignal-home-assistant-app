package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type AgentArtifactClient interface {
	RequestArtifactUpload(context.Context, ArtifactUploadRequest) (ArtifactUploadSession, error)
	CompleteArtifactUpload(context.Context, ArtifactUploadCompleteRequest) error
	RecordCommandResult(context.Context, CommandResultRequest) error
}

type LocalArtifactGenerator interface {
	GenerateArtifact(context.Context, GenerateArtifactRequest) (GeneratedArtifact, error)
}

type SignedURLUploader interface {
	UploadArtifact(context.Context, ArtifactUploadSession, GeneratedArtifact) error
}

type AgentAlarmSink interface {
	EmitAgentAlarm(context.Context, AgentAlarm) error
}

type ArtifactCommandPayload struct {
	Purpose          string `json:"purpose"`
	LocalArtifactRef string `json:"local_artifact_ref"`
	ContentType      string `json:"content_type"`
	MaxBytes         int64  `json:"max_bytes"`
	RedactionProfile string `json:"redaction_profile,omitempty"`
}

type GenerateArtifactRequest struct {
	Purpose          string
	LocalArtifactRef string
	RedactionProfile string
}

type GeneratedArtifact struct {
	Purpose          string
	LocalArtifactRef string
	ContentType      string
	Bytes            []byte
	SHA256           string
	GeneratedAt      time.Time
}

type ArtifactUploadRequest struct {
	CommandID        string `json:"command_id"`
	Purpose          string `json:"purpose"`
	LocalArtifactRef string `json:"local_artifact_ref"`
	ContentType      string `json:"content_type"`
	SizeBytes        int64  `json:"size_bytes"`
	SHA256           string `json:"sha256"`
}

type ArtifactUploadSession struct {
	UploadID    string
	Method      string
	URL         string
	ContentType string
	MaxBytes    int64
	ExpiresAt   time.Time
}

type ArtifactUploadCompleteRequest struct {
	UploadID  string `json:"upload_id"`
	CommandID string `json:"command_id"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type CommandResultRequest struct {
	CommandID  string          `json:"command_id"`
	Status     string          `json:"status"`
	ResultType string          `json:"result_type"`
	ReasonCode string          `json:"reason_code,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	FinishedAt time.Time       `json:"finished_at"`
}

type AgentAlarm struct {
	AlarmType         string    `json:"alarm_type"`
	Severity          string    `json:"severity"`
	CommandID         string    `json:"command_id,omitempty"`
	ArtifactPurpose   string    `json:"artifact_purpose,omitempty"`
	ReasonCode        string    `json:"reason_code"`
	MoreLogsAvailable bool      `json:"more_logs_available"`
	EmittedAt         time.Time `json:"emitted_at"`
}

type ArtifactUploadHandler struct {
	Client    AgentArtifactClient
	Generator LocalArtifactGenerator
	Uploader  SignedURLUploader
	AlarmSink AgentAlarmSink
	Now       func() time.Time
}

func (h ArtifactUploadHandler) HandleArtifactCommand(ctx context.Context, detail AgentCommandDetail) error {
	if h.Client == nil {
		return fmt.Errorf("agent artifact client is required")
	}
	if h.Generator == nil {
		return fmt.Errorf("local artifact generator is required")
	}
	if h.Uploader == nil {
		return fmt.Errorf("signed URL uploader is required")
	}
	if detail.CommandType != commandTypeUploadArtifact {
		return fmt.Errorf("unsupported artifact command type %q", detail.CommandType)
	}
	payload, err := parseArtifactCommandPayload(detail.Payload)
	if err != nil {
		return h.failCommand(ctx, detail.CommandID, "invalid_artifact_command", nil)
	}
	artifact, err := h.Generator.GenerateArtifact(ctx, GenerateArtifactRequest{
		Purpose:          payload.Purpose,
		LocalArtifactRef: payload.LocalArtifactRef,
		RedactionProfile: payload.RedactionProfile,
	})
	if err != nil {
		return h.failCommand(ctx, detail.CommandID, "unknown_local_artifact_ref", nil)
	}
	if artifact.SHA256 == "" {
		sum := sha256.Sum256(artifact.Bytes)
		artifact.SHA256 = hex.EncodeToString(sum[:])
	}
	if artifact.ContentType != payload.ContentType {
		return h.failCommand(ctx, detail.CommandID, "artifact_content_type_mismatch", nil)
	}
	if payload.MaxBytes > 0 && int64(len(artifact.Bytes)) > payload.MaxBytes {
		return h.failCommand(ctx, detail.CommandID, "artifact_exceeds_command_limit", nil)
	}
	session, err := h.Client.RequestArtifactUpload(ctx, ArtifactUploadRequest{
		CommandID:        detail.CommandID,
		Purpose:          payload.Purpose,
		LocalArtifactRef: payload.LocalArtifactRef,
		ContentType:      artifact.ContentType,
		SizeBytes:        int64(len(artifact.Bytes)),
		SHA256:           artifact.SHA256,
	})
	if err != nil {
		return fmt.Errorf("request artifact upload: %w", err)
	}
	if err := validateUploadSession(session, artifact, h.now()); err != nil {
		return h.failCommand(ctx, detail.CommandID, "invalid_upload_session", nil)
	}
	if err := h.Uploader.UploadArtifact(ctx, session, artifact); err != nil {
		alarmErr := h.emitUploadFailureAlarm(ctx, detail.CommandID, payload.Purpose)
		return h.failCommand(ctx, detail.CommandID, "artifact_upload_failed", alarmErr)
	}
	if err := h.Client.CompleteArtifactUpload(ctx, ArtifactUploadCompleteRequest{
		UploadID:  session.UploadID,
		CommandID: detail.CommandID,
		SizeBytes: int64(len(artifact.Bytes)),
		SHA256:    artifact.SHA256,
	}); err != nil {
		return fmt.Errorf("complete artifact upload: %w", err)
	}
	resultPayload, err := json.Marshal(map[string]any{
		"upload_id":           session.UploadID,
		"purpose":             payload.Purpose,
		"local_artifact_ref":  payload.LocalArtifactRef,
		"size_bytes":          len(artifact.Bytes),
		"sha256":              artifact.SHA256,
		"content_type":        artifact.ContentType,
		"redaction_profile":   payload.RedactionProfile,
		"more_logs_available": false,
	})
	if err != nil {
		return fmt.Errorf("marshal artifact result: %w", err)
	}
	return h.Client.RecordCommandResult(ctx, CommandResultRequest{
		CommandID:  detail.CommandID,
		Status:     "succeeded",
		ResultType: "artifact_upload",
		Payload:    resultPayload,
		FinishedAt: h.now(),
	})
}

func (h ArtifactUploadHandler) failCommand(ctx context.Context, commandID string, reasonCode string, prior error) error {
	err := h.Client.RecordCommandResult(ctx, CommandResultRequest{
		CommandID:  commandID,
		Status:     "failed",
		ResultType: "artifact_upload",
		ReasonCode: reasonCode,
		Payload:    json.RawMessage(`{"more_logs_available":false}`),
		FinishedAt: h.now(),
	})
	if prior != nil && err != nil {
		return fmt.Errorf("%v; record command failure: %w", prior, err)
	}
	if prior != nil {
		return prior
	}
	return err
}

func (h ArtifactUploadHandler) emitUploadFailureAlarm(ctx context.Context, commandID string, purpose string) error {
	if h.AlarmSink == nil {
		return nil
	}
	return h.AlarmSink.EmitAgentAlarm(ctx, AgentAlarm{
		AlarmType:         "artifact_upload_failed",
		Severity:          "warning",
		CommandID:         commandID,
		ArtifactPurpose:   purpose,
		ReasonCode:        "artifact_upload_failed",
		MoreLogsAvailable: false,
		EmittedAt:         h.now(),
	})
}

func parseArtifactCommandPayload(raw json.RawMessage) (ArtifactCommandPayload, error) {
	var payload ArtifactCommandPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ArtifactCommandPayload{}, err
	}
	payload.Purpose = strings.TrimSpace(payload.Purpose)
	payload.LocalArtifactRef = strings.TrimSpace(payload.LocalArtifactRef)
	payload.ContentType = strings.TrimSpace(payload.ContentType)
	payload.RedactionProfile = strings.TrimSpace(payload.RedactionProfile)
	if payload.Purpose == "" || payload.LocalArtifactRef == "" || payload.ContentType == "" {
		return ArtifactCommandPayload{}, fmt.Errorf("purpose, local_artifact_ref, and content_type are required")
	}
	if !allowedLocalArtifactPurpose(payload.Purpose) {
		return ArtifactCommandPayload{}, fmt.Errorf("unsupported artifact purpose")
	}
	return payload, nil
}

func validateUploadSession(session ArtifactUploadSession, artifact GeneratedArtifact, now time.Time) error {
	if strings.TrimSpace(session.UploadID) == "" {
		return fmt.Errorf("upload_id is required")
	}
	if session.Method != "PUT" {
		return fmt.Errorf("upload method must be PUT")
	}
	parsed, err := url.Parse(session.URL)
	if err != nil {
		return fmt.Errorf("parse upload URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("upload URL must be https")
	}
	if !session.ExpiresAt.After(now) {
		return fmt.Errorf("upload session expired")
	}
	if session.ContentType != artifact.ContentType {
		return fmt.Errorf("upload content type mismatch")
	}
	if session.MaxBytes > 0 && int64(len(artifact.Bytes)) > session.MaxBytes {
		return fmt.Errorf("artifact exceeds upload session limit")
	}
	return nil
}

func allowedLocalArtifactPurpose(purpose string) bool {
	switch purpose {
	case "diagnostic_bundle", "debug_bundle", "error_log_bundle", "backup_artifact":
		return true
	default:
		return false
	}
}

func (h ArtifactUploadHandler) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}
