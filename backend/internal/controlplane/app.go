package controlplane

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant/backend/internal/platform/config"
)

type App struct {
	cfg    config.Config
	logger *slog.Logger
}

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

var requestCounter atomic.Uint64

func New(cfg config.Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		cfg:    cfg,
		logger: logger,
	}
}

func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = newRequestID()
		}

		response := a.Serve(r.Method, r.URL.Path, firstHeaderValues(r.Header), nil)
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}
		w.Header().Set("X-Request-Id", requestID)
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(response.Body)

		a.logger.Info(
			"request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", response.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestID,
		)
	})
}

func (a *App) Serve(method string, path string, _ map[string]string, _ []byte) Response {
	if method != http.MethodGet {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]any{
			"error": "method_not_allowed",
		})
	}

	switch path {
	case "/healthz":
		return jsonResponse(http.StatusOK, map[string]any{
			"status":      "ok",
			"service":     a.cfg.ServiceName,
			"environment": a.cfg.Environment,
			"version":     a.cfg.Version,
		})
	case "/readyz":
		return jsonResponse(http.StatusOK, map[string]any{
			"status":      "ready",
			"service":     a.cfg.ServiceName,
			"environment": a.cfg.Environment,
			"version":     a.cfg.Version,
			"checks": map[string]string{
				"process": "ok",
			},
		})
	case "/version":
		return jsonResponse(http.StatusOK, map[string]any{
			"service":     a.cfg.ServiceName,
			"environment": a.cfg.Environment,
			"version":     a.cfg.Version,
		})
	default:
		return jsonResponse(http.StatusNotFound, map[string]any{
			"error": "not_found",
		})
	}
}

func jsonResponse(status int, body map[string]any) Response {
	payload, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		payload = []byte(`{"error":"internal_error"}`)
	}

	return Response{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json; charset=utf-8",
		},
		Body: payload,
	}
}

func firstHeaderValues(headers http.Header) map[string]string {
	values := make(map[string]string, len(headers))
	for key, headerValues := range headers {
		if len(headerValues) > 0 {
			values[key] = headerValues[0]
		}
	}
	return values
}

func newRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), requestCounter.Add(1))
}
