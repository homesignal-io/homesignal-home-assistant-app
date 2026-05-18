package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/telemetry-ingest/internal/pipeline"
)

type Server struct {
	Pipeline        *pipeline.RuntimePipeline
	LifecycleWriter pipeline.LifecycleWriter
	Version         string
	Commit          string
}

func NewHandler(server Server) http.Handler {
	if server.Pipeline == nil {
		server.Pipeline = pipeline.NewRuntimePipeline(&pipeline.MemoryWriter{}, &pipeline.MemoryFailureSink{})
	}
	if server.LifecycleWriter == nil {
		server.LifecycleWriter = &pipeline.MemoryLifecycleWriter{}
	}
	if server.Version == "" {
		server.Version = "dev"
	}
	if server.Commit == "" {
		server.Commit = "unknown"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ready": true})
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": server.Version, "commit": server.Commit})
	})
	mux.HandleFunc("/agent/telemetry", runtimeHandler(server.Pipeline, pipeline.RouteTelemetry))
	mux.HandleFunc("/agent/events", runtimeHandler(server.Pipeline, pipeline.RouteEvents))
	mux.HandleFunc("/internal/iot/lifecycle", lifecycleHandler(server.LifecycleWriter))
	return mux
}

func runtimeHandler(runtimePipeline *pipeline.RuntimePipeline, route pipeline.RuntimeRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return
		}
		result, err := runtimePipeline.Ingest(r.Context(), pipeline.IngestRequest{
			Route:      route,
			Device:     deviceContextFromHeaders(r),
			Credential: credentialFromHeaders(r),
			Body:       body,
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

func deviceContextFromHeaders(r *http.Request) pipeline.AuthenticatedDeviceContext {
	return pipeline.AuthenticatedDeviceContext{
		DeviceID:               r.Header.Get("X-HomeSignal-Device-ID"),
		SiteID:                 r.Header.Get("X-HomeSignal-Site-ID"),
		OrgID:                  r.Header.Get("X-HomeSignal-Org-ID"),
		CertificateFingerprint: r.Header.Get("X-Client-Cert-Fingerprint"),
		CertificateSerial:      r.Header.Get("X-Client-Cert-Serial"),
	}
}

func credentialFromHeaders(r *http.Request) pipeline.TransportCredential {
	return pipeline.TransportCredential{
		CertificateFingerprint: r.Header.Get("X-Client-Cert-Fingerprint"),
		CertificateSerial:      r.Header.Get("X-Client-Cert-Serial"),
		CertificateIssuer:      r.Header.Get("X-Client-Cert-Issuer"),
	}
}

func lifecycleHandler(writer pipeline.LifecycleWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return
		}
		event, err := parseLifecycleEvent(body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := writer.WriteLifecycle(r.Context(), event)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

func parseLifecycleEvent(body []byte) (pipeline.LifecycleEvent, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return pipeline.LifecycleEvent{}, fmt.Errorf("decode lifecycle event: %w", err)
	}
	eventType := firstString(raw, "event_type", "lifecycle_event", "eventType")
	clientID := firstString(raw, "client_id", "clientId", "clientid")
	if eventType == "" {
		return pipeline.LifecycleEvent{}, fmt.Errorf("lifecycle event_type is required")
	}
	if clientID == "" {
		return pipeline.LifecycleEvent{}, fmt.Errorf("lifecycle client_id is required")
	}
	observedAt, err := firstTime(raw, "observed_at", "timestamp", "eventTimestamp")
	if err != nil {
		return pipeline.LifecycleEvent{}, err
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	return pipeline.LifecycleEvent{
		EventType:           strings.TrimSpace(eventType),
		ClientID:            strings.TrimSpace(clientID),
		PrincipalIdentifier: firstString(raw, "principal_identifier", "principalIdentifier", "principal_id", "principal"),
		SessionIdentifier:   firstString(raw, "session_identifier", "sessionIdentifier"),
		VersionNumber:       firstString(raw, "version_number", "versionNumber"),
		ObservedAt:          observedAt.UTC(),
		ReceivedAt:          time.Now().UTC(),
	}, nil
}

func firstString(raw map[string]json.RawMessage, fields ...string) string {
	for _, field := range fields {
		payload, ok := raw[field]
		if !ok {
			continue
		}
		var value string
		if json.Unmarshal(payload, &value) == nil {
			return strings.TrimSpace(value)
		}
		var number json.Number
		decoder := json.NewDecoder(strings.NewReader(string(payload)))
		decoder.UseNumber()
		if decoder.Decode(&number) == nil {
			return number.String()
		}
	}
	return ""
}

func firstTime(raw map[string]json.RawMessage, fields ...string) (time.Time, error) {
	value := firstString(raw, fields...)
	if value == "" {
		return time.Time{}, nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts, nil
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse lifecycle timestamp: %w", err)
	}
	if number > 9999999999 {
		return time.UnixMilli(number), nil
	}
	return time.Unix(number, 0), nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
