package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/homesignal-io/homesignal-home-assistant/telemetry-ingest/internal/app"
	"github.com/homesignal-io/homesignal-home-assistant/telemetry-ingest/internal/pipeline"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	writer := &pipeline.MemoryWriter{}
	failures := &pipeline.MemoryFailureSink{}
	handler := app.NewHandler(app.Server{
		Pipeline: pipeline.NewRuntimePipeline(writer, failures),
		Version:  version,
		Commit:   commit,
	})

	logger.Info("telemetry ingest ready", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error("telemetry ingest stopped", "error", err)
		os.Exit(1)
	}
}
