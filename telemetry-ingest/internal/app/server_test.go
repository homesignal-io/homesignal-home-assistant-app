package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/homesignal-io/homesignal-home-assistant-app/telemetry-ingest/internal/pipeline"
)

func TestHealthAndReadyEndpoints(t *testing.T) {
	handler := NewHandler(Server{})

	for _, path := range []string{"/healthz", "/readyz", "/version"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", recorder.Code)
			}
		})
	}
}

func TestTelemetryRouteRejectsWrongMethod(t *testing.T) {
	handler := NewHandler(Server{Pipeline: pipeline.NewRuntimePipeline(&pipeline.MemoryWriter{}, &pipeline.MemoryFailureSink{})})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/agent/telemetry", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "method_not_allowed") {
		t.Fatalf("expected method error, got %s", recorder.Body.String())
	}
}

func TestLifecycleRouteAcceptsIoTShapedEvent(t *testing.T) {
	handler := NewHandler(Server{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/iot/lifecycle", strings.NewReader(`{
		"lifecycle_event": "connected",
		"client_id": "dev_01J00000000000000000000000",
		"principal_id": "principal-fixture",
		"sessionIdentifier": "session-fixture",
		"timestamp": "2026-05-14T12:00:00Z"
	}`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"connection_state":"online"`) {
		t.Fatalf("expected online lifecycle response, got %s", recorder.Body.String())
	}
}
