package objectstore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/artifacts"
)

func TestS3UploadURLIssuerPresignsPutObjectWithScopedKeyAndTTL(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	presigner := &fakePutObjectPresigner{
		response: PresignedRequest{
			URL:    "https://homesignal-staging-artifacts.s3.amazonaws.com/artifacts/key?X-Amz-Signature=fixture",
			Method: "PUT",
		},
	}
	issuer := S3UploadURLIssuer{
		Presigner: presigner,
		Clock:     func() time.Time { return now },
	}

	capability, err := issuer.IssueUploadURL(context.Background(), artifacts.UploadURLRequest{
		UploadID:       "art_123",
		ObjectBucket:   "homesignal-staging-artifacts",
		ObjectKey:      "artifacts/accounts/acct_123/sites/site_123/devices/dev_123/backup_artifact/2026/05/18/art_123",
		Method:         "PUT",
		ContentType:    "application/gzip",
		MaxSizeBytes:   1024,
		ExpiresAt:      now.Add(15 * time.Minute),
		ChecksumSHA256: "abc123",
	})
	if err != nil {
		t.Fatalf("issue upload URL: %v", err)
	}
	if capability.Method != "PUT" {
		t.Fatalf("expected PUT capability, got %s", capability.Method)
	}
	if len(presigner.requests) != 1 {
		t.Fatalf("expected one presign request, got %d", len(presigner.requests))
	}
	req := presigner.requests[0]
	if req.Bucket != "homesignal-staging-artifacts" {
		t.Fatalf("unexpected bucket %q", req.Bucket)
	}
	if req.Key != "artifacts/accounts/acct_123/sites/site_123/devices/dev_123/backup_artifact/2026/05/18/art_123" {
		t.Fatalf("unexpected key %q", req.Key)
	}
	if req.Expires != 15*time.Minute {
		t.Fatalf("unexpected TTL %s", req.Expires)
	}
	if req.ChecksumSHA256 != "abc123" {
		t.Fatalf("checksum was not passed to signer")
	}
}

func TestS3UploadURLIssuerRejectsUnsafeURLsAndExcessiveTTL(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		response  PresignedRequest
		expiresAt time.Time
		want      string
	}{
		{
			name:      "http",
			response:  PresignedRequest{URL: "http://bucket.example/upload", Method: "PUT"},
			expiresAt: now.Add(15 * time.Minute),
			want:      "https",
		},
		{
			name:      "ttl",
			response:  PresignedRequest{URL: "https://bucket.example/upload", Method: "PUT"},
			expiresAt: now.Add(time.Hour),
			want:      "TTL exceeds",
		},
		{
			name:      "method",
			response:  PresignedRequest{URL: "https://bucket.example/upload", Method: "POST"},
			expiresAt: now.Add(15 * time.Minute),
			want:      "method mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuer := S3UploadURLIssuer{
				Presigner: &fakePutObjectPresigner{response: tt.response},
				Clock:     func() time.Time { return now },
			}
			_, err := issuer.IssueUploadURL(context.Background(), artifacts.UploadURLRequest{
				UploadID:     "art_123",
				ObjectBucket: "bucket",
				ObjectKey:    "key",
				Method:       "PUT",
				ContentType:  "application/json",
				MaxSizeBytes: 1024,
				ExpiresAt:    tt.expiresAt,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestS3UploadURLIssuerRejectsNonPutRequest(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	issuer := S3UploadURLIssuer{
		Presigner: &fakePutObjectPresigner{},
		Clock:     func() time.Time { return now },
	}

	_, err := issuer.IssueUploadURL(context.Background(), artifacts.UploadURLRequest{
		UploadID:     "art_123",
		ObjectBucket: "bucket",
		ObjectKey:    "key",
		Method:       "GET",
		ContentType:  "application/json",
		MaxSizeBytes: 1024,
		ExpiresAt:    now.Add(15 * time.Minute),
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported object-store upload method") {
		t.Fatalf("expected method rejection, got %v", err)
	}
}

type fakePutObjectPresigner struct {
	response PresignedRequest
	requests []PutObjectPresignRequest
}

func (p *fakePutObjectPresigner) PresignPutObject(_ context.Context, req PutObjectPresignRequest) (PresignedRequest, error) {
	p.requests = append(p.requests, req)
	if p.response.URL == "" {
		return PresignedRequest{URL: "https://bucket.example/upload", Method: "PUT"}, nil
	}
	return p.response, nil
}
