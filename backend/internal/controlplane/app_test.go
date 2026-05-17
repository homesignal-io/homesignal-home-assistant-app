package controlplane

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/homesignal-io/homesignal-home-assistant/backend/internal/platform/config"
)

func TestOperationalRoutes(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantField  string
		wantValue  string
		wantError  string
	}{
		{name: "health", path: "/healthz", wantStatus: http.StatusOK, wantField: "status", wantValue: "ok"},
		{name: "ready", path: "/readyz", wantStatus: http.StatusOK, wantField: "status", wantValue: "ready"},
		{name: "version", path: "/version", wantStatus: http.StatusOK, wantField: "version", wantValue: "test-version"},
		{name: "not found", path: "/missing", wantStatus: http.StatusNotFound, wantError: "NOT_FOUND"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			app.Handler().ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if recorder.Header().Get("X-Request-Id") == "" {
				t.Fatal("missing X-Request-Id header")
			}
			if recorder.Header().Get("X-Correlation-Id") == "" {
				t.Fatal("missing X-Correlation-Id header")
			}

			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			if tt.wantError != "" {
				assertErrorCode(t, body, tt.wantError)
				return
			}
			if got := body[tt.wantField]; got != tt.wantValue {
				t.Fatalf("%s = %v, want %q", tt.wantField, got, tt.wantValue)
			}
		})
	}
}

func TestCorrelationIDHeader(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Correlation-ID", "corr_external_123")
	recorder := httptest.NewRecorder()

	app.Handler().ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Correlation-ID"); got != "corr_external_123" {
		t.Fatalf("correlation header = %q", got)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	response := app.Serve(http.MethodPost, "/healthz", nil, nil)
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusMethodNotAllowed)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	assertErrorCode(t, body, "METHOD_NOT_ALLOWED")
}

func assertErrorCode(t *testing.T, body map[string]any, want string) {
	t.Helper()
	errorBody, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope: %#v", body)
	}
	if got := errorBody["code"]; got != want {
		t.Fatalf("error code = %v, want %q", got, want)
	}
	if errorBody["request_id"] == "" {
		t.Fatalf("missing request_id in error envelope: %#v", errorBody)
	}
}
