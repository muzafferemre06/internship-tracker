package scraper

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLeverSourceParsesActivePosting(t *testing.T) {
	fixture, err := os.ReadFile("testdata/lever/commencis-posting.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://jobs.lever.co/commencis/04a5cd98-ab26-4b48-bb64-3397ffe79a55" {
			t.Fatalf("unexpected request URL: %s", request.URL)
		}
		if !strings.Contains(request.Header.Get("User-Agent"), "personal career monitoring") {
			t.Fatal("expected descriptive user agent")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := NewLeverSource("commencis-lever-camp", "Commencis", "https://jobs.lever.co/commencis/04a5cd98-ab26-4b48-bb64-3397ffe79a55?lever-source=test#fragment", client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	source.now = func() time.Time { return time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC) }
	policy := source.AccessPolicy()
	if policy.Scope != "jobs.lever.co" || policy.MinimumInterval != time.Second {
		t.Fatalf("unexpected Lever access policy: %#v", policy)
	}

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("fetch listings: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected one listing, got %d", len(listings))
	}
	listing := listings[0]
	if listing.Title != "Spring Boot Development Camp 2026" || listing.Company != "Commencis" {
		t.Fatalf("unexpected listing: %#v", listing)
	}
	if strings.Contains(listing.URL, "lever-source") || !strings.Contains(listing.RawText, "Intern") ||
		!strings.Contains(listing.RawText, "August 14, 2026") || !strings.Contains(listing.RawText, "Spring Boot") {
		t.Fatalf("listing was not normalized safely: %#v", listing)
	}
}

func TestLeverSourceRejectsClosedOrUnexpectedPosting(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing posting page", body: `<html><h2>Not found</h2></html>`},
		{name: "missing apply link", body: `<div class="posting-page"><div class="posting-headline"><h2>Intern</h2></div></div>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			source, err := NewLeverSource("test", "Company", "https://jobs.lever.co/company/posting", client)
			if err != nil {
				t.Fatalf("create source: %v", err)
			}
			if _, err := source.FetchListings(context.Background()); err == nil {
				t.Fatal("expected unexpected page error")
			}
		})
	}
}

func TestLeverSourceValidatesOfficialPostingURL(t *testing.T) {
	for _, rawURL := range []string{
		"http://jobs.lever.co/company/posting",
		"https://example.com/company/posting",
		"https://jobs.lever.co/company",
		"https://jobs.lever.co/company/posting/apply",
	} {
		if _, err := NewLeverSource("test", "Company", rawURL, nil); err == nil {
			t.Fatalf("expected URL %q to be rejected", rawURL)
		}
	}
}
