package controlplane

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/config"
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

func TestPublicRouteFixturesReferenceOpenAPIOperations(t *testing.T) {
	openAPI, err := os.ReadFile("../../openapi/public-v1.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI scaffold: %v", err)
	}
	fixturePayload, err := os.ReadFile("../../testdata/api/public-route-fixtures.json")
	if err != nil {
		t.Fatalf("read public route fixtures: %v", err)
	}

	var fixtures []struct {
		Name           string `json:"name"`
		Method         string `json:"method"`
		Path           string `json:"path"`
		OperationID    string `json:"operation_id"`
		ExpectedStatus int    `json:"expected_status"`
	}
	if err := json.Unmarshal(fixturePayload, &fixtures); err != nil {
		t.Fatalf("decode public route fixtures: %v", err)
	}

	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			if !strings.Contains(string(openAPI), "operationId: "+fixture.OperationID) {
				t.Fatalf("OpenAPI scaffold does not include operationId %q", fixture.OperationID)
			}

			response := app.Serve(fixture.Method, fixture.Path, nil, nil)
			if response.StatusCode != fixture.ExpectedStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, fixture.ExpectedStatus)
			}
			if response.StatusCode == http.StatusNotImplemented && response.Headers["X-HomeSignal-Operation-ID"] != fixture.OperationID {
				t.Fatalf("operation header = %q, want %q", response.Headers["X-HomeSignal-Operation-ID"], fixture.OperationID)
			}
		})
	}
}

func TestRegisteredPublicRoutesMustHaveOpenAPIOperations(t *testing.T) {
	openAPI, err := os.ReadFile("../../openapi/public-v1.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI scaffold: %v", err)
	}
	openAPIText := string(openAPI)

	for _, route := range publicRouteSpecs {
		if route.OperationID == "" {
			t.Fatalf("%s %s has empty operation id", route.Method, route.Pattern)
		}
		if !strings.Contains(openAPIText, "operationId: "+route.OperationID) {
			t.Fatalf("%s %s operationId %q missing from OpenAPI scaffold", route.Method, route.Pattern, route.OperationID)
		}
	}
}

func TestRouteShellAuthBoundaries(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	tests := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "public human route requires auth",
			method:     http.MethodGet,
			path:       "/api/v1/dashboard",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "AUTHENTICATION_REQUIRED",
		},
		{
			name:       "public human route is only shell after auth boundary",
			method:     http.MethodGet,
			path:       "/api/v1/dashboard",
			headers:    map[string]string{"Authorization": "Bearer test-token"},
			wantStatus: http.StatusNotImplemented,
			wantCode:   "ROUTE_NOT_IMPLEMENTED",
		},
		{
			name:       "agent route requires device certificate metadata",
			method:     http.MethodGet,
			path:       "/agent/commands/cmd_123",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "DEVICE_CERTIFICATE_REQUIRED",
		},
		{
			name:       "agent route is only shell after certificate boundary",
			method:     http.MethodGet,
			path:       "/agent/commands/cmd_123",
			headers:    map[string]string{"X-HomeSignal-Device-Cert-Fingerprint": "sha256:test"},
			wantStatus: http.StatusNotImplemented,
			wantCode:   "ROUTE_NOT_IMPLEMENTED",
		},
		{
			name:       "internal route requires service principal",
			method:     http.MethodPost,
			path:       "/internal/alert-candidates",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "SERVICE_AUTHENTICATION_REQUIRED",
		},
		{
			name:       "internal route is only shell after service boundary",
			method:     http.MethodPost,
			path:       "/internal/alert-candidates",
			headers:    map[string]string{"X-HomeSignal-Service-Principal": "service:telemetry-ingest"},
			wantStatus: http.StatusNotImplemented,
			wantCode:   "ROUTE_NOT_IMPLEMENTED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := app.Serve(tt.method, tt.path, tt.headers, nil)
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body, &body); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			assertErrorCode(t, body, tt.wantCode)
		})
	}
}

func TestEnrollmentShellDisablesCaching(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	response := app.Serve(http.MethodPost, "/api/v1/device-enrollment/claim-invites/verify", nil, nil)
	if response.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotImplemented)
	}
	if response.Headers["Cache-Control"] != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Headers["Cache-Control"])
	}
}

func TestIdempotencyKeyReplaysSameRequest(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	headers := map[string]string{
		"Authorization":   "Bearer test-token",
		"Idempotency-Key": "idem_123",
	}

	first := app.Serve(http.MethodPost, "/api/v1/sites/site_123/device-claim-invites", headers, []byte(`{"recipient_email":"person@example.com"}`))
	second := app.Serve(http.MethodPost, "/api/v1/sites/site_123/device-claim-invites", headers, []byte(`{"recipient_email":"person@example.com"}`))

	if first.StatusCode != http.StatusNotImplemented {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusNotImplemented)
	}
	if second.StatusCode != http.StatusNotImplemented {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusNotImplemented)
	}
	if second.Headers["X-HomeSignal-Idempotency-Replayed"] != "true" {
		t.Fatalf("replay header = %q, want true", second.Headers["X-HomeSignal-Idempotency-Replayed"])
	}
	if string(second.Body) != string(first.Body) {
		t.Fatalf("replayed body differs\nfirst: %s\nsecond: %s", first.Body, second.Body)
	}
}

func TestIdempotencyKeyRejectsDifferentRequest(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	headers := map[string]string{
		"Authorization":   "Bearer test-token",
		"Idempotency-Key": "idem_123",
	}

	_ = app.Serve(http.MethodPost, "/api/v1/sites/site_123/device-claim-invites", headers, []byte(`{"recipient_email":"person@example.com"}`))
	response := app.Serve(http.MethodPost, "/api/v1/sites/site_123/device-claim-invites", headers, []byte(`{"recipient_email":"other@example.com"}`))

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	assertErrorCode(t, body, "IDEMPOTENCY_KEY_REUSED")
}

func TestIdempotencyKeyRequiredForApprovedMutation(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	response := app.Serve(http.MethodPost, "/api/v1/sites/site_123/device-claim-invites", map[string]string{"Authorization": "Bearer test-token"}, nil)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	assertErrorCode(t, body, "IDEMPOTENCY_KEY_REQUIRED")
}

func TestRateLimitReturnsRetryAfter(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app.rateLimiter = newRateLimiter(1, time.Minute)

	headers := map[string]string{"Authorization": "Bearer test-token"}
	_ = app.Serve(http.MethodGet, "/api/v1/dashboard", headers, nil)
	response := app.Serve(http.MethodGet, "/api/v1/dashboard", headers, nil)

	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusTooManyRequests)
	}
	if response.Headers["Retry-After"] == "" {
		t.Fatalf("missing Retry-After header")
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	assertErrorCode(t, body, "RATE_LIMITED")
}

func TestInternalRouteRejectsUnknownServicePrincipal(t *testing.T) {
	app := New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	response := app.Serve(http.MethodPost, "/internal/alert-candidates", map[string]string{"X-HomeSignal-Service-Principal": "service:unknown"}, nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	assertErrorCode(t, body, "SERVICE_AUTHENTICATION_REQUIRED")
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
