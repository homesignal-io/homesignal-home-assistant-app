package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(Options{DefaultVersion: "test-version"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Environment != "local" {
		t.Fatalf("Environment = %q, want local", cfg.Environment)
	}
	if cfg.ServiceName != "control-plane" {
		t.Fatalf("ServiceName = %q, want control-plane", cfg.ServiceName)
	}
	if cfg.Version != "test-version" {
		t.Fatalf("Version = %q, want test-version", cfg.Version)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.AWSRegion != "" {
		t.Fatalf("AWSRegion = %q, want empty", cfg.AWSRegion)
	}
}

func TestLoadRejectsDevEnvironment(t *testing.T) {
	t.Setenv("HOMESIGNAL_ENV", "dev")

	_, err := Load(Options{DefaultVersion: "test-version"})
	if err == nil {
		t.Fatal("Load returned nil error for dev environment")
	}
}

func TestLoadStaging(t *testing.T) {
	t.Setenv("HOMESIGNAL_ENV", "staging")
	t.Setenv("HOMESIGNAL_VERSION", "staging-version")
	t.Setenv("HOMESIGNAL_HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("HOMESIGNAL_AWS_REGION", "us-east-1")

	cfg, err := Load(Options{DefaultVersion: "test-version"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Environment != "staging" {
		t.Fatalf("Environment = %q, want staging", cfg.Environment)
	}
	if cfg.Version != "staging-version" {
		t.Fatalf("Version = %q, want staging-version", cfg.Version)
	}
	if cfg.HTTPAddr != "127.0.0.1:9000" {
		t.Fatalf("HTTPAddr = %q, want 127.0.0.1:9000", cfg.HTTPAddr)
	}
	if cfg.AWSRegion != "us-east-1" {
		t.Fatalf("AWSRegion = %q, want us-east-1", cfg.AWSRegion)
	}
}
