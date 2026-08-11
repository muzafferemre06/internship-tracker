package scraper

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestCareerLinksSourceExtractsScopedUniqueListings(t *testing.T) {
	fixture, err := os.ReadFile("testdata/careerlinks/secondary-careers.html")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != "https://careers.example.test/career/" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(fixture)))}, nil
	})}
	source, err := NewCareerLinksSource(
		"secondary-careers", "Secondary Co", "https://careers.example.test/career/",
		"open-positions", "/career/", client,
	)
	if err != nil {
		t.Fatal(err)
	}
	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 2 {
		t.Fatalf("expected two unique scoped listings, got %#v", listings)
	}
	if listings[0].Title != "Backend Intern" || listings[0].URL != "https://careers.example.test/career/backend-intern/" {
		t.Fatalf("unexpected first listing: %#v", listings[0])
	}
	if !strings.Contains(listings[0].RawText, "Go, API geliştirme") {
		t.Fatalf("card description missing from raw text: %q", listings[0].RawText)
	}
	if listings[1].Title != "Operations Intern" || listings[1].URL != "https://careers.example.test/career/operations-intern/" {
		t.Fatalf("query/fragment normalization or dedup failed: %#v", listings[1])
	}
}

func TestCareerLinksSourceRejectsUnexpectedPage(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`<main id="open-positions"><a href="/about">About</a></main>`))}, nil
	})}
	source, err := NewCareerLinksSource("secondary-careers", "Secondary Co", "https://careers.example.test/career/", "open-positions", "/career/", client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.FetchListings(context.Background()); err == nil || !strings.Contains(err.Error(), ErrUnexpectedPage.Error()) {
		t.Fatalf("expected an unexpected-page error, got %v", err)
	}
}
