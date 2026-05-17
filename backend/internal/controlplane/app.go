package controlplane

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant/backend/internal/platform/api"
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
		requestContext := api.NewRequestContext(r)

		response := a.ServeWithContext(requestContext, r.Method, r.URL.Path, firstHeaderValues(r.Header), nil)
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}
		w.Header().Set(api.RequestIDHeader, requestContext.RequestID)
		w.Header().Set(api.CorrelationIDHeader, requestContext.CorrelationID)
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(response.Body)

		a.logger.Info(
			"request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", response.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestContext.RequestID,
			"correlation_id", requestContext.CorrelationID,
			"correlation_id_source", requestContext.CorrelationIDSource,
		)
	})
}

func (a *App) Serve(method string, path string, _ map[string]string, _ []byte) Response {
	return a.ServeWithContext(api.NewSyntheticRequestContext(), method, path, nil, nil)
}

func (a *App) ServeWithContext(requestContext api.RequestContext, method string, path string, _ map[string]string, _ []byte) Response {
	if method != http.MethodGet {
		return errorResponse(requestContext, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed.")
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
		return errorResponse(requestContext, http.StatusNotFound, "NOT_FOUND", "Not found.")
	}
}

func jsonResponse(status int, body map[string]any) Response {
	statusCode, headers, payload := api.JSONResponse(status, body)
	return Response{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       payload,
	}
}

func errorResponse(requestContext api.RequestContext, status int, code string, message string) Response {
	statusCode, headers, body := api.JSONResponse(status, api.NewErrorEnvelope(requestContext, code, message, nil))
	return Response{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
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
