package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNewErrorEnvelope(t *testing.T) {
	ctx := RequestContext{RequestID: "req_123"}
	envelope := NewErrorEnvelope(ctx, "NOT_FOUND", "Not found.", map[string]any{"resource": "site"})

	if envelope.Error.Code != "NOT_FOUND" {
		t.Fatalf("code = %q", envelope.Error.Code)
	}
	if envelope.Error.RequestID != "req_123" {
		t.Fatalf("request_id = %q", envelope.Error.RequestID)
	}
	if envelope.Error.Details["resource"] != "site" {
		t.Fatalf("details = %#v", envelope.Error.Details)
	}
}

func TestJSONResponse(t *testing.T) {
	status, headers, body := JSONResponse(http.StatusTeapot, map[string]string{"status": "short"})
	if status != http.StatusTeapot {
		t.Fatalf("status = %d", status)
	}
	if headers["Content-Type"] != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", headers["Content-Type"])
	}
	var decoded map[string]string
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded["status"] != "short" {
		t.Fatalf("body = %#v", decoded)
	}
}
