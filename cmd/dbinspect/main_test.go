package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/database"
)

func TestRunReportsSafePersistentDatabaseIdentityAndRowCounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tracker.db")
	db, err := database.Open(context.Background(), path, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO companies(name, priority_group) VALUES ('Example', 'primary');
		INSERT INTO company_sources(company_id, source_type, url, adapter_type, source_key)
		VALUES (1, 'fixture', 'https://example.test', 'fixture', 'example');
		INSERT INTO listings(id, company_id, source_id, title, canonical_url, raw_text, content_hash, first_seen_at, last_seen_at)
		VALUES ('listing-1', 1, 1, 'Staj', 'https://example.test/1', 'staj', 'hash', '2026-08-10T00:00:00Z', '2026-08-10T00:00:00Z');
		INSERT INTO opportunities(id, company_id, normalized_title) VALUES ('opportunity-1', 1, 'staj');
		INSERT INTO listing_opportunities(listing_id, opportunity_id, match_method, match_reason)
		VALUES ('listing-1', 'opportunity-1', 'new', 'new_listing');
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := run(context.Background(), []string{"-database", path}, &output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, expected := range []string{`"database_path":"` + path + `"`, `"listings":1`, `"opportunities":1`, `"memberships":1`} {
		if !strings.Contains(got, expected) {
			t.Fatalf("diagnostic output missing %s: %s", expected, got)
		}
	}
}

func TestRunRejectsMissingDatabase(t *testing.T) {
	if err := run(context.Background(), nil, &bytes.Buffer{}); err == nil {
		t.Fatal("missing database must fail")
	}
}
