package config

import (
	"fmt"
	"os"
)

type Options struct {
	DefaultVersion string
}

type Config struct {
	Environment string
	ServiceName string
	Version     string
	HTTPAddr    string
	AWSRegion   string
}

func Load(options Options) (Config, error) {
	cfg := Config{
		Environment: getenvDefault("HOMESIGNAL_ENV", "local"),
		ServiceName: getenvDefault("HOMESIGNAL_SERVICE_NAME", "control-plane"),
		Version:     getenvDefault("HOMESIGNAL_VERSION", options.DefaultVersion),
		HTTPAddr:    getenvDefault("HOMESIGNAL_HTTP_ADDR", ":8080"),
		AWSRegion:   getenvDefault("HOMESIGNAL_AWS_REGION", os.Getenv("AWS_REGION")),
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}

	if err := validateEnvironment(cfg.Environment); err != nil {
		return Config{}, err
	}
	if cfg.ServiceName == "" {
		return Config{}, fmt.Errorf("HOMESIGNAL_SERVICE_NAME must not be empty")
	}
	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HOMESIGNAL_HTTP_ADDR must not be empty")
	}

	return cfg, nil
}

func getenvDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func validateEnvironment(environment string) error {
	switch environment {
	case "local", "test", "staging", "production":
		return nil
	case "dev", "development":
		return fmt.Errorf("HOMESIGNAL_ENV=%q is not a launch environment; use local or staging", environment)
	default:
		return fmt.Errorf("unsupported HOMESIGNAL_ENV=%q", environment)
	}
}
