package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type mockFeedCheckpointStore struct {
	mu sync.Mutex

	checkpoints map[string]domain.FeedCheckpoint
	seenItems   map[string]map[string]string

	saveCheckpointCalls []domain.FeedCheckpoint
	markSeenItemCalls   []markSeenCall
}

type markSeenCall struct {
	SourceKey   string
	ItemKey     string
	ContentHash string
	SeenAt      time.Time
}

func newMockFeedCheckpointStore() *mockFeedCheckpointStore {
	return &mockFeedCheckpointStore{
		checkpoints: make(map[string]domain.FeedCheckpoint),
		seenItems:   make(map[string]map[string]string),
	}
}

func (m *mockFeedCheckpointStore) LoadFeedCheckpoint(ctx context.Context, sourceKey string) (domain.FeedCheckpoint, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp, ok := m.checkpoints[sourceKey]
	return cp, ok, nil
}

func (m *mockFeedCheckpointStore) SaveFeedCheckpoint(ctx context.Context, checkpoint domain.FeedCheckpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCheckpointCalls = append(m.saveCheckpointCalls, checkpoint)
	m.checkpoints[checkpoint.SourceKey] = checkpoint
	return nil
}

func (m *mockFeedCheckpointStore) LoadSeenFeedItem(ctx context.Context, sourceKey, itemKey string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sourceItems, ok := m.seenItems[sourceKey]
	if !ok {
		return "", false, nil
	}
	hash, ok := sourceItems[itemKey]
	return hash, ok, nil
}

func (m *mockFeedCheckpointStore) MarkSeenFeedItem(ctx context.Context, sourceKey, itemKey, contentHash string, seenAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markSeenItemCalls = append(m.markSeenItemCalls, markSeenCall{
		SourceKey:   sourceKey,
		ItemKey:     itemKey,
		ContentHash: contentHash,
		SeenAt:      seenAt,
	})
	if _, ok := m.seenItems[sourceKey]; !ok {
		m.seenItems[sourceKey] = make(map[string]string)
	}
	m.seenItems[sourceKey][itemKey] = contentHash
	return nil
}

func TestNewRSSFeedSource_Validation(t *testing.T) {
	mockStore := newMockFeedCheckpointStore()

	tests := []struct {
		name        string
		sourceName  string
		company     string
		feedURL     string
		checkpoints FeedCheckpointStore
		wantErr     bool
	}{
		{
			name:        "valid parameters",
			sourceName:  "acme-careers",
			company:     "Acme Corp",
			feedURL:     "https://careers.acme.com/feed.xml",
			checkpoints: mockStore,
			wantErr:     false,
		},
		{
			name:        "empty name",
			sourceName:  "  ",
			company:     "Acme Corp",
			feedURL:     "https://careers.acme.com/feed.xml",
			checkpoints: mockStore,
			wantErr:     true,
		},
		{
			name:        "empty company",
			sourceName:  "acme-careers",
			company:     "",
			feedURL:     "https://careers.acme.com/feed.xml",
			checkpoints: mockStore,
			wantErr:     true,
		},
		{
			name:        "nil checkpoint store",
			sourceName:  "acme-careers",
			company:     "Acme Corp",
			feedURL:     "https://careers.acme.com/feed.xml",
			checkpoints: nil,
			wantErr:     true,
		},
		{
			name:        "relative URL",
			sourceName:  "acme-careers",
			company:     "Acme Corp",
			feedURL:     "/feed.xml",
			checkpoints: mockStore,
			wantErr:     true,
		},
		{
			name:        "invalid scheme",
			sourceName:  "acme-careers",
			company:     "Acme Corp",
			feedURL:     "ftp://careers.acme.com/feed.xml",
			checkpoints: mockStore,
			wantErr:     true,
		},
		{
			name:        "malformed URL",
			sourceName:  "acme-careers",
			company:     "Acme Corp",
			feedURL:     "://invalid-url",
			checkpoints: mockStore,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := NewRSSFeedSource(tt.sourceName, tt.company, tt.feedURL, tt.checkpoints, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewRSSFeedSource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && src == nil {
				t.Fatal("expected non-nil RSSFeedSource on success")
			}
		})
	}
}

func TestRSSFeedSource_AccessPolicy(t *testing.T) {
	mockStore := newMockFeedCheckpointStore()
	src, err := NewRSSFeedSource("test-feed", "Acme", "https://jobs.example.com/rss", mockStore, nil)
	if err != nil {
		t.Fatalf("unexpected error creating source: %v", err)
	}

	policy := src.AccessPolicy()
	if policy.Scope != "jobs.example.com" {
		t.Errorf("AccessPolicy.Scope = %q, want %q", policy.Scope, "jobs.example.com")
	}
	if policy.MinimumInterval != time.Second {
		t.Errorf("AccessPolicy.MinimumInterval = %v, want %v", policy.MinimumInterval, time.Second)
	}
	if policy.BaseCooldown != time.Minute {
		t.Errorf("AccessPolicy.BaseCooldown = %v, want %v", policy.BaseCooldown, time.Minute)
	}
	if policy.MaximumCooldown != time.Hour {
		t.Errorf("AccessPolicy.MaximumCooldown = %v, want %v", policy.MaximumCooldown, time.Hour)
	}
}

func TestRSSFeedSource_FetchListings_RSS20(t *testing.T) {
	rssBody := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>Acme Careers</title>
    <link>https://acme.com/careers</link>
    <item>
      <title>Software Engineering Intern</title>
      <link>https://acme.com/jobs/swe-intern</link>
      <description>&lt;p&gt;Build scalable systems &amp;amp; tools.&lt;/p&gt;</description>
      <guid>acme-swe-1</guid>
    </item>
    <item>
      <title>Data Science Intern</title>
      <link>https://acme.com/jobs/ds-intern</link>
      <description>Fallback summary</description>
      <content:encoded><![CDATA[<div>Work with <b>ML models</b> & algorithms.</div>]]></content:encoded>
      <guid>acme-ds-2</guid>
    </item>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "internship-tracker/0.1 (+personal career monitoring)" {
			t.Errorf("unexpected User-Agent: %s", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("Accept") != "application/rss+xml, application/atom+xml, application/xml, text/xml" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}

		w.Header().Set("ETag", `"etag-rss-123"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2025 07:28:00 GMT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rssBody))
	}))
	defer server.Close()

	mockStore := newMockFeedCheckpointStore()
	src, err := NewRSSFeedSource("acme-rss", "Acme Corp", server.URL, mockStore, server.Client())
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	listings, err := src.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("FetchListings failed: %v", err)
	}

	if len(listings) != 2 {
		t.Fatalf("expected 2 listings, got %d", len(listings))
	}

	if listings[0].Title != "Software Engineering Intern" {
		t.Errorf("listing[0].Title = %q, want %q", listings[0].Title, "Software Engineering Intern")
	}
	if listings[0].URL != "https://acme.com/jobs/swe-intern" {
		t.Errorf("listing[0].URL = %q, want %q", listings[0].URL, "https://acme.com/jobs/swe-intern")
	}
	if listings[0].RawText != "Build scalable systems & tools." {
		t.Errorf("listing[0].RawText = %q, want %q", listings[0].RawText, "Build scalable systems & tools.")
	}
	if listings[0].Company != "Acme Corp" || listings[0].SourceID != "acme-rss" {
		t.Errorf("listing[0] metadata mismatch: company=%s sourceID=%s", listings[0].Company, listings[0].SourceID)
	}

	if listings[1].Title != "Data Science Intern" {
		t.Errorf("listing[1].Title = %q, want %q", listings[1].Title, "Data Science Intern")
	}
	if listings[1].RawText != "Work with ML models & algorithms." {
		t.Errorf("listing[1].RawText = %q, want %q", listings[1].RawText, "Work with ML models & algorithms.")
	}

	if len(mockStore.saveCheckpointCalls) != 1 {
		t.Fatalf("expected 1 SaveFeedCheckpoint call, got %d", len(mockStore.saveCheckpointCalls))
	}
	cp := mockStore.saveCheckpointCalls[0]
	if cp.ETag != `"etag-rss-123"` || cp.LastModified != "Wed, 21 Oct 2025 07:28:00 GMT" {
		t.Errorf("saved checkpoint mismatch: %+v", cp)
	}

	if len(mockStore.markSeenItemCalls) != 2 {
		t.Errorf("expected 2 MarkSeenFeedItem calls, got %d", len(mockStore.markSeenItemCalls))
	}
}

func TestRSSFeedSource_FetchListings_Atom(t *testing.T) {
	atomBody := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Beta Corp Careers</title>
  <entry>
    <title>Product Management Intern</title>
    <link rel="alternate" href="https://beta.com/jobs/pm-intern"/>
    <id>urn:job:pm-001</id>
    <content type="html">&lt;p&gt;Drive product roadmap &amp;amp; specs.&lt;/p&gt;</content>
  </entry>
  <entry>
    <title>Frontend Engineering Intern</title>
    <link href="https://beta.com/jobs/fe-intern"/>
    <id>urn:job:fe-002</id>
    <summary>Build React and TypeScript web apps.</summary>
  </entry>
</feed>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"etag-atom-456"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(atomBody))
	}))
	defer server.Close()

	mockStore := newMockFeedCheckpointStore()
	src, err := NewRSSFeedSource("beta-atom", "Beta Corp", server.URL, mockStore, server.Client())
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	listings, err := src.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("FetchListings failed: %v", err)
	}

	if len(listings) != 2 {
		t.Fatalf("expected 2 listings, got %d", len(listings))
	}

	if listings[0].Title != "Product Management Intern" || listings[0].URL != "https://beta.com/jobs/pm-intern" {
		t.Errorf("unexpected listing[0]: %+v", listings[0])
	}
	if listings[0].RawText != "Drive product roadmap & specs." {
		t.Errorf("listing[0].RawText = %q, want %q", listings[0].RawText, "Drive product roadmap & specs.")
	}

	if listings[1].Title != "Frontend Engineering Intern" || listings[1].URL != "https://beta.com/jobs/fe-intern" {
		t.Errorf("unexpected listing[1]: %+v", listings[1])
	}
	if listings[1].RawText != "Build React and TypeScript web apps." {
		t.Errorf("listing[1].RawText = %q, want %q", listings[1].RawText, "Build React and TypeScript web apps.")
	}
}

func TestRSSFeedSource_FetchListings_304NotModified(t *testing.T) {
	etag := `"cached-etag-789"`
	lastMod := "Sun, 10 Nov 2025 12:00:00 GMT"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != etag {
			t.Errorf("If-None-Match = %q, want %q", r.Header.Get("If-None-Match"), etag)
		}
		if r.Header.Get("If-Modified-Since") != lastMod {
			t.Errorf("If-Modified-Since = %q, want %q", r.Header.Get("If-Modified-Since"), lastMod)
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	mockStore := newMockFeedCheckpointStore()
	mockStore.checkpoints["cached-source"] = domain.FeedCheckpoint{
		SourceKey:    "cached-source",
		ETag:         etag,
		LastModified: lastMod,
	}

	src, err := NewRSSFeedSource("cached-source", "Acme", server.URL, mockStore, server.Client())
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	listings, err := src.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("FetchListings failed: %v", err)
	}

	if len(listings) != 0 {
		t.Errorf("expected 0 listings on 304, got %d", len(listings))
	}
	if listings != nil {
		t.Errorf("expected nil slice on 304, got %v", listings)
	}
}

func TestRSSFeedSource_FetchListings_Deduplication(t *testing.T) {
	feedXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Jobs</title>
    <link>https://example.com</link>
    <item>
      <title>QA Intern</title>
      <link>https://example.com/jobs/qa</link>
      <description>Automation testing role</description>
      <guid>job-qa-1</guid>
    </item>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(feedXML))
	}))
	defer server.Close()

	mockStore := newMockFeedCheckpointStore()
	src, err := NewRSSFeedSource("qa-source", "Example", server.URL, mockStore, server.Client())
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// First poll returns 1 listing
	firstListings, err := src.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("first poll failed: %v", err)
	}
	if len(firstListings) != 1 {
		t.Fatalf("expected 1 listing on first poll, got %d", len(firstListings))
	}
	if len(mockStore.markSeenItemCalls) != 1 {
		t.Fatalf("expected 1 MarkSeen call, got %d", len(mockStore.markSeenItemCalls))
	}

	// Second poll with identical body and no 304 returns 0 listings
	secondListings, err := src.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("second poll failed: %v", err)
	}
	if len(secondListings) != 0 {
		t.Fatalf("expected 0 listings on repeat poll, got %d", len(secondListings))
	}
	if len(mockStore.markSeenItemCalls) != 1 {
		t.Errorf("MarkSeenFeedItem should not have been called on duplicate poll, total calls: %d", len(mockStore.markSeenItemCalls))
	}
}

func TestRSSFeedSource_FetchListings_ItemUpdated(t *testing.T) {
	var currentXML string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(currentXML))
	}))
	defer server.Close()

	mockStore := newMockFeedCheckpointStore()
	src, err := NewRSSFeedSource("update-source", "Example", server.URL, mockStore, server.Client())
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	currentXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Jobs</title>
    <link>https://example.com</link>
    <item>
      <title>SWE Intern</title>
      <link>https://example.com/jobs/swe</link>
      <description>Original description</description>
      <guid>job-swe</guid>
    </item>
    <item>
      <title>DevOps Intern</title>
      <link>https://example.com/jobs/devops</link>
      <description>Unchanged description</description>
      <guid>job-devops</guid>
    </item>
  </channel>
</rss>`

	firstListings, err := src.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("first poll failed: %v", err)
	}
	if len(firstListings) != 2 {
		t.Fatalf("expected 2 listings on first poll, got %d", len(firstListings))
	}

	// Update SWE description only
	currentXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Jobs</title>
    <link>https://example.com</link>
    <item>
      <title>SWE Intern</title>
      <link>https://example.com/jobs/swe</link>
      <description>Updated description with new requirements</description>
      <guid>job-swe</guid>
    </item>
    <item>
      <title>DevOps Intern</title>
      <link>https://example.com/jobs/devops</link>
      <description>Unchanged description</description>
      <guid>job-devops</guid>
    </item>
  </channel>
</rss>`

	secondListings, err := src.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("second poll failed: %v", err)
	}
	if len(secondListings) != 1 {
		t.Fatalf("expected exactly 1 updated listing, got %d", len(secondListings))
	}
	if secondListings[0].Title != "SWE Intern" || secondListings[0].RawText != "Updated description with new requirements" {
		t.Errorf("unexpected updated listing: %+v", secondListings[0])
	}
}

func TestRSSFeedSource_FetchListings_MalformedXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"etag-bad"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<rss><channel><broken`))
	}))
	defer server.Close()

	mockStore := newMockFeedCheckpointStore()
	src, err := NewRSSFeedSource("bad-source", "Acme", server.URL, mockStore, server.Client())
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	listings, err := src.FetchListings(context.Background())
	if err == nil {
		t.Fatal("expected error on malformed XML, got nil")
	}
	if listings != nil {
		t.Errorf("expected nil listings on error, got %v", listings)
	}

	if len(mockStore.saveCheckpointCalls) != 0 {
		t.Errorf("SaveFeedCheckpoint should NOT be called on XML parse failure")
	}
	if len(mockStore.markSeenItemCalls) != 0 {
		t.Errorf("MarkSeenFeedItem should NOT be called on XML parse failure")
	}
}

func TestRSSFeedSource_FetchListings_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	mockStore := newMockFeedCheckpointStore()
	src, err := NewRSSFeedSource("error-source", "Acme", server.URL, mockStore, server.Client())
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	listings, err := src.FetchListings(context.Background())
	if err == nil {
		t.Fatal("expected error on 500 status code, got nil")
	}
	if listings != nil {
		t.Errorf("expected nil listings on error, got %v", listings)
	}
}