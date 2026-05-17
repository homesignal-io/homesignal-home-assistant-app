package api

import (
	"net/http/httptest"
	"testing"
)

func TestNewRequestContextPreservesValidCorrelationID(t *testing.T) {
	request := httptest.NewRequest("GET", "/healthz", nil)
	request.Header.Set(CorrelationIDHeader, "corr_external_123")

	ctx := NewRequestContext(request)
	if ctx.RequestID == "" {
		t.Fatal("expected generated request ID")
	}
	if ctx.CorrelationID != "corr_external_123" {
		t.Fatalf("correlation ID = %q", ctx.CorrelationID)
	}
	if ctx.CorrelationIDSource != CorrelationIDSourceCaller {
		t.Fatalf("source = %q", ctx.CorrelationIDSource)
	}
}

func TestNewRequestContextRejectsInvalidCorrelationID(t *testing.T) {
	request := httptest.NewRequest("GET", "/healthz", nil)
	request.Header.Set(CorrelationIDHeader, "bad value")

	ctx := NewRequestContext(request)
	if ctx.CorrelationID == "bad value" {
		t.Fatal("expected invalid correlation ID to be replaced")
	}
	if ctx.CorrelationIDSource != CorrelationIDSourceGenerated {
		t.Fatalf("source = %q", ctx.CorrelationIDSource)
	}
}

func TestValidCorrelationID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "valid", value: "corr_abc-123", want: true},
		{name: "empty", value: "", want: false},
		{name: "space", value: "corr abc", want: false},
		{name: "control", value: "corr\nabc", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidCorrelationID(tt.value); got != tt.want {
				t.Fatalf("ValidCorrelationID(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
