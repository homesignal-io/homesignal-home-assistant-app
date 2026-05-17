package migration

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var migrationFilePattern = regexp.MustCompile(`^([0-9]+)_.+\.sql$`)

type Migration struct {
	Version  string
	Filename string
	UpSQL    string
	DownSQL  string
	Checksum string
}

type AppliedMigration struct {
	Version   string
	Filename  string
	Checksum  string
	AppliedAt time.Time
}

type Status struct {
	Migration Migration
	Applied   *AppliedMigration
}

type Runner struct {
	DB         *sql.DB
	Migrations []Migration
}

func LoadDir(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		matches := migrationFilePattern.FindStringSubmatch(filename)
		if matches == nil {
			continue
		}

		path := filepath.Join(dir, filename)
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", filename, err)
		}

		up, down, err := ParseGooseSections(string(body))
		if err != nil {
			return nil, fmt.Errorf("parse migration %s: %w", filename, err)
		}
		if strings.TrimSpace(up) == "" {
			return nil, fmt.Errorf("migration %s has an empty Up section", filename)
		}

		hash := sha256.Sum256(body)
		migrations = append(migrations, Migration{
			Version:  matches[1],
			Filename: filename,
			UpSQL:    strings.TrimSpace(up),
			DownSQL:  strings.TrimSpace(down),
			Checksum: hex.EncodeToString(hash[:]),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].Version == migrations[i].Version {
			return nil, fmt.Errorf("duplicate migration version %s", migrations[i].Version)
		}
	}

	return migrations, nil
}

func ParseGooseSections(sqlText string) (up string, down string, err error) {
	var upBuilder strings.Builder
	var downBuilder strings.Builder
	section := ""
	foundUp := false

	scanner := bufio.NewScanner(strings.NewReader(sqlText))
	for scanner.Scan() {
		line := scanner.Text()
		switch strings.TrimSpace(line) {
		case "-- +goose Up":
			section = "up"
			foundUp = true
			continue
		case "-- +goose Down":
			section = "down"
			continue
		case "-- +goose StatementBegin", "-- +goose StatementEnd":
			continue
		}

		switch section {
		case "up":
			upBuilder.WriteString(line)
			upBuilder.WriteByte('\n')
		case "down":
			downBuilder.WriteString(line)
			downBuilder.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	if !foundUp {
		return "", "", fmt.Errorf("missing -- +goose Up directive")
	}

	return upBuilder.String(), downBuilder.String(), nil
}

func (r Runner) Status(ctx context.Context) ([]Status, error) {
	applied, err := r.applied(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]Status, 0, len(r.Migrations))
	for _, m := range r.Migrations {
		status := Status{Migration: m}
		if appliedMigration, ok := applied[m.Version]; ok {
			appliedCopy := appliedMigration
			status.Applied = &appliedCopy
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (r Runner) Pending(ctx context.Context) ([]Migration, error) {
	statuses, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}

	pending := make([]Migration, 0)
	for _, status := range statuses {
		if status.Applied == nil {
			pending = append(pending, status.Migration)
			continue
		}
		if status.Applied.Checksum != status.Migration.Checksum {
			return nil, fmt.Errorf("migration %s checksum changed after apply", status.Migration.Filename)
		}
	}

	return pending, nil
}

func (r Runner) Apply(ctx context.Context) ([]Migration, error) {
	pending, err := r.Pending(ctx)
	if err != nil {
		return nil, err
	}

	applied := make([]Migration, 0, len(pending))
	for _, m := range pending {
		tx, err := r.DB.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("begin migration %s: %w", m.Filename, err)
		}

		for _, statement := range splitSQLStatements(m.UpSQL) {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("apply migration %s: %w", m.Filename, err)
			}
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations (version, filename, checksum) VALUES ($1, $2, $3)`,
			m.Version,
			m.Filename,
			m.Checksum,
		); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("record migration %s: %w", m.Filename, err)
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit migration %s: %w", m.Filename, err)
		}
		applied = append(applied, m)
	}

	return applied, nil
}

func (r Runner) applied(ctx context.Context) (map[string]AppliedMigration, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("migration runner requires a database")
	}
	if err := r.ensureSchemaTable(ctx); err != nil {
		return nil, err
	}

	rows, err := r.DB.QueryContext(
		ctx,
		`SELECT version, filename, checksum, applied_at FROM schema_migrations ORDER BY version`,
	)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]AppliedMigration{}
	for rows.Next() {
		var migration AppliedMigration
		if err := rows.Scan(&migration.Version, &migration.Filename, &migration.Checksum, &migration.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[migration.Version] = migration
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations: %w", err)
	}

	return applied, nil
}

func (r Runner) ensureSchemaTable(ctx context.Context) error {
	_, err := r.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  filename text NOT NULL,
  checksum text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
)`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func splitSQLStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
	}
	return statements
}
