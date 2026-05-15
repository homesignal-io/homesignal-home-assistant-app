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
	}{
		{name: "health", path: "/healthz", wantStatus: http.StatusOK, wantField: "status", wantValue: "ok"},
		{name: "ready", path: "/readyz", wantStatus: http.StatusOK, wantField: "status", wantValue: "ready"},
		{name: "version", path: "/version", wantStatus: http.StatusOK, wantField: "version", wantValue: "test-version"},
		{name: "not found", path: "/missing", wantStatus: http.StatusNotFound, wantField: "error", wantValue: "not_found"},
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

			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			if got := body[tt.wantField]; got != tt.wantValue {
				t.Fatalf("%s = %v, want %q", tt.wantField, got, tt.wantValue)
			}
		})
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
}
