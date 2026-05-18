package app

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/homesignal-io/homesignal-home-assistant-app/telemetry-ingest/internal/pipeline"
)

type Server struct {
	Pipeline *pipeline.RuntimePipeline
	Version  string
	Commit   string
}

func NewHandler(server Server) http.Handler {
	if server.Pipeline == nil {
		server.Pipeline = pipeline.NewRuntimePipeline(&pipeline.MemoryWriter{}, &pipeline.MemoryFailureSink{})
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
			Route:  route,
			Device: deviceContextFromHeaders(r),
			Body:   body,
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

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
