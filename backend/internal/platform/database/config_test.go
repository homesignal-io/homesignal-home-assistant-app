package database

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	cfg := Config{
		URL:             "postgres://example",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to validate: %v", err)
	}
}

func TestConfigValidateRejectsMissingURL(t *testing.T) {
	cfg := Config{MaxOpenConns: 1}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing URL to fail")
	}
}

func TestConfigValidateRejectsIdleGreaterThanOpen(t *testing.T) {
	cfg := Config{
		URL:          "postgres://example",
		MaxOpenConns: 1,
		MaxIdleConns: 2,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid pool sizing to fail")
	}
}
