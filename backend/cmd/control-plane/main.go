package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/controlplane"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/lambdaproxy"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/config"
)

var version = "dev"

func main() {
	cfg, err := config.Load(config.Options{DefaultVersion: version})
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		"service", cfg.ServiceName,
		"environment", cfg.Environment,
		"version", cfg.Version,
		"aws_region", cfg.AWSRegion,
	)

	app := controlplane.New(cfg, logger)

	if runtimeAPI := os.Getenv("AWS_LAMBDA_RUNTIME_API"); runtimeAPI != "" {
		if err := lambdaproxy.Run(context.Background(), runtimeAPI, app, logger); err != nil {
			logger.Error("lambda runtime stopped", "error", err)
			os.Exit(1)
		}
		return
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting control plane", "addr", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("control plane stopped", "error", err)
		os.Exit(1)
	}
}
