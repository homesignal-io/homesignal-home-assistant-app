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
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/authn"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/config"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/database"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/readmodels"
	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/platform/secrets"
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

	options := []controlplane.Option{}
	databaseConfig := database.LoadConfigFromEnv()
	cognitoConfig := authn.LoadCognitoConfigFromEnv()
	if databaseConfig.URL == "" && databaseConfig.SecretID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		secretClient, err := secrets.NewSecretsManagerClient(ctx, cfg.AWSRegion)
		if err == nil {
			databaseConfig.URL, err = secrets.ReadString(ctx, secretClient, databaseConfig.SecretID)
		}
		cancel()
		if err != nil {
			logger.Error("resolve control-plane database secret", "error", err)
			os.Exit(1)
		}
	}
	if databaseConfig.URL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err := database.Open(ctx, databaseConfig)
		cancel()
		if err != nil {
			logger.Error("open control-plane database", "error", err)
			os.Exit(1)
		}
		defer db.Close()
		options = append(options, controlplane.WithPublicReadModels(readmodels.Store{DB: db}))

		if cognitoConfig.Enabled() {
			verifier, err := authn.NewCognitoVerifier(cognitoConfig)
			if err != nil {
				logger.Error("configure Cognito verifier", "error", err)
				os.Exit(1)
			}
			options = append(options, controlplane.WithHumanAuthenticator(authn.HumanAuthenticator{
				Verifier: verifier,
				Users:    authn.PostgresAuthRepository{DB: db},
			}))
		}
	} else if cognitoConfig.Enabled() {
		logger.Error("Cognito human authentication requires database configuration")
		os.Exit(1)
	}

	app := controlplane.New(cfg, logger, options...)

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
