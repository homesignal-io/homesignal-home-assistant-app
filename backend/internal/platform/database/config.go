package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const DriverName = "pgx"

type Config struct {
	URL             string
	SecretID        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func LoadConfigFromEnv() Config {
	return Config{
		URL:             firstNonEmpty(os.Getenv("HOMESIGNAL_DATABASE_URL"), os.Getenv("DATABASE_URL")),
		SecretID:        os.Getenv("HOMESIGNAL_DATABASE_SECRET_ID"),
		MaxOpenConns:    envInt("HOMESIGNAL_DATABASE_MAX_OPEN_CONNS", 5),
		MaxIdleConns:    envInt("HOMESIGNAL_DATABASE_MAX_IDLE_CONNS", 2),
		ConnMaxLifetime: envDuration("HOMESIGNAL_DATABASE_CONN_MAX_LIFETIME", 30*time.Minute),
	}
}

func (c Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("database URL is required")
	}
	if c.MaxOpenConns < 1 {
		return fmt.Errorf("max open connections must be greater than zero")
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("max idle connections must not be negative")
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("max idle connections must not exceed max open connections")
	}
	if c.ConnMaxLifetime < 0 {
		return fmt.Errorf("connection max lifetime must not be negative")
	}
	return nil
}

func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	db, err := sql.Open(DriverName, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
