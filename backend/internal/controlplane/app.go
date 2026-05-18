package controlplane

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/api"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/authn"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/config"
)

type App struct {
	cfg                  config.Config
	logger               *slog.Logger
	idempotency          *idempotencyStore
	rateLimiter          *rateLimiter
	serviceAuthenticator authn.ServiceAuthenticator
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
		cfg:                  cfg,
		logger:               logger,
		idempotency:          newIdempotencyStore(10 * time.Minute),
		rateLimiter:          newRateLimiter(600, time.Minute),
		serviceAuthenticator: authn.DefaultStaticServiceAuthenticator(),
	}
}

func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestContext := api.NewRequestContext(r)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			response := errorResponse(requestContext, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Request body could not be read.")
			writeResponse(w, requestContext, response)
			return
		}

		response := a.ServeWithContext(requestContext, r.Method, r.URL.Path, firstHeaderValues(r.Header), body)
		writeResponse(w, requestContext, response)

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

func (a *App) Serve(method string, path string, headers map[string]string, body []byte) Response {
	return a.ServeWithContext(api.NewSyntheticRequestContext(), method, path, headers, body)
}

func (a *App) ServeWithContext(requestContext api.RequestContext, method string, path string, headers map[string]string, body []byte) Response {
	if route, ok := operationalRoutes(method, path); ok {
		requestContext.RouteTemplate = route
		return a.handleOperationalRoute(requestContext, path)
	}
	if operationalPathExists(path) {
		return errorResponse(requestContext, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed.")
	}

	if strings.HasPrefix(path, "/api/v1") {
		return a.handlePublicAPIRoute(requestContext, method, path, headers, body)
	}
	if strings.HasPrefix(path, "/agent") {
		return a.handleAgentRoute(requestContext, method, path, headers)
	}
	if strings.HasPrefix(path, "/internal") {
		return a.handleInternalRoute(requestContext, method, path, headers)
	}

	return errorResponse(requestContext, http.StatusNotFound, "NOT_FOUND", "Not found.")
}

func (a *App) handleOperationalRoute(_ api.RequestContext, path string) Response {
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
		return jsonResponse(http.StatusInternalServerError, map[string]any{
			"error": "unregistered operational route",
		})
	}
}

func (a *App) handlePublicAPIRoute(requestContext api.RequestContext, method string, path string, headers map[string]string, body []byte) Response {
	route, ok, pathMatched := findPublicRoute(method, path)
	if !ok {
		if pathMatched {
			return errorResponse(requestContext, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed.")
		}
		return errorResponse(requestContext, http.StatusNotFound, "NOT_FOUND", "Not found.")
	}
	requestContext.RouteTemplate = route.Pattern

	if route.Auth == publicAuthHuman && strings.TrimSpace(headerValue(headers, "Authorization")) == "" {
		return errorResponse(requestContext, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication required.")
	}

	produce := func() Response {
		if ok, retryAfter := a.allowRoute(requestContext, route.Pattern); !ok {
			return rateLimitedResponse(requestContext, retryAfter)
		}

		response := errorResponse(requestContext, http.StatusNotImplemented, "ROUTE_NOT_IMPLEMENTED", "Route is registered but not implemented yet.")
		response.Headers["X-HomeSignal-Operation-ID"] = route.OperationID
		if route.Auth == publicAuthEnrollment {
			response.Headers["Cache-Control"] = "no-store"
		}
		return response
	}

	if route.RequiresIdempotency {
		key := strings.TrimSpace(headerValue(headers, "Idempotency-Key"))
		if key == "" {
			return errorResponse(requestContext, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required.")
		}
		scope := route.Pattern + "|" + rateLimitSubject(requestContext)
		return a.idempotency.getOrStore(requestContext, scope, key, requestHash(method, path, body), produce)
	}

	return produce()
}

func (a *App) handleAgentRoute(requestContext api.RequestContext, method string, path string, headers map[string]string) Response {
	route, ok, pathMatched := findRoute(method, path, agentRouteSpecs)
	if !ok {
		if pathMatched {
			return errorResponse(requestContext, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed.")
		}
		return errorResponse(requestContext, http.StatusNotFound, "NOT_FOUND", "Not found.")
	}
	requestContext.RouteTemplate = route.Pattern

	if strings.TrimSpace(headerValue(headers, "X-HomeSignal-Device-Cert-Fingerprint")) == "" {
		return errorResponse(requestContext, http.StatusUnauthorized, "DEVICE_CERTIFICATE_REQUIRED", "Device certificate authentication is required.")
	}
	return errorResponse(requestContext, http.StatusNotImplemented, "ROUTE_NOT_IMPLEMENTED", "Route is registered but not implemented yet.")
}

func (a *App) handleInternalRoute(requestContext api.RequestContext, method string, path string, headers map[string]string) Response {
	route, ok, pathMatched := findRoute(method, path, internalRouteSpecs)
	if !ok {
		if pathMatched {
			return errorResponse(requestContext, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed.")
		}
		return errorResponse(requestContext, http.StatusNotFound, "NOT_FOUND", "Not found.")
	}
	requestContext.RouteTemplate = route.Pattern

	serviceSubject, err := a.serviceAuthenticator.AuthenticateService(context.Background(), headerValue(headers, "X-HomeSignal-Service-Principal"))
	if err != nil {
		return errorResponse(requestContext, http.StatusUnauthorized, "SERVICE_AUTHENTICATION_REQUIRED", "Service authentication is required.")
	}
	if ok, retryAfter := a.allowRoute(requestContext, route.Pattern+"|"+serviceSubject.ID); !ok {
		return rateLimitedResponse(requestContext, retryAfter)
	}
	response := errorResponse(requestContext, http.StatusNotImplemented, "ROUTE_NOT_IMPLEMENTED", "Route is registered but not implemented yet.")
	response.Headers["X-HomeSignal-Service-Subject"] = serviceSubject.ID
	return response
}

func operationalRoutes(method string, path string) (string, bool) {
	if method != http.MethodGet {
		return "", false
	}
	if operationalPathExists(path) {
		return path, true
	}
	return "", false
}

func operationalPathExists(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/version":
		return true
	default:
		return false
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

func headerValue(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func (a *App) allowRoute(requestContext api.RequestContext, routePattern string) (bool, int) {
	if a.rateLimiter == nil {
		return true, 0
	}
	return a.rateLimiter.allow(routePattern + "|" + rateLimitSubject(requestContext))
}

func rateLimitSubject(requestContext api.RequestContext) string {
	if requestContext.SourceIP != "" {
		return requestContext.SourceIP
	}
	return "synthetic"
}

func writeResponse(w http.ResponseWriter, requestContext api.RequestContext, response Response) {
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}
	w.Header().Set(api.RequestIDHeader, requestContext.RequestID)
	w.Header().Set(api.CorrelationIDHeader, requestContext.CorrelationID)
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(response.Body)
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
