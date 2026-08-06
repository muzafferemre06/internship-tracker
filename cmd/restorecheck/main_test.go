package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/backup"
	"github.com/muzaffer/internship-tracker/internal/database"
)

func TestRunVerifiesMigrationsUsingReadOnlySnapshot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source, err := database.Open(ctx, filepath.Join(root, "tracker.db"), os.DirFS("../../migrations"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	result, err := backup.Create(ctx, source, filepath.Join(root, "backups"), time.Now())
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	var output bytes.Buffer
	if err := run(ctx, []string{"-backup", result.Path, "-migrations", "../../migrations"}, &output); err != nil {
		t.Fatalf("run restorecheck: %v", err)
	}
	if got := strings.TrimSpace(output.String()); got != "verified="+result.Path {
		t.Fatalf("output = %q", got)
	}
	for _, sidecar := range []string{result.Path + "-journal", result.Path + "-wal", result.Path + "-shm"} {
		if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
			t.Fatalf("restore check created sidecar %q: %v", sidecar, err)
		}
	}
}

func TestRunRequiresBackup(t *testing.T) {
	if err := run(context.Background(), nil, &bytes.Buffer{}); err == nil {
		t.Fatal("expected missing backup to fail")
	}
}
