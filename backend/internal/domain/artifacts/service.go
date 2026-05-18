package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	PurposeErrorLogBundle   Purpose = "error_log_bundle"
	PurposeDiagnosticBundle Purpose = "diagnostic_bundle"
	PurposeDebugBundle      Purpose = "debug_bundle"
	PurposeBackupArtifact   Purpose = "backup_artifact"

	StatusPendingUpload Status = "pending_upload"
	StatusUploaded      Status = "uploaded"
	StatusValidated     Status = "validated"
	StatusRejected      Status = "rejected"
	StatusExpired       Status = "expired"
	StatusCanceled      Status = "canceled"

	DefaultUploadMethod = "PUT"
)

type Purpose string
type Status string

type PurposeDefinition struct {
	MaxSizeBytes       int64
	DefaultURLTTL      time.Duration
	AllowedContentType map[string]bool
}

type UploadSlot struct {
	UploadID               string
	AccountID              string
	SiteID                 string
	DeviceID               string
	CommandID              string
	Purpose                Purpose
	Status                 Status
	RequestedBySubjectType string
	RequestedBySubjectID   string
	ObjectBucket           string
	ObjectKey              string
	ContentType            string
	MaxSizeBytes           int64
	ExpectedSizeBytes      *int64
	ChecksumSHA256         string
	LocalArtifactRef       string
	RedactionProfile       string
	Metadata               json.RawMessage
	ExpiresAt              time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type CreateUploadRequest struct {
	AccountID              string
	SiteID                 string
	DeviceID               string
	CommandID              string
	Purpose                Purpose
	RequestedBySubjectType string
	RequestedBySubjectID   string
	ContentType            string
	ExpectedSizeBytes      *int64
	ChecksumSHA256         string
	LocalArtifactRef       string
	RedactionProfile       string
	Metadata               json.RawMessage
}

type UploadURLRequest struct {
	UploadID       string
	ObjectBucket   string
	ObjectKey      string
	Method         string
	ContentType    string
	MaxSizeBytes   int64
	ExpiresAt      time.Time
	ChecksumSHA256 string
}

type UploadCapability struct {
	UploadID    string
	Method      string
	URL         string
	ExpiresAt   time.Time
	ContentType string
	MaxBytes    int64
}

type CreateUploadResponse struct {
	Upload     UploadSlot
	Capability UploadCapability
}

type Repository interface {
	CreateUploadSlot(ctx context.Context, slot UploadSlot) error
}

type SignedURLIssuer interface {
	IssueUploadURL(ctx context.Context, req UploadURLRequest) (UploadCapability, error)
}

type IDGenerator func() string
type Clock func() time.Time

type Service struct {
	Repository  Repository
	Issuer      SignedURLIssuer
	Purposes    map[Purpose]PurposeDefinition
	Bucket      string
	IDGenerator IDGenerator
	Clock       Clock
}

func DefaultPurposeRegistry() map[Purpose]PurposeDefinition {
	return map[Purpose]PurposeDefinition{
		PurposeErrorLogBundle: {
			MaxSizeBytes:  5 * 1024 * 1024,
			DefaultURLTTL: 15 * time.Minute,
			AllowedContentType: contentTypes(
				"application/json",
				"application/x-ndjson",
				"text/plain",
				"application/gzip",
				"application/zstd",
			),
		},
		PurposeDiagnosticBundle: {
			MaxSizeBytes:  25 * 1024 * 1024,
			DefaultURLTTL: 15 * time.Minute,
			AllowedContentType: contentTypes(
				"application/json",
				"application/x-ndjson",
				"text/plain",
				"application/gzip",
				"application/zstd",
				"application/zip",
			),
		},
		PurposeDebugBundle: {
			MaxSizeBytes:  25 * 1024 * 1024,
			DefaultURLTTL: 15 * time.Minute,
			AllowedContentType: contentTypes(
				"application/json",
				"application/x-ndjson",
				"text/plain",
				"application/gzip",
				"application/zstd",
				"application/zip",
			),
		},
		PurposeBackupArtifact: {
			MaxSizeBytes:  250 * 1024 * 1024,
			DefaultURLTTL: 15 * time.Minute,
			AllowedContentType: contentTypes(
				"application/gzip",
				"application/octet-stream",
				"application/x-tar",
				"application/zip",
			),
		},
	}
}

func (s Service) CreateUpload(ctx context.Context, req CreateUploadRequest) (CreateUploadResponse, error) {
	if s.Repository == nil {
		return CreateUploadResponse{}, fmt.Errorf("artifact repository is required")
	}
	if s.Issuer == nil {
		return CreateUploadResponse{}, fmt.Errorf("signed URL issuer is required")
	}
	if strings.TrimSpace(s.Bucket) == "" {
		return CreateUploadResponse{}, fmt.Errorf("object bucket is required")
	}
	definition, err := s.definitionFor(req.Purpose)
	if err != nil {
		return CreateUploadResponse{}, err
	}
	now := s.now()
	uploadID, err := s.newUploadID()
	if err != nil {
		return CreateUploadResponse{}, err
	}
	contentType := strings.TrimSpace(req.ContentType)
	if !definition.AllowedContentType[contentType] {
		return CreateUploadResponse{}, fmt.Errorf("unsupported content type %q for purpose %q", contentType, req.Purpose)
	}
	if req.ExpectedSizeBytes != nil {
		if *req.ExpectedSizeBytes < 0 {
			return CreateUploadResponse{}, fmt.Errorf("expected size cannot be negative")
		}
		if *req.ExpectedSizeBytes > definition.MaxSizeBytes {
			return CreateUploadResponse{}, fmt.Errorf("expected size exceeds purpose max")
		}
	}
	metadata := normalizeJSON(req.Metadata)
	if !json.Valid(metadata) {
		return CreateUploadResponse{}, fmt.Errorf("metadata must be valid JSON")
	}
	slot := UploadSlot{
		UploadID:               uploadID,
		AccountID:              cleanSegment("account_id", req.AccountID),
		SiteID:                 cleanSegment("site_id", req.SiteID),
		DeviceID:               cleanSegment("device_id", req.DeviceID),
		CommandID:              strings.TrimSpace(req.CommandID),
		Purpose:                req.Purpose,
		Status:                 StatusPendingUpload,
		RequestedBySubjectType: strings.TrimSpace(req.RequestedBySubjectType),
		RequestedBySubjectID:   strings.TrimSpace(req.RequestedBySubjectID),
		ObjectBucket:           strings.TrimSpace(s.Bucket),
		ContentType:            contentType,
		MaxSizeBytes:           definition.MaxSizeBytes,
		ExpectedSizeBytes:      req.ExpectedSizeBytes,
		ChecksumSHA256:         strings.TrimSpace(req.ChecksumSHA256),
		LocalArtifactRef:       strings.TrimSpace(req.LocalArtifactRef),
		RedactionProfile:       strings.TrimSpace(req.RedactionProfile),
		Metadata:               metadata,
		ExpiresAt:              now.Add(definition.DefaultURLTTL),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := validateUploadSlot(slot); err != nil {
		return CreateUploadResponse{}, err
	}
	slot.ObjectKey = buildObjectKey(slot)
	if err := s.Repository.CreateUploadSlot(ctx, slot); err != nil {
		return CreateUploadResponse{}, fmt.Errorf("create artifact upload slot: %w", err)
	}
	capability, err := s.Issuer.IssueUploadURL(ctx, UploadURLRequest{
		UploadID:       slot.UploadID,
		ObjectBucket:   slot.ObjectBucket,
		ObjectKey:      slot.ObjectKey,
		Method:         DefaultUploadMethod,
		ContentType:    slot.ContentType,
		MaxSizeBytes:   slot.MaxSizeBytes,
		ExpiresAt:      slot.ExpiresAt,
		ChecksumSHA256: slot.ChecksumSHA256,
	})
	if err != nil {
		return CreateUploadResponse{}, fmt.Errorf("issue signed upload URL: %w", err)
	}
	return CreateUploadResponse{Upload: slot, Capability: capability}, nil
}

func (c UploadCapability) String() string {
	return fmt.Sprintf("upload_capability{upload_id:%s method:%s url:<redacted> expires_at:%s}", c.UploadID, c.Method, c.ExpiresAt.Format(time.RFC3339))
}

func (r CreateUploadResponse) String() string {
	return fmt.Sprintf("artifact_upload{upload_id:%s object_key:%s signed_url:<redacted>}", r.Upload.UploadID, r.Upload.ObjectKey)
}

func (s Service) definitionFor(purpose Purpose) (PurposeDefinition, error) {
	purposes := s.Purposes
	if purposes == nil {
		purposes = DefaultPurposeRegistry()
	}
	definition, ok := purposes[purpose]
	if !ok {
		return PurposeDefinition{}, fmt.Errorf("unsupported artifact purpose %q", purpose)
	}
	if definition.MaxSizeBytes <= 0 {
		return PurposeDefinition{}, fmt.Errorf("max size is required for purpose %q", purpose)
	}
	if definition.DefaultURLTTL <= 0 {
		return PurposeDefinition{}, fmt.Errorf("default URL TTL is required for purpose %q", purpose)
	}
	if len(definition.AllowedContentType) == 0 {
		return PurposeDefinition{}, fmt.Errorf("content type registry is required for purpose %q", purpose)
	}
	return definition, nil
}

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func (s Service) newUploadID() (string, error) {
	if s.IDGenerator == nil {
		return "", fmt.Errorf("upload id generator is required")
	}
	uploadID := strings.TrimSpace(s.IDGenerator())
	if uploadID == "" {
		return "", fmt.Errorf("upload id is required")
	}
	if strings.ContainsAny(uploadID, `/\`) {
		return "", fmt.Errorf("upload id contains path separators")
	}
	return uploadID, nil
}

func validateUploadSlot(slot UploadSlot) error {
	if slot.AccountID == "" {
		return fmt.Errorf("account_id is required")
	}
	if slot.SiteID == "" {
		return fmt.Errorf("site_id is required")
	}
	if slot.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if slot.ContentType == "" {
		return fmt.Errorf("content_type is required")
	}
	if slot.ExpiresAt.IsZero() || !slot.ExpiresAt.After(slot.CreatedAt) {
		return fmt.Errorf("expires_at must be after created_at")
	}
	return nil
}

func buildObjectKey(slot UploadSlot) string {
	created := slot.CreatedAt.UTC()
	return fmt.Sprintf(
		"artifacts/accounts/%s/sites/%s/devices/%s/%s/%04d/%02d/%02d/%s",
		slot.AccountID,
		slot.SiteID,
		slot.DeviceID,
		slot.Purpose,
		created.Year(),
		created.Month(),
		created.Day(),
		slot.UploadID,
	)
}

func cleanSegment(name string, value string) string {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, `/\`) {
		return ""
	}
	return value
}

func normalizeJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	clone := make(json.RawMessage, len(value))
	copy(clone, value)
	return clone
}

func contentTypes(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}
