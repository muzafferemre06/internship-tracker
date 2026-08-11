package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationsOnlyOnce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tracker.db")
	migrations := os.DirFS("../../migrations")

	db, err := Open(ctx, path, migrations)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	migrationNames, err := MigrationNames(migrations)
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	assertTableExists(t, db, "listings")
	assertTableExists(t, db, "program_windows")
	assertMigrationCount(t, db, len(migrationNames))
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	db, err = Open(ctx, path, migrations)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	assertMigrationCount(t, db, len(migrationNames))
}

func TestOpenRejectsMissingMigrations(t *testing.T) {
	_, err := Open(context.Background(), ":memory:", os.DirFS(t.TempDir()))
	if err == nil {
		t.Fatal("expected missing migrations to fail")
	}
}

func TestReadinessCheckerVerifiesDatabaseAndAppliedMigrations(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "tracker.db"), os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	checker, err := NewReadinessChecker(db, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("create readiness checker: %v", err)
	}
	if err := checker.Check(ctx); err != nil {
		t.Fatalf("expected migrated database to be ready: %v", err)
	}
	if _, err := db.Exec("DELETE FROM schema_migrations WHERE name = ?", "005_web_push.sql"); err != nil {
		t.Fatalf("remove migration record: %v", err)
	}
	if err := checker.Check(ctx); err == nil {
		t.Fatal("expected readiness to fail when a bundled migration is missing")
	}
}

func assertTableExists(t *testing.T, db queryRower, name string) {
	t.Helper()
	var found string
	if err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
		name,
	).Scan(&found); err != nil {
		t.Fatalf("find table %q: %v", name, err)
	}
}

func assertMigrationCount(t *testing.T, db queryRower, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d migration, got %d", expected, count)
	}
}

type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}
