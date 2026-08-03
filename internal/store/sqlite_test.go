package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
)

func TestSQLiteRepositoryDeduplicatesCanonicalURL(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)

	firstSeen := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	listing := domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Yazılım Stajyeri",
		URL:     "HTTPS://WWW.KARIYER.NET:443/is-ilani/staj-123/?utm_source=test&ref=profile#details",
		RawText: "İlk içerik", FetchedAt: firstSeen,
	}

	listingID, isNew, err := repository.UpsertRawListing(context.Background(), listing)
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}
	if !isNew || listingID == "" {
		t.Fatalf("expected a new listing, got id=%q new=%v", listingID, isNew)
	}

	listing.URL = "https://www.kariyer.net/is-ilani/staj-123?ref=profile&gclid=tracking"
	listing.RawText = "Güncellenmiş içerik"
	listing.FetchedAt = firstSeen.Add(time.Hour)
	secondID, isNew, err := repository.UpsertRawListing(context.Background(), listing)
	if err != nil {
		t.Fatalf("update listing: %v", err)
	}
	if isNew || secondID != listingID {
		t.Fatalf("expected duplicate id %q, got id=%q new=%v", listingID, secondID, isNew)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM listings").Scan(&count); err != nil {
		t.Fatalf("count listings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one listing, got %d", count)
	}

	var rawText, lastSeen string
	if err := db.QueryRow("SELECT raw_text, last_seen_at FROM listings WHERE id = ?", listingID).Scan(&rawText, &lastSeen); err != nil {
		t.Fatalf("read listing: %v", err)
	}
	if rawText != "Güncellenmiş içerik" || lastSeen != listing.FetchedAt.Format(time.RFC3339Nano) {
		t.Fatalf("listing was not refreshed: text=%q last_seen=%q", rawText, lastSeen)
	}
}

func TestSQLiteRepositoryRequiresRegisteredSource(t *testing.T) {
	repository, _ := newTestRepository(t)

	_, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "missing", Title: "Staj",
		URL: "https://example.test/is-ilani/1", RawText: "Staj",
	})
	if err == nil {
		t.Fatal("expected unregistered source to fail")
	}
}

func TestSQLiteRepositorySavesAnalysis(t *testing.T) {
	repository, db := newTestRepository(t)
	registerMeteksan(t, repository)
	listingID, _, err := repository.UpsertRawListing(context.Background(), domain.RawListing{
		Company: "Meteksan Savunma", SourceID: "meteksan-kariyer-net", Title: "Backend Stajyeri",
		URL: "https://example.test/is-ilani/1", RawText: "Backend ve Go",
	})
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}

	err = repository.SaveAnalysis(context.Background(), listingID, domain.ListingAnalysis{
		OpportunityType: "staj", ApplicationOpen: true, Relevant: true,
		MatchingAreas: []string{"backend"}, Eligibility: domain.EligibilitySuitable,
		Summary: "Backend odaklı staj ilanı", Confidence: 0.9,
	})
	if err != nil {
		t.Fatalf("save analysis: %v", err)
	}

	var status, provider string
	if err := db.QueryRow("SELECT processing_status, provider FROM listing_analyses WHERE listing_id = ?", listingID).Scan(&status, &provider); err != nil {
		t.Fatalf("read analysis: %v", err)
	}
	if status != "processed" || provider != "deterministic" {
		t.Fatalf("unexpected analysis state: status=%q provider=%q", status, provider)
	}
}

func TestCanonicalURL(t *testing.T) {
	canonical, err := CanonicalURL("HTTPS://Example.COM:443/jobs/42/?b=2&utm_medium=email&a=1#apply")
	if err != nil {
		t.Fatalf("canonicalize URL: %v", err)
	}
	if canonical != "https://example.com/jobs/42?a=1&b=2" {
		t.Fatalf("unexpected canonical URL %q", canonical)
	}
}

func newTestRepository(t *testing.T) (*SQLiteRepository, queryDB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "tracker.db"), os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	return repository, db
}

func registerMeteksan(t *testing.T, repository *SQLiteRepository) {
	t.Helper()
	err := repository.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: "meteksan-kariyer-net", Company: "Meteksan Savunma", PriorityGroup: "primary",
		Type: "career_page", URL: "https://www.kariyer.net/firma-profil/meteksan", Adapter: "kariyer_net", Enabled: true,
	})
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
}

type queryDB interface {
	QueryRow(query string, args ...any) *sql.Row
}
