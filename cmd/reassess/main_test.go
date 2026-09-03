package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/database"
)

func insertFixture(t *testing.T, db *sql.DB) (listingID, opportunityID string) {
	t.Helper()

	ts := time.Now().Format(time.RFC3339Nano)

	res, err := db.Exec("INSERT INTO companies (name, priority_group) VALUES (?, ?)", "Test Company", "secondary")
	if err != nil {
		t.Fatalf("inserting company: %v", err)
	}
	companyID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("getting company id: %v", err)
	}

	res, err = db.Exec("INSERT INTO company_sources (company_id, source_type, url, adapter_type) VALUES (?, ?, ?, ?)",
		companyID, "careers", "https://example.test/jobs", "static_html")
	if err != nil {
		t.Fatalf("inserting company_source: %v", err)
	}
	sourceID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("getting source id: %v", err)
	}

	listingID = "listing-brazil"
	_, err = db.Exec(`INSERT INTO listings (id, company_id, source_id, title, canonical_url, raw_text, content_hash, first_seen_at, last_seen_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		listingID, companyID, sourceID, "Test Listing", "https://example.test/job/1", "test", "test", ts, ts)
	if err != nil {
		t.Fatalf("inserting listing: %v", err)
	}

	_, err = db.Exec(`INSERT INTO listing_analyses (listing_id, opportunity_type, is_application_open, is_relevant, matching_areas_json, location, work_model, eligibility_status, confidence, provider, analyzed_at, processing_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		listingID, "staj", 1, 1, `["backend"]`, "Sao Paulo, Brazil", "ofis", "uygun", 0.9, "deterministic", ts, "processed")
	if err != nil {
		t.Fatalf("inserting listing_analysis: %v", err)
	}

	opportunityID = "opp-brazil"
	_, err = db.Exec(`INSERT INTO opportunities (id, company_id, normalized_title, visibility_layer, match_score, assessment_reason)
		VALUES (?, ?, ?, ?, ?, ?)`,
		opportunityID, companyID, "test opportunity", "incelenecek", 50, "uncertain_match")
	if err != nil {
		t.Fatalf("inserting opportunity: %v", err)
	}

	_, err = db.Exec(`INSERT INTO listing_opportunities (listing_id, opportunity_id, match_method, match_reason)
		VALUES (?, ?, ?, ?)`,
		listingID, opportunityID, "new", "fixture")
	if err != nil {
		t.Fatalf("inserting listing_opportunity: %v", err)
	}

	_, err = db.Exec(`INSERT INTO opportunity_evidence (opportunity_id, listing_id, source_type, source_url, first_observed_at, last_observed_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		opportunityID, listingID, "web", "https://example.test/job/1", ts, ts)
	if err != nil {
		t.Fatalf("inserting opportunity_evidence: %v", err)
	}

	return listingID, opportunityID
}

func setupTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("creating test db file: %v", err)
	}
	f.Close()

	ctx := context.Background()
	db, err := database.Open(ctx, dbPath, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db.Close()

	insertFixture(t, db)

	return dbPath
}

func writeProfile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	content := `{
  "education": {"university": "Test Universitesi", "department": "Bilgisayar Muhendisligi", "class_year": 3, "gpa": 3.2},
  "focus_areas": ["backend", "veri"],
  "experience": [],
  "location_preferences": {"primary": ["Istanbul"], "summer_other_cities": true, "term_time_part_time_other_cities": false}
}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test profile: %v", err)
	}
	return path
}

func getOpportunityCurrent(t *testing.T, dbPath string) (layer, reason string, score int) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, dbPath, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	err = db.QueryRowContext(ctx, "SELECT visibility_layer, match_score, assessment_reason FROM opportunities WHERE id = 'opp-brazil'").Scan(&layer, &score, &reason)
	if err != nil {
		t.Fatalf("querying opportunity: %v", err)
	}
	return
}

func getNotificationsCount(t *testing.T, dbPath string) int {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, dbPath, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	var count int
	// Some projects might not have the notifications table fully set up in early migrations
	// We do an existence check or just query it assuming migrations create it.
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notifications").Scan(&count)
	if err != nil {
		// If notifications table does not exist, there are 0 notifications.
		if strings.Contains(err.Error(), "no such table: notifications") {
			return 0
		}
		t.Fatalf("querying notifications: %v", err)
	}
	return count
}

func TestRunRequiresDatabaseFlag(t *testing.T) {
	err := run(context.Background(), []string{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error missing -database")
	}
	if !strings.Contains(err.Error(), "-database is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDryRunReportsChangeWithoutWriting(t *testing.T) {
	dbPath := setupTestDB(t)
	profilePath := writeProfile(t)

	var buf bytes.Buffer
	args := []string{
		"-database", dbPath,
		"-profile", profilePath,
		"-migrations", "../../migrations",
	}

	err := run(context.Background(), args, &buf)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	lastLine := lines[len(lines)-1]

	var sum summary
	if err := json.Unmarshal([]byte(lastLine), &sum); err != nil {
		t.Fatalf("parsing summary json: %v, text was: %s", err, lastLine)
	}

	if sum.Changed < 1 {
		t.Errorf("expected at least 1 changed, got %d", sum.Changed)
	}
	if sum.Updated != 0 {
		t.Errorf("expected 0 updated on dry run, got %d", sum.Updated)
	}

	layer, _, _ := getOpportunityCurrent(t, dbPath)
	if layer != "incelenecek" {
		t.Errorf("expected layer to remain incelenecek, got %s", layer)
	}
}

func TestRunApplyUpdatesVisibility(t *testing.T) {
	dbPath := setupTestDB(t)
	profilePath := writeProfile(t)

	var buf bytes.Buffer
	args := []string{
		"-database", dbPath,
		"-profile", profilePath,
		"-migrations", "../../migrations",
		"-apply",
	}

	err := run(context.Background(), args, &buf)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	lastLine := lines[len(lines)-1]

	var sum summary
	if err := json.Unmarshal([]byte(lastLine), &sum); err != nil {
		t.Fatalf("parsing summary json: %v, text was: %s", err, lastLine)
	}

	if sum.Updated < 1 {
		t.Errorf("expected at least 1 updated, got %d", sum.Updated)
	}

	layer, reason, _ := getOpportunityCurrent(t, dbPath)
	if layer == "incelenecek" {
		t.Errorf("expected layer to change from incelenecek")
	}
	if reason != "foreign_non_remote_location" {
		t.Errorf("expected reason to be foreign_non_remote_location, got %s", reason)
	}
}

func TestRunLeavesNoNotifications(t *testing.T) {
	dbPath := setupTestDB(t)
	profilePath := writeProfile(t)

	var buf bytes.Buffer
	args := []string{
		"-database", dbPath,
		"-profile", profilePath,
		"-migrations", "../../migrations",
		"-apply",
	}

	err := run(context.Background(), args, &buf)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	count := getNotificationsCount(t, dbPath)
	if count != 0 {
		t.Errorf("expected 0 notifications, got %d", count)
	}
}
