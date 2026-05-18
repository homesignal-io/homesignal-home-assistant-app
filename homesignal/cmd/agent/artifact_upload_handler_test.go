package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestArtifactUploadHandlerUploadsAndReportsSuccess(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	client := &fakeArtifactClient{
		session: ArtifactUploadSession{
			UploadID:    "art_123",
			Method:      "PUT",
			URL:         "https://bucket.example/artifacts/art_123?sig=fixture",
			ContentType: "application/gzip",
			MaxBytes:    1024,
			ExpiresAt:   now.Add(15 * time.Minute),
		},
	}
	generator := &fakeArtifactGenerator{
		artifacts: map[string]GeneratedArtifact{
			"logref_123": {
				Purpose:          "error_log_bundle",
				LocalArtifactRef: "logref_123",
				ContentType:      "application/gzip",
				Bytes:            []byte("redacted logs"),
			},
		},
	}
	uploader := &fakeUploader{}
	handler := ArtifactUploadHandler{
		Client:    client,
		Generator: generator,
		Uploader:  uploader,
		Now:       func() time.Time { return now },
	}

	err := handler.HandleArtifactCommand(context.Background(), AgentCommandDetail{
		CommandID:   "cmd_123",
		CommandType: commandTypeUploadArtifact,
		DeviceID:    "dev_123",
		Payload:     artifactPayload(t, "error_log_bundle", "logref_123", "application/gzip", 1024),
	})
	if err != nil {
		t.Fatalf("handle artifact command: %v", err)
	}
	if len(client.uploadRequests) != 1 {
		t.Fatalf("expected upload request, got %#v", client.uploadRequests)
	}
	if len(uploader.uploads) != 1 {
		t.Fatalf("expected signed URL upload, got %#v", uploader.uploads)
	}
	if len(client.completions) != 1 {
		t.Fatalf("expected upload completion, got %#v", client.completions)
	}
	if len(client.results) != 1 || client.results[0].Status != "succeeded" {
		t.Fatalf("expected succeeded command result, got %#v", client.results)
	}
}

func TestArtifactUploadHandlerRejectsUnknownLocalArtifactRef(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	client := &fakeArtifactClient{}
	handler := ArtifactUploadHandler{
		Client:    client,
		Generator: &fakeArtifactGenerator{err: errors.New("not found")},
		Uploader:  &fakeUploader{},
		Now:       func() time.Time { return now },
	}

	err := handler.HandleArtifactCommand(context.Background(), AgentCommandDetail{
		CommandID:   "cmd_123",
		CommandType: commandTypeUploadArtifact,
		DeviceID:    "dev_123",
		Payload:     artifactPayload(t, "error_log_bundle", "missing", "application/gzip", 1024),
	})
	if err != nil {
		t.Fatalf("unknown local ref should report failed result without surfacing an error, got %v", err)
	}
	if len(client.uploadRequests) != 0 {
		t.Fatalf("unknown local ref must not request upload session")
	}
	if len(client.results) != 1 || client.results[0].ReasonCode != "unknown_local_artifact_ref" {
		t.Fatalf("expected unknown ref failed result, got %#v", client.results)
	}
}

func TestArtifactUploadHandlerUploadFailureEmitsOneBoundedAlarm(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	client := &fakeArtifactClient{
		session: ArtifactUploadSession{
			UploadID:    "art_123",
			Method:      "PUT",
			URL:         "https://bucket.example/artifacts/art_123?sig=fixture",
			ContentType: "application/gzip",
			MaxBytes:    1024,
			ExpiresAt:   now.Add(15 * time.Minute),
		},
	}
	alarmSink := &fakeAlarmSink{}
	handler := ArtifactUploadHandler{
		Client: client,
		Generator: &fakeArtifactGenerator{artifacts: map[string]GeneratedArtifact{
			"logref_123": {
				Purpose:          "error_log_bundle",
				LocalArtifactRef: "logref_123",
				ContentType:      "application/gzip",
				Bytes:            []byte("redacted logs"),
			},
		}},
		Uploader:  &fakeUploader{err: errors.New("put failed")},
		AlarmSink: alarmSink,
		Now:       func() time.Time { return now },
	}

	err := handler.HandleArtifactCommand(context.Background(), AgentCommandDetail{
		CommandID:   "cmd_123",
		CommandType: commandTypeUploadArtifact,
		DeviceID:    "dev_123",
		Payload:     artifactPayload(t, "error_log_bundle", "logref_123", "application/gzip", 1024),
	})
	if err != nil {
		t.Fatalf("upload failure alarm/result should not surface an error when reporting succeeds, got %v", err)
	}
	if len(alarmSink.alarms) != 1 {
		t.Fatalf("expected one bounded alarm, got %#v", alarmSink.alarms)
	}
	if alarmSink.alarms[0].MoreLogsAvailable {
		t.Fatalf("upload failure alarm must not request more logs")
	}
	if len(client.completions) != 0 {
		t.Fatalf("failed upload must not complete session")
	}
	if len(client.results) != 1 || client.results[0].ReasonCode != "artifact_upload_failed" {
		t.Fatalf("expected failed command result, got %#v", client.results)
	}
	if !strings.Contains(string(client.results[0].Payload), `"more_logs_available":false`) {
		t.Fatalf("failed result must suppress recursive logs: %s", client.results[0].Payload)
	}
}

func TestArtifactUploadHandlerRejectsInvalidUploadSession(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	client := &fakeArtifactClient{
		session: ArtifactUploadSession{
			UploadID:    "art_123",
			Method:      "PUT",
			URL:         "http://bucket.example/artifacts/art_123",
			ContentType: "application/gzip",
			MaxBytes:    1024,
			ExpiresAt:   now.Add(15 * time.Minute),
		},
	}
	handler := ArtifactUploadHandler{
		Client: client,
		Generator: &fakeArtifactGenerator{artifacts: map[string]GeneratedArtifact{
			"logref_123": {
				Purpose:          "error_log_bundle",
				LocalArtifactRef: "logref_123",
				ContentType:      "application/gzip",
				Bytes:            []byte("redacted logs"),
			},
		}},
		Uploader: &fakeUploader{},
		Now:      func() time.Time { return now },
	}

	err := handler.HandleArtifactCommand(context.Background(), AgentCommandDetail{
		CommandID:   "cmd_123",
		CommandType: commandTypeUploadArtifact,
		DeviceID:    "dev_123",
		Payload:     artifactPayload(t, "error_log_bundle", "logref_123", "application/gzip", 1024),
	})
	if err != nil {
		t.Fatalf("invalid upload session should report failed result without surfacing an error, got %v", err)
	}
	if len(client.results) != 1 || client.results[0].ReasonCode != "invalid_upload_session" {
		t.Fatalf("expected invalid upload session result, got %#v", client.results)
	}
}

func artifactPayload(t *testing.T, purpose string, ref string, contentType string, maxBytes int64) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(ArtifactCommandPayload{
		Purpose:          purpose,
		LocalArtifactRef: ref,
		ContentType:      contentType,
		MaxBytes:         maxBytes,
		RedactionProfile: "default_v1",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return payload
}

type fakeArtifactClient struct {
	session        ArtifactUploadSession
	uploadRequests []ArtifactUploadRequest
	completions    []ArtifactUploadCompleteRequest
	results        []CommandResultRequest
}

func (c *fakeArtifactClient) RequestArtifactUpload(_ context.Context, req ArtifactUploadRequest) (ArtifactUploadSession, error) {
	c.uploadRequests = append(c.uploadRequests, req)
	return c.session, nil
}

func (c *fakeArtifactClient) CompleteArtifactUpload(_ context.Context, req ArtifactUploadCompleteRequest) error {
	c.completions = append(c.completions, req)
	return nil
}

func (c *fakeArtifactClient) RecordCommandResult(_ context.Context, req CommandResultRequest) error {
	c.results = append(c.results, req)
	return nil
}

type fakeArtifactGenerator struct {
	artifacts map[string]GeneratedArtifact
	err       error
}

func (g *fakeArtifactGenerator) GenerateArtifact(_ context.Context, req GenerateArtifactRequest) (GeneratedArtifact, error) {
	if g.err != nil {
		return GeneratedArtifact{}, g.err
	}
	artifact, ok := g.artifacts[req.LocalArtifactRef]
	if !ok {
		return GeneratedArtifact{}, errors.New("not found")
	}
	return artifact, nil
}

type fakeUploader struct {
	uploads []GeneratedArtifact
	err     error
}

func (u *fakeUploader) UploadArtifact(_ context.Context, _ ArtifactUploadSession, artifact GeneratedArtifact) error {
	if u.err != nil {
		return u.err
	}
	u.uploads = append(u.uploads, artifact)
	return nil
}

type fakeAlarmSink struct {
	alarms []AgentAlarm
}

func (s *fakeAlarmSink) EmitAgentAlarm(_ context.Context, alarm AgentAlarm) error {
	s.alarms = append(s.alarms, alarm)
	return nil
}
