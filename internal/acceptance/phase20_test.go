package acceptance_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

// rssFeedServer is a conditional-GET-aware fixture RSS server whose body and
// ETag can be swapped mid-test to simulate a feed publishing an update.
type rssFeedServer struct {
	mu       sync.Mutex
	etag     string
	body     string
	requests int
	notMod   int
}

func newRSSFeedServer() *rssFeedServer { return &rssFeedServer{} }

func (s *rssFeedServer) set(etag, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.etag, s.body = etag, body
}

func (s *rssFeedServer) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	if r.Header.Get("If-None-Match") == s.etag && s.etag != "" {
		s.notMod++
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", s.etag)
	w.Header().Set("Content-Type", "application/rss+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(s.body))
}

func rssFixtureV1(baseURL string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
<title>Acme Careers</title>
<item>
  <title>Backend Internship</title>
  <link>%[1]s/jobs/backend-intern</link>
  <guid>acme-backend-intern</guid>
  <description>Backend staj programina basvurun.</description>
</item>
<item>
  <title>Data Internship</title>
  <link>%[1]s/jobs/data-intern</link>
  <guid>acme-data-intern</guid>
  <description>Veri muhendisligi staj programi.</description>
</item>
</channel></rss>`, baseURL)
}

func rssFixtureV2Updated(baseURL string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
<title>Acme Careers</title>
<item>
  <title>Backend Internship</title>
  <link>%[1]s/jobs/backend-intern</link>
  <guid>acme-backend-intern</guid>
  <description>Backend staj programina basvurun.</description>
</item>
<item>
  <title>Data Internship</title>
  <link>%[1]s/jobs/data-intern</link>
  <guid>acme-data-intern</guid>
  <description>Veri muhendisligi staj programi - basvuru tarihi uzatildi.</description>
</item>
</channel></rss>`, baseURL)
}

// TestPhase20RSSFeedSurvivesRestartWithoutDuplicateNotifications exercises the
// Faz 20 exit criterion: a fixture RSS feed, polled across a simulated process
// restart (the sqlite file is closed and reopened between scans, exactly like
// an orchestrator restart), must not re-notify on unchanged items, must
// recognize the feed's own conditional-GET validators (ETag) persisted across
// that restart, and must still surface a genuinely updated item after restart.
func TestPhase20RSSFeedSurvivesRestartWithoutDuplicateNotifications(t *testing.T) {
	feed := newRSSFeedServer()
	server := httptest.NewServer(http.HandlerFunc(feed.handler))
	t.Cleanup(server.Close)
	feed.set(`"v1"`, rssFixtureV1(server.URL))

	dbPath := filepath.Join(t.TempDir(), "phase20.db")
	const sourceKey = "acme-careers-rss"

	provider := &fixtureProvider{}
	modelAnalyzer, err := analyzer.NewModelAnalyzer(provider, "fixture-strict-model", analyzer.CostRates{
		InputPerMillionUSD: 0.25, OutputPerMillionUSD: 1.50,
	})
	if err != nil {
		t.Fatalf("create fixture analyzer: %v", err)
	}
	profile := minimizedAcceptanceProfile()
	now := time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)

	// --- Scan 1: first-ever poll, both items are new. ---
	db1, repository1 := openRestartableRepository(t, dbPath)
	if err := repository1.RegisterSource(context.Background(), domain.SourceRegistration{
		Key: sourceKey, Company: "Acme", PriorityGroup: "candidate",
		Type: "rss_feed", URL: server.URL, Adapter: "rss_feed", Strategy: "rss_feed", Enabled: true,
	}); err != nil {
		t.Fatalf("register RSS source: %v", err)
	}
	source1, err := scraper.NewRSSFeedSource(sourceKey, "Acme", server.URL, repository1, server.Client())
	if err != nil {
		t.Fatalf("create RSS source: %v", err)
	}
	service1 := &orchestrator.Service{
		Sources: []scraper.Source{source1}, Analyzer: modelAnalyzer, Store: repository1, Profile: profile,
	}
	service1.Now = func() time.Time { return now }

	first, err := service1.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("first RSS scan: %v", err)
	}
	if first.Status != "completed" || first.Sources[0].Found != 2 || first.Sources[0].New != 2 || first.Sources[0].ProcessError != 0 {
		t.Fatalf("unexpected first RSS scan result: %#v", first.Sources)
	}
	if provider.calls != 2 {
		t.Fatalf("expected 2 analyses after first scan, got %d", provider.calls)
	}
	assertEvidenceSourceType(t, db1, "Acme", "rss", 2)
	_ = db1.Close()

	// --- Simulated restart: close and reopen the same sqlite file. ---
	now = now.Add(time.Hour)
	db2, repository2 := openRestartableRepository(t, dbPath)
	source2, err := scraper.NewRSSFeedSource(sourceKey, "Acme", server.URL, repository2, server.Client())
	if err != nil {
		t.Fatalf("recreate RSS source after restart: %v", err)
	}
	service2 := &orchestrator.Service{
		Sources: []scraper.Source{source2}, Analyzer: modelAnalyzer, Store: repository2, Profile: profile,
	}
	service2.Now = func() time.Time { return now }

	// --- Scan 2 (post-restart): feed unchanged; server must see the ETag
	// this process never issued itself and answer 304, proving the
	// checkpoint survived the restart via sqlite, not adapter memory. ---
	second, err := service2.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("second RSS scan (post-restart): %v", err)
	}
	if second.Sources[0].Found != 0 || second.Sources[0].New != 0 || second.Sources[0].ProcessError != 0 {
		t.Fatalf("unchanged post-restart rescan must find nothing: %#v", second.Sources)
	}
	if feed.notMod == 0 {
		t.Fatalf("server never received a matching conditional GET after restart; checkpoint was not restored")
	}
	if provider.calls != 2 {
		t.Fatalf("unchanged rescan must not trigger re-analysis (no duplicate notification), calls=%d", provider.calls)
	}

	// --- The feed publishes an update to one item. ---
	now = now.Add(time.Hour)
	feed.set(`"v2"`, rssFixtureV2Updated(server.URL))

	third, err := service2.Run(context.Background(), "manual")
	if err != nil {
		t.Fatalf("third RSS scan (post-update): %v", err)
	}
	if third.Sources[0].Found != 1 || third.Sources[0].New != 0 || third.Sources[0].ProcessError != 0 {
		t.Fatalf("updated item must be found and its listing content refreshed, but not counted as new: %#v", third.Sources)
	}
	// The existing pipeline analyzes a listing exactly once regardless of
	// adapter (AnalysisRequired only checks whether processing ever
	// completed, not content_hash) — this holds for every adapter, not a
	// Faz 20 regression. What Faz 20 guarantees is that the updated raw
	// content itself reaches the common model, which the raw_text
	// assertion below verifies directly.
	if provider.calls != 2 {
		t.Fatalf("update must not trigger a spurious re-analysis, calls=%d", provider.calls)
	}

	var updatedRawText string
	if err := db2.QueryRow(`
		SELECT listings.raw_text FROM listings
		JOIN companies ON companies.id = listings.company_id
		WHERE companies.name = 'Acme' AND listings.title = 'Data Internship'
	`).Scan(&updatedRawText); err != nil {
		t.Fatalf("read updated listing raw text: %v", err)
	}
	if !strings.Contains(updatedRawText, "basvuru tarihi uzatildi") {
		t.Fatalf("updated feed content did not reach the normalized listing model: %q", updatedRawText)
	}

	dashboard, err := repository2.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("load dashboard: %v", err)
	}
	found := false
	for _, listing := range append(append([]store.DashboardListing(nil), dashboard.NewListings...), dashboard.NeedsDecision...) {
		if listing.Title == "Data Internship" {
			found = true
		}
	}
	if !found {
		t.Fatalf("updated RSS listing not visible in dashboard: %#v", dashboard)
	}
	assertEvidenceSourceType(t, db2, "Acme", "rss", 2)
}

// openRestartableRepository opens (or reopens) a durable sqlite-backed
// repository at a fixed path, unlike openRepository's throwaway temp file, so
// a test can close it and reopen it to simulate a real process restart.
func openRestartableRepository(t *testing.T, path string) (*sql.DB, *store.SQLiteRepository) {
	t.Helper()
	db, err := database.Open(context.Background(), path, os.DirFS("../../migrations"))
	if err != nil {
		t.Fatalf("open restartable acceptance database: %v", err)
	}
	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("create restartable acceptance repository: %v", err)
	}
	return db, repository
}

func assertEvidenceSourceType(t *testing.T, db *sql.DB, company, wantSourceType string, wantCount int) {
	t.Helper()
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM opportunity_evidence
		JOIN listings ON listings.id = opportunity_evidence.listing_id
		JOIN companies ON companies.id = listings.company_id
		WHERE companies.name = ? AND opportunity_evidence.source_type = ?
	`, company, wantSourceType).Scan(&count)
	if err != nil {
		t.Fatalf("query opportunity_evidence source_type: %v", err)
	}
	if count != wantCount {
		t.Fatalf("expected %d opportunity_evidence rows with source_type=%q for %q, got %d", wantCount, wantSourceType, company, count)
	}
}
