package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/homesignal-io/homesignal-home-assistant/backend/internal/platform/migration"
)

func main() {
	var (
		mode        = flag.String("mode", "plan", "one of: plan, status, up")
		dir         = flag.String("dir", "migrations", "directory containing Goose-style SQL migrations")
		databaseURL = flag.String("database-url", "", "PostgreSQL connection URL; defaults to HOMESIGNAL_DATABASE_URL or DATABASE_URL")
		timeout     = flag.Duration("timeout", 30*time.Second, "migration command timeout")
	)
	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = firstNonEmpty(os.Getenv("HOMESIGNAL_DATABASE_URL"), os.Getenv("DATABASE_URL"))
	}

	migrations, err := migration.LoadDir(*dir)
	if err != nil {
		exitf(1, "load migrations: %v", err)
	}

	switch *mode {
	case "plan":
		printFilesystemPlan(migrations)
	case "status", "up":
		if *databaseURL == "" {
			exitf(2, "missing database URL; set HOMESIGNAL_DATABASE_URL, DATABASE_URL, or -database-url")
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		db, err := sql.Open("pgx", *databaseURL)
		if err != nil {
			exitf(1, "open database: %v", err)
		}
		defer db.Close()
		if err := db.PingContext(ctx); err != nil {
			exitf(1, "ping database: %v", err)
		}

		runner := migration.Runner{DB: db, Migrations: migrations}
		if *mode == "status" {
			statuses, err := runner.Status(ctx)
			if err != nil {
				exitf(1, "read migration status: %v", err)
			}
			printStatus(statuses)
			return
		}

		applied, err := runner.Apply(ctx)
		if err != nil {
			exitf(1, "apply migrations: %v", err)
		}
		if len(applied) == 0 {
			fmt.Println("No pending migrations")
			return
		}
		for _, appliedMigration := range applied {
			fmt.Printf("Applied %s %s\n", appliedMigration.Version, appliedMigration.Filename)
		}
	default:
		exitf(2, "unsupported mode %q; use plan, status, or up", *mode)
	}
}

func printFilesystemPlan(migrations []migration.Migration) {
	if len(migrations) == 0 {
		fmt.Println("No migrations found")
		return
	}
	for _, m := range migrations {
		fmt.Printf("Migration %s %s checksum=%s\n", m.Version, m.Filename, shortChecksum(m.Checksum))
	}
}

func printStatus(statuses []migration.Status) {
	for _, status := range statuses {
		state := "pending"
		if status.Applied != nil {
			state = "applied"
		}
		fmt.Printf("%s %s %s checksum=%s\n", state, status.Migration.Version, status.Migration.Filename, shortChecksum(status.Migration.Checksum))
	}
}

func shortChecksum(checksum string) string {
	if len(checksum) <= 12 {
		return checksum
	}
	return checksum[:12]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func exitf(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(code)
}
