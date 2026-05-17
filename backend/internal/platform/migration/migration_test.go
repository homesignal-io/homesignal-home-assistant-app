package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGooseSections(t *testing.T) {
	up, down, err := ParseGooseSections(`
-- ignored before directive
-- +goose Up
CREATE TABLE example (
  id text PRIMARY KEY
);
-- +goose Down
DROP TABLE example;
`)
	if err != nil {
		t.Fatalf("ParseGooseSections returned error: %v", err)
	}
	if !strings.Contains(up, "CREATE TABLE example") {
		t.Fatalf("expected Up section to contain create statement, got %q", up)
	}
	if !strings.Contains(down, "DROP TABLE example") {
		t.Fatalf("expected Down section to contain drop statement, got %q", down)
	}
}

func TestParseGooseSectionsRequiresUpDirective(t *testing.T) {
	_, _, err := ParseGooseSections(`CREATE TABLE example (id text);`)
	if err == nil {
		t.Fatal("expected missing Up directive to fail")
	}
}

func TestLoadDirOrdersMigrationsAndRejectsDuplicateVersions(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "000002_second.sql", "CREATE TABLE second_table (id text PRIMARY KEY);")
	writeMigration(t, dir, "000001_first.sql", "CREATE TABLE first_table (id text PRIMARY KEY);")

	migrations, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}
	if migrations[0].Version != "000001" || migrations[1].Version != "000002" {
		t.Fatalf("expected migrations ordered by version, got %#v", migrations)
	}

	writeMigration(t, dir, "000002_duplicate.sql", "CREATE TABLE duplicate_table (id text PRIMARY KEY);")
	_, err = LoadDir(dir)
	if err == nil {
		t.Fatal("expected duplicate migration versions to fail")
	}
}

func writeMigration(t *testing.T, dir, filename, upSQL string) {
	t.Helper()
	body := "-- +goose Up\n" + upSQL + "\n-- +goose Down\n"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write migration fixture: %v", err)
	}
}
