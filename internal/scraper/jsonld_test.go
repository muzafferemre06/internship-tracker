package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestJSONLDSourceParsesJobPostings(t *testing.T) {
	fixture, err := os.ReadFile("testdata/jsonld/career-jobposting.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	const pageURL = "https://careers.northstar.example/"
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != pageURL {
			t.Fatalf("unexpected request URL: %s", request.URL)
		}
		if !strings.Contains(request.Header.Get("User-Agent"), "personal career monitoring") {
			t.Fatal("expected descriptive user agent")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := NewJSONLDSource("northstar-careers", "Northstar Robotics", pageURL, client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	source.now = func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("fetch listings: %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("expected two listings, got %d", len(listings))
	}
	first := listings[0]
	if first.Title != "Software Engineering Intern" || first.Company != "Northstar Robotics" || first.SourceID != "northstar-careers" {
		t.Fatalf("unexpected first listing: %#v", first)
	}
	if first.URL != "https://careers.northstar.example/jobs/se-intern-2026" {
		t.Fatalf("URL was not canonicalized (query/fragment stripped): %q", first.URL)
	}
	if !first.FetchedAt.Equal(time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected FetchedAt: %v", first.FetchedAt)
	}
	// RawText must carry normalized structured fields and stripped description text, no HTML tags.
	for _, want := range []string{"Software Engineering Intern", "INTERN", "Ankara", "2026-09-15", "distributed systems", "3rd/4th year"} {
		if !strings.Contains(first.RawText, want) {
			t.Fatalf("RawText missing %q: %q", want, first.RawText)
		}
	}
	if strings.Contains(first.RawText, "<") || strings.Contains(first.RawText, "&lt;") {
		t.Fatalf("RawText still contains markup: %q", first.RawText)
	}
	if listings[1].Title != "Data Engineering Intern" {
		t.Fatalf("unexpected second listing: %#v", listings[1])
	}
}

func TestJSONLDSourceErrorsWhenNoJobPosting(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "no json-ld at all", body: `<html><head><title>x</title></head><body>no data</body></html>`},
		{name: "json-ld without JobPosting", body: `<html><head><script type="application/ld+json">{"@type":"WebSite","name":"x"}</script></head></html>`},
		{name: "JobPosting missing title", body: `<html><head><script type="application/ld+json">{"@type":"JobPosting","url":"https://x.example/j/1"}</script></head></html>`},
		{name: "malformed json-ld", body: `<html><head><script type="application/ld+json">{ not json </script></head></html>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			source, err := NewJSONLDSource("test", "Company", "https://careers.example/", client)
			if err != nil {
				t.Fatalf("create source: %v", err)
			}
			if _, err := source.FetchListings(context.Background()); err == nil {
				t.Fatal("expected error for structural failure, got nil (silent zero not allowed)")
			}
		})
	}
}

func TestJSONLDSourceRejectsInvalidURL(t *testing.T) {
	if _, err := NewJSONLDSource("test", "Company", "ftp://careers.example/", nil); err == nil {
		t.Fatal("expected error for non-HTTP(S) URL")
	}
	if _, err := NewJSONLDSource("", "Company", "https://careers.example/", nil); err == nil {
		t.Fatal("expected error for empty source name")
	}
}

func TestJSONLDSourceReportsAccessProtection(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("just a moment..."))}, nil
	})}
	source, err := NewJSONLDSource("test", "Company", "https://careers.example/", client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	_, err = source.FetchListings(context.Background())
	var accessErr *AccessError
	if !errors.As(err, &accessErr) || !accessErr.Protective() {
		t.Fatalf("expected protective access error, got %v", err)
	}
}
