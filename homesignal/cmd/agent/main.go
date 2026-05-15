package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

const (
	defaultListenAddr = ":8099"
	defaultConfigDir  = "/config"
	defaultDataDir    = "/data"
	coreAPIBaseURL    = "http://supervisor/core/api/"
)

type DeviceIdentity struct {
	InstallationID string    `json:"installation_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type OptionsState struct {
	Present bool                   `json:"present"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type RuntimeState struct {
	Enrollment      *EnrollmentManager
	Options         OptionsState
	SupervisorToken bool
	CoreAPI         CoreAPIClient
}

type CoreAPIClient struct {
	BaseURL  string
	HasToken bool
}

type readyResponse struct {
	Ready                    bool     `json:"ready"`
	Degraded                 bool     `json:"degraded"`
	InstallationID           string   `json:"installation_id,omitempty"`
	OptionsLoaded            bool     `json:"options_loaded"`
	SupervisorToken          bool     `json:"supervisor_token"`
	CoreAPIBaseURL           string   `json:"core_api_base_url"`
	Status                   string   `json:"status"`
	Version                  string   `json:"version"`
	ClaimState               string   `json:"claim_state"`
	EnrollmentDegradedReason string   `json:"enrollment_degraded_reason,omitempty"`
	DegradedReason           string   `json:"degraded_reason,omitempty"`
	DegradedReasons          []string `json:"degraded_reasons,omitempty"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	state, err := loadRuntimeState(os.Getenv("CONFIG_DIR"), os.Getenv("DATA_DIR"), os.Getenv("SUPERVISOR_TOKEN"))
	if err != nil {
		logger.Error("failed to initialize agent", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state.Enrollment.Start(ctx, logger)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = defaultListenAddr
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           newRouter(state),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("homesignal agent ready", "addr", addr, "installation_id", state.Enrollment.Snapshot().InstallationID)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
		os.Exit(1)
	}
}

func loadRuntimeState(configDir, dataDir, supervisorToken string) (RuntimeState, error) {
	return loadRuntimeStateWithClients(configDir, dataDir, supervisorToken, nil, nil, time.Now)
}

func loadRuntimeStateWithClients(configDir, dataDir, supervisorToken string, enrollmentClient HomeSignalEnrollmentClient, provisioner FleetProvisioningClient, now func() time.Time) (RuntimeState, error) {
	if configDir == "" {
		configDir = defaultConfigDir
	}
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	if now == nil {
		now = time.Now
	}

	options, err := loadOptions(filepath.Join(dataDir, "options.json"))
	if err != nil {
		return RuntimeState{}, err
	}

	enrollmentConfig := loadEnrollmentConfig(options)
	if enrollmentClient == nil && enrollmentConfig.HomeSignalAPIBaseURL != "" {
		enrollmentClient = NewHTTPHomeSignalClient(enrollmentConfig.HomeSignalAPIBaseURL)
	}
	if provisioner == nil {
		provisioner = UnsupportedFleetProvisioningClient{}
	}

	record, err := loadDeviceRecord(filepath.Join(configDir, "device.json"), now().UTC())
	if err != nil {
		return RuntimeState{}, err
	}

	enrollment := NewEnrollmentManager(EnrollmentManagerConfig{
		ConfigDir:        configDir,
		DeviceRecordPath: filepath.Join(configDir, "device.json"),
		Config:           enrollmentConfig,
		Client:           enrollmentClient,
		Provisioner:      provisioner,
		Now:              now,
		Record:           record,
	})

	hasToken := supervisorToken != ""
	return RuntimeState{
		Enrollment:      enrollment,
		Options:         options,
		SupervisorToken: hasToken,
		CoreAPI: CoreAPIClient{
			BaseURL:  coreAPIBaseURL,
			HasToken: hasToken,
		},
	}, nil
}

func ensureIdentity(path string) (DeviceIdentity, error) {
	record, err := loadDeviceRecord(path, time.Now().UTC())
	if err != nil {
		return DeviceIdentity{}, err
	}
	return DeviceIdentity{
		InstallationID: record.InstallationID,
		CreatedAt:      record.CreatedAt,
	}, nil
}

func loadOptions(path string) (OptionsState, error) {
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return OptionsState{Present: false}, nil
	}
	if err != nil {
		return OptionsState{}, fmt.Errorf("read options: %w", err)
	}

	options := map[string]interface{}{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &options); err != nil {
			return OptionsState{}, fmt.Errorf("parse options: %w", err)
		}
	}

	return OptionsState{Present: true, Options: options}, nil
}

func newRouter(state RuntimeState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", readyHandler(state))
	mux.HandleFunc("/status", statusHandler(state))
	mux.HandleFunc("/version", versionHandler)
	mux.HandleFunc("/ui", uiHandler(state))
	ui := uiHandler(state)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, "/ui") {
			ui(w, r)
			return
		}
		http.NotFound(w, r)
	})
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

func readyHandler(state RuntimeState) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		response := readiness(state)
		writeJSON(w, http.StatusOK, response)
	}
}

func readiness(state RuntimeState) readyResponse {
	snapshot := state.Enrollment.Snapshot()
	reasons := []string{}
	if !state.SupervisorToken {
		reasons = append(reasons, "SUPERVISOR_TOKEN is not present; Supervisor and Core API calls are disabled")
	}
	reasons = append(reasons, snapshot.DegradedReasons...)

	degraded := len(reasons) > 0
	status := "ready"
	if degraded {
		status = "degraded"
	}

	return readyResponse{
		Ready:                    true,
		Degraded:                 degraded,
		InstallationID:           snapshot.InstallationID,
		OptionsLoaded:            state.Options.Present,
		SupervisorToken:          state.SupervisorToken,
		CoreAPIBaseURL:           state.CoreAPI.BaseURL,
		Status:                   status,
		Version:                  version,
		ClaimState:               string(snapshot.ClaimState),
		EnrollmentDegradedReason: strings.Join(snapshot.DegradedReasons, "; "),
		DegradedReason:           strings.Join(reasons, "; "),
		DegradedReasons:          reasons,
	}
}

func statusHandler(state RuntimeState) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, newStatusResponse(state.Enrollment.Snapshot()))
	}
}

func versionHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":    version,
		"commit":     commit,
		"build_time": buildTime,
	})
}

func uiHandler(state RuntimeState) http.HandlerFunc {
	tmpl := template.Must(template.New("ui").Parse(uiHTML))
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		view := newUIView(state.Enrollment.Snapshot(), readiness(state))
		if err := tmpl.Execute(w, view); err != nil {
			http.Error(w, "failed to render status page", http.StatusInternalServerError)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
