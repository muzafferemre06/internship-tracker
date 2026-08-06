package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestRunCreatesGeneratedSnapshot(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "tracker.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE entries (value TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO entries(value) VALUES ('before deploy')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	originalNow := now
	now = func() time.Time { return time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = originalNow })
	var output bytes.Buffer
	directory := filepath.Join(root, "snapshots")
	if err := run(context.Background(), []string{"-database", databasePath, "-directory", directory}, &output); err != nil {
		t.Fatalf("run backup: %v", err)
	}
	path := strings.TrimPrefix(strings.TrimSpace(output.String()), "snapshot=")
	if filepath.Dir(path) != directory {
		t.Fatalf("snapshot directory = %q, want %q", filepath.Dir(path), directory)
	}
	if !strings.HasPrefix(filepath.Base(path), "internship-tracker-20260806T100000.000000000Z-") {
		t.Fatalf("snapshot name is not generated safely: %q", filepath.Base(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat snapshot: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot mode = %o, want 600", info.Mode().Perm())
	}
}

func TestRunRequiresPaths(t *testing.T) {
	if err := run(context.Background(), nil, &bytes.Buffer{}); err == nil {
		t.Fatal("expected missing paths to fail")
	}
}
