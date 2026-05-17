package lambdaproxy

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/homesignal-io/homesignal-home-assistant/backend/internal/controlplane"
	"github.com/homesignal-io/homesignal-home-assistant/backend/internal/platform/config"
)

func TestHandleInvocationAdaptsAPIGatewayV2Event(t *testing.T) {
	app := controlplane.New(config.Config{
		Environment: "test",
		ServiceName: "control-plane",
		Version:     "test-version",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	payload := []byte(`{
		"version": "2.0",
		"rawPath": "/healthz",
		"headers": {"x-request-id": "request-123"},
		"requestContext": {
			"http": {
				"method": "GET",
				"path": "/healthz"
			}
		}
	}`)

	response, summary, err := handleInvocation(app, payload)
	if err != nil {
		t.Fatalf("handleInvocation returned error: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", response.StatusCode)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(response.Body), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %v, want ok", body["status"])
	}
	if summary.Method != "GET" {
		t.Fatalf("Method = %q, want GET", summary.Method)
	}
	if summary.Path != "/healthz" {
		t.Fatalf("Path = %q, want /healthz", summary.Path)
	}
}
