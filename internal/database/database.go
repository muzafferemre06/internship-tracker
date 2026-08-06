package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string, migrations fs.FS) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is required")
	}
	if migrations == nil {
		return nil, errors.New("migration filesystem is required")
	}

	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := prepare(ctx, db, migrations); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ReadinessChecker verifies that an already initialized database can still
// answer queries and contains every migration bundled with this release.
// It deliberately does not apply migrations; startup owns that mutating step.
type ReadinessChecker struct {
	db             *sql.DB
	migrationNames []string
}

func NewReadinessChecker(db *sql.DB, migrations fs.FS) (*ReadinessChecker, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	names, err := MigrationNames(migrations)
	if err != nil {
		return nil, err
	}
	return &ReadinessChecker{db: db, migrationNames: names}, nil
}

func (c *ReadinessChecker) Check(ctx context.Context) error {
	if c == nil || c.db == nil {
		return errors.New("database readiness checker is unavailable")
	}
	if err := c.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	rows, err := c.db.QueryContext(ctx, "SELECT name FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{}, len(c.migrationNames))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan applied migration: %w", err)
		}
		applied[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}
	for _, name := range c.migrationNames {
		if _, ok := applied[name]; !ok {
			return fmt.Errorf("migration %q is not applied", name)
		}
	}
	return nil
}

// Backup creates a transactionally consistent SQLite snapshot using VACUUM INTO.
// The destination must not exist. Callers are responsible for placing the
// snapshot atomically and applying their file-permission policy.
func Backup(ctx context.Context, db *sql.DB, destination string) error {
	if db == nil {
		return errors.New("sqlite database is required")
	}
	if strings.TrimSpace(destination) == "" {
		return errors.New("backup destination is required")
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return fmt.Errorf("vacuum sqlite backup into %q: %w", destination, err)
	}
	return nil
}

func prepare(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite database: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if err := runMigrations(ctx, db, migrations); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func runMigrations(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create migration registry: %w", err)
	}

	names, err := MigrationNames(migrations)
	if err != nil {
		return err
	}

	for _, name := range names {
		if err := applyMigration(ctx, db, migrations, name); err != nil {
			return err
		}
	}
	return nil
}

// MigrationNames returns the sorted names of every migration bundled with a
// release. Read-only tools use it to verify a snapshot without opening the
// production database through Open (which would apply migrations).
func MigrationNames(migrations fs.FS) ([]string, error) {
	if migrations == nil {
		return nil, errors.New("migration filesystem is required")
	}
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return nil, fmt.Errorf("read migration directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, errors.New("no .sql migrations found")
	}
	return names, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migrations fs.FS, name string) error {
	var applied bool
	if err := db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = ?)",
		name,
	).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %q: %w", name, err)
	}
	if applied {
		return nil
	}

	contents, err := fs.ReadFile(migrations, name)
	if err != nil {
		return fmt.Errorf("read migration %q: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", name, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
		return fmt.Errorf("execute migration %q: %w", name, err)
	}
	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO schema_migrations(name) VALUES (?)",
		name,
	); err != nil {
		return fmt.Errorf("record migration %q: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", name, err)
	}
	return nil
}
