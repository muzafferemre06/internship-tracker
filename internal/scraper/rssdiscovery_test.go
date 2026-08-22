package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type dummyCheckpointStore struct{}

func (d *dummyCheckpointStore) LoadFeedCheckpoint(ctx context.Context, sourceKey string) (domain.FeedCheckpoint, bool, error) {
	return domain.FeedCheckpoint{}, false, nil
}

func (d *dummyCheckpointStore) SaveFeedCheckpoint(ctx context.Context, checkpoint domain.FeedCheckpoint) error {
	return nil
}

func (d *dummyCheckpointStore) LoadSeenFeedItem(ctx context.Context, sourceKey, itemKey string) (string, bool, error) {
	return "", false, nil
}

func (d *dummyCheckpointStore) MarkSeenFeedItem(ctx context.Context, sourceKey, itemKey, contentHash string, seenAt time.Time) error {
	return nil
}

func TestDiscoverFeedLinks(t *testing.T) {
	tests := []struct {
		name       string
		htmlBody   string
		statusCode int
		invalidURL bool
		wantLinks  []string
		wantErr    bool
	}{
		{
			name: "typed RSS relative link",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <title>Test Blog</title>
    <link rel="alternate" type="application/rss+xml" href="/feed.xml">
</head>
<body><h1>Hello</h1></body>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"/feed.xml"},
			wantErr:    false,
		},
		{
			name: "typed Atom link with space separated rel attributes",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <link rel="alternate stylesheet" type="text/css" href="/style.css">
    <link rel="feed alternate" type="application/atom+xml" href="/atom.xml">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"/atom.xml"},
			wantErr:    false,
		},
		{
			name: "x-atom and x.atom mime types",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <link rel="alternate" type="application/x.atom+xml" href="/xdotatom.xml">
    <link rel="alternate" type="application/x-atom+xml" href="/xdashatom.xml">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"/xdotatom.xml", "/xdashatom.xml"},
			wantErr:    false,
		},
		{
			name: "mixed hosts: filters out cross-domain link and keeps same-host link",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <link rel="alternate" type="application/rss+xml" href="/internal-feed.xml">
    <link rel="alternate" type="application/rss+xml" href="https://evil.example/feed.xml">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"/internal-feed.xml"},
			wantErr:    false,
		},
		{
			name: "multiple same-domain links de-duplicated preserving order",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <link rel="alternate" type="application/rss+xml" href="/feed1.xml">
    <link rel="alternate" type="application/rss+xml" href="/feed2.xml">
    <link rel="alternate" type="application/rss+xml" href="/feed1.xml">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"/feed1.xml", "/feed2.xml"},
			wantErr:    false,
		},
		{
			name: "no matching link tags returns empty slice and nil error",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <title>No Feed</title>
    <link rel="stylesheet" href="/style.css">
    <link rel="alternate" type="text/html" href="/alternate-page.html">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{},
			wantErr:    false,
		},
		{
			name: "fallback untyped link with .rss or .atom or .xml extension",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <link rel="alternate" href="/news.atom">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"/news.atom"},
			wantErr:    false,
		},
		{
			name: "prefers typed matches over fallback untyped matches",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <link rel="alternate" type="application/rss+xml" href="/typed.xml">
    <link rel="alternate" href="/untyped.xml">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"/typed.xml"},
			wantErr:    false,
		},
		{
			name: "discards non-http schemes",
			htmlBody: `<!DOCTYPE html>
<html>
<head>
    <link rel="alternate" type="application/rss+xml" href="javascript:alert(1)">
    <link rel="alternate" type="application/rss+xml" href="ftp://localhost/feed.xml">
</head>
</html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{},
			wantErr:    false,
		},
		{
			name:       "non-200 HTTP status returns error",
			htmlBody:   "Not Found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "invalid pageURL returns error",
			invalidURL: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.invalidURL {
				_, err := DiscoverFeedLinks(context.Background(), "invalid-url", nil)
				if err == nil {
					t.Fatalf("expected error for invalid URL, got nil")
				}
				return
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") != defaultDiscoveryUA {
					t.Errorf("expected User-Agent %q, got %q", defaultDiscoveryUA, r.Header.Get("User-Agent"))
				}
				if r.Header.Get("Accept") != "text/html,application/xhtml+xml" {
					t.Errorf("expected Accept header %q, got %q", "text/html,application/xhtml+xml", r.Header.Get("Accept"))
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.htmlBody))
			}))
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			got, err := DiscoverFeedLinks(ctx, server.URL, server.Client())
			if (err != nil) != tt.wantErr {
				t.Fatalf("DiscoverFeedLinks() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			var expected []string
			for _, w := range tt.wantLinks {
				expected = append(expected, server.URL+w)
			}
			if len(expected) == 0 {
				expected = []string{}
			}

			if !reflect.DeepEqual(got, expected) {
				t.Errorf("DiscoverFeedLinks() = %v, want %v", got, expected)
			}
		})
	}
}

func TestNewRSSFeedSourceFromPage(t *testing.T) {
	t.Run("succeeds when feed link is discovered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><head><link rel="alternate" type="application/rss+xml" href="/jobs.rss"></head></html>`))
		}))
		defer server.Close()

		ctx := context.Background()
		store := &dummyCheckpointStore{}
		source, err := NewRSSFeedSourceFromPage(ctx, "TestFeed", "TestCorp", server.URL, store, server.Client())
		if err != nil {
			t.Fatalf("NewRSSFeedSourceFromPage() unexpected error: %v", err)
		}
		if source == nil {
			t.Fatal("NewRSSFeedSourceFromPage() returned nil source")
		}
		if source.Name() != "TestFeed" {
			t.Errorf("source.Name() = %q, want %q", source.Name(), "TestFeed")
		}
	})

	t.Run("fails when no feed link is discovered", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><head><title>No feeds</title></head></html>`))
		}))
		defer server.Close()

		ctx := context.Background()
		store := &dummyCheckpointStore{}
		source, err := NewRSSFeedSourceFromPage(ctx, "TestFeed", "TestCorp", server.URL, store, server.Client())
		if err == nil {
			t.Fatal("expected error when no feed discovered, got nil")
		}
		if source != nil {
			t.Errorf("expected nil source, got %v", source)
		}
	})
}
