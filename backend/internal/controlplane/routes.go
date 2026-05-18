package controlplane

import (
	"net/http"
	"strings"
)

type publicAuthBoundary string

const (
	publicAuthHuman      publicAuthBoundary = "human"
	publicAuthEnrollment publicAuthBoundary = "enrollment_bootstrap"
)

type publicRouteSpec struct {
	Method      string
	Pattern     string
	OperationID string
	Auth        publicAuthBoundary
}

type routeSpec struct {
	Method  string
	Pattern string
}

var publicRouteSpecs = []publicRouteSpec{
	{Method: http.MethodGet, Pattern: "/api/v1/dashboard", OperationID: "getDashboard", Auth: publicAuthHuman},
	{Method: http.MethodGet, Pattern: "/api/v1/devices", OperationID: "listDevices", Auth: publicAuthHuman},
	{Method: http.MethodGet, Pattern: "/api/v1/activity", OperationID: "listActivity", Auth: publicAuthHuman},
	{Method: http.MethodGet, Pattern: "/api/v1/alerts", OperationID: "listAlerts", Auth: publicAuthHuman},
	{Method: http.MethodGet, Pattern: "/api/v1/alert-recipients", OperationID: "listAlertRecipients", Auth: publicAuthHuman},
	{Method: http.MethodPost, Pattern: "/api/v1/sites/{site_id}/device-claim-invites", OperationID: "createDeviceClaimInvite", Auth: publicAuthHuman},
	{Method: http.MethodPost, Pattern: "/api/v1/device-enrollment/claim-invites/verify", OperationID: "verifyClaimInvite", Auth: publicAuthEnrollment},
	{Method: http.MethodPost, Pattern: "/api/v1/device-enrollment/claim-verifications/{claim_verification_id}/confirm", OperationID: "confirmClaimVerification", Auth: publicAuthEnrollment},
}

var agentRouteSpecs = []routeSpec{
	{Method: http.MethodGet, Pattern: "/agent/commands/{command_id}"},
	{Method: http.MethodPost, Pattern: "/agent/commands/{command_id}/ack"},
	{Method: http.MethodPost, Pattern: "/agent/commands/{command_id}/artifact-upload"},
	{Method: http.MethodPost, Pattern: "/agent/artifact-uploads/{upload_id}/complete"},
	{Method: http.MethodPost, Pattern: "/agent/commands/{command_id}/result"},
	{Method: http.MethodPost, Pattern: "/agent/telemetry"},
	{Method: http.MethodPost, Pattern: "/agent/events"},
}

var internalRouteSpecs = []routeSpec{
	{Method: http.MethodPost, Pattern: "/internal/alert-candidates"},
}

func findPublicRoute(method string, path string) (publicRouteSpec, bool, bool) {
	var pathMatched bool
	for _, route := range publicRouteSpecs {
		if routePatternMatches(route.Pattern, path) {
			pathMatched = true
			if route.Method == method {
				return route, true, false
			}
		}
	}
	return publicRouteSpec{}, false, pathMatched
}

func findRoute(method string, path string, routes []routeSpec) (routeSpec, bool, bool) {
	var pathMatched bool
	for _, route := range routes {
		if routePatternMatches(route.Pattern, path) {
			pathMatched = true
			if route.Method == method {
				return route, true, false
			}
		}
	}
	return routeSpec{}, false, pathMatched
}

func routePatternMatches(pattern string, path string) bool {
	patternParts := splitRoute(pattern)
	pathParts := splitRoute(path)
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i, patternPart := range patternParts {
		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			if pathParts[i] == "" {
				return false
			}
			continue
		}
		if patternPart != pathParts[i] {
			return false
		}
	}
	return true
}

func splitRoute(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}
