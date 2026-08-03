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
	assertTableExists(t, db, "listings")
	assertMigrationCount(t, db, 4)
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	db, err = Open(ctx, path, migrations)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	assertMigrationCount(t, db, 4)
}

func TestOpenRejectsMissingMigrations(t *testing.T) {
	_, err := Open(context.Background(), ":memory:", os.DirFS(t.TempDir()))
	if err == nil {
		t.Fatal("expected missing migrations to fail")
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
