package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func enabledConfig(directory string) Config {
	return Config{
		Enabled: "true", Directory: directory, Time: "02:00", Timezone: "Europe/Istanbul", Retention: "2",
	}
}

func TestNewDoesNotCreateFilesWhenDisabled(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "backups")
	service, err := New(nil, Config{Enabled: "false", Directory: directory}, nil)
	if err != nil {
		t.Fatalf("new disabled backup service: %v", err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("disabled service unexpectedly created backup directory: %v", err)
	}
	if err := service.Wait(context.Background()); err != nil {
		t.Fatalf("disabled service did not stop: %v", err)
	}
}

func TestNewRejectsEnabledInvalidConfiguration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, cfg := range []Config{
		{Enabled: "invalid"},
		{Enabled: "true", Time: "02:00", Timezone: "UTC", Retention: "1"},
		{Enabled: "true", Directory: t.TempDir(), Time: "invalid", Timezone: "UTC", Retention: "1"},
		{Enabled: "true", Directory: t.TempDir(), Time: "02:00", Timezone: "invalid", Retention: "1"},
		{Enabled: "true", Directory: t.TempDir(), Time: "02:00", Timezone: "UTC", Retention: "0"},
	} {
		if _, err := New(db, cfg, nil); err == nil {
			t.Fatalf("expected invalid config to fail: %#v", cfg)
		}
	}
}

func TestRunCreatesConsistentRestorableSnapshotWithSecureRetention(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "backups")
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	source, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err := source.Exec("CREATE TABLE listings (id INTEGER PRIMARY KEY, title TEXT NOT NULL)"); err != nil {
		t.Fatalf("create source table: %v", err)
	}
	if _, err := source.Exec("INSERT INTO listings(title) VALUES ('first')"); err != nil {
		t.Fatalf("seed source database: %v", err)
	}

	service, err := New(source, enabledConfig(directory), nil)
	if err != nil {
		t.Fatalf("new backup service: %v", err)
	}
	result, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if _, err := source.Exec("INSERT INTO listings(title) VALUES ('after snapshot')"); err != nil {
		t.Fatalf("mutate source after snapshot: %v", err)
	}

	backupInfo, err := os.Stat(result.Path)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("backup permissions = %o, want 600", backupInfo.Mode().Perm())
	}
	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat backup directory: %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("backup directory permissions = %o, want 700", directoryInfo.Mode().Perm())
	}

	restored, err := sql.Open("sqlite", result.Path)
	if err != nil {
		t.Fatalf("open snapshot for restore: %v", err)
	}
	defer restored.Close()
	var count int
	if err := restored.QueryRow("SELECT COUNT(*) FROM listings").Scan(&count); err != nil {
		t.Fatalf("read restored snapshot: %v", err)
	}
	if count != 1 {
		t.Fatalf("restored snapshot has %d rows, want 1", count)
	}
	for i := 0; i < 2; i++ {
		time.Sleep(time.Millisecond)
		if _, err := service.Run(context.Background()); err != nil {
			t.Fatalf("create retained backup %d: %v", i, err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read backup directory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("retained backup count = %d, want 2", len(entries))
	}
}

func TestNextUsesConfiguredTimezone(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service, err := New(db, enabledConfig(t.TempDir()), nil)
	if err != nil {
		t.Fatal(err)
	}
	service.at = 2*time.Hour + 30*time.Minute
	after := time.Date(2026, 8, 6, 23, 0, 0, 0, time.UTC).In(service.location)
	next := service.next(after)
	want := time.Date(2026, 8, 7, 2, 30, 0, 0, service.location)
	if !next.Equal(want) || next.Location() != service.location {
		t.Fatalf("next backup = %s (%s), want %s (%s)", next, next.Location(), want, service.location)
	}
}
