package objectstore

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/artifacts"
)

const DefaultMaxPresignTTL = 30 * time.Minute

type PutObjectPresigner interface {
	PresignPutObject(ctx context.Context, req PutObjectPresignRequest) (PresignedRequest, error)
}

type PutObjectPresignRequest struct {
	Bucket         string
	Key            string
	ContentType    string
	ChecksumSHA256 string
	Expires        time.Duration
}

type PresignedRequest struct {
	URL    string
	Method string
}

type S3UploadURLIssuer struct {
	Presigner PutObjectPresigner
	Clock     func() time.Time
	MaxTTL    time.Duration
}

func (i S3UploadURLIssuer) IssueUploadURL(ctx context.Context, req artifacts.UploadURLRequest) (artifacts.UploadCapability, error) {
	if i.Presigner == nil {
		return artifacts.UploadCapability{}, fmt.Errorf("s3 presigner is required")
	}
	if strings.TrimSpace(req.Method) != artifacts.DefaultUploadMethod {
		return artifacts.UploadCapability{}, fmt.Errorf("unsupported object-store upload method %q", req.Method)
	}
	now := i.now()
	if req.ExpiresAt.IsZero() || !req.ExpiresAt.After(now) {
		return artifacts.UploadCapability{}, fmt.Errorf("upload URL expiry must be in the future")
	}
	ttl := req.ExpiresAt.Sub(now)
	maxTTL := i.maxTTL()
	if ttl > maxTTL {
		return artifacts.UploadCapability{}, fmt.Errorf("upload URL TTL exceeds %s", maxTTL)
	}
	presigned, err := i.Presigner.PresignPutObject(ctx, PutObjectPresignRequest{
		Bucket:         req.ObjectBucket,
		Key:            req.ObjectKey,
		ContentType:    req.ContentType,
		ChecksumSHA256: req.ChecksumSHA256,
		Expires:        ttl,
	})
	if err != nil {
		return artifacts.UploadCapability{}, fmt.Errorf("presign put object: %w", err)
	}
	if strings.TrimSpace(presigned.Method) != artifacts.DefaultUploadMethod {
		return artifacts.UploadCapability{}, fmt.Errorf("presigned method mismatch")
	}
	if err := validatePresignedURL(presigned.URL); err != nil {
		return artifacts.UploadCapability{}, err
	}
	return artifacts.UploadCapability{
		UploadID:    req.UploadID,
		Method:      artifacts.DefaultUploadMethod,
		URL:         presigned.URL,
		ExpiresAt:   req.ExpiresAt.UTC(),
		ContentType: req.ContentType,
		MaxBytes:    req.MaxSizeBytes,
	}, nil
}

func (i S3UploadURLIssuer) now() time.Time {
	if i.Clock != nil {
		return i.Clock().UTC()
	}
	return time.Now().UTC()
}

func (i S3UploadURLIssuer) maxTTL() time.Duration {
	if i.MaxTTL > 0 {
		return i.MaxTTL
	}
	return DefaultMaxPresignTTL
}

func validatePresignedURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse presigned URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("presigned URL must use https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("presigned URL host is required")
	}
	return nil
}
