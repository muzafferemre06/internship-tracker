// Command dbinspect reports a safe, read-only identity and row-count snapshot
// for the configured SQLite database. It never prints listing text, notes, or
// secrets and is intended for restart/deployment diagnosis.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type inspection struct {
	DatabasePath      string `json:"database_path"`
	SizeBytes         int64  `json:"size_bytes"`
	ModifiedAt        string `json:"modified_at"`
	Companies         int    `json:"companies"`
	Sources           int    `json:"sources"`
	Listings          int    `json:"listings"`
	Opportunities     int    `json:"opportunities"`
	Memberships       int    `json:"memberships"`
	Analyses          int    `json:"analyses"`
	Applications      int    `json:"applications"`
	ScanRuns          int    `json:"scan_runs"`
	AppliedMigrations int    `json:"applied_migrations"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "dbinspect:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("dbinspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	databasePath := flags.String("database", "", "existing SQLite database path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if *databasePath == "" {
		return errors.New("-database is required")
	}
	absolute, err := filepath.Abs(*databasePath)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("stat database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("database must be a regular file")
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open database read-only: %w", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		return fmt.Errorf("enable query-only mode: %w", err)
	}
	result := inspection{
		DatabasePath: absolute, SizeBytes: info.Size(), ModifiedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
	queries := []struct {
		name   string
		table  string
		target *int
	}{
		{"companies", "companies", &result.Companies},
		{"sources", "company_sources", &result.Sources},
		{"listings", "listings", &result.Listings},
		{"opportunities", "opportunities", &result.Opportunities},
		{"memberships", "listing_opportunities", &result.Memberships},
		{"analyses", "listing_analyses", &result.Analyses},
		{"applications", "application_tracking", &result.Applications},
		{"scan runs", "scan_runs", &result.ScanRuns},
		{"migrations", "schema_migrations", &result.AppliedMigrations},
	}
	for _, query := range queries {
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+query.table).Scan(query.target); err != nil {
			return fmt.Errorf("count %s: %w", query.name, err)
		}
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}
