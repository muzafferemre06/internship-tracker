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

func TestLeverBoardSourceFetchesCanonicalCompanyPostings(t *testing.T) {
	fixture, err := os.ReadFile("testdata/lever/mobileaction-board.html")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://jobs.lever.co/mobile-action" {
			t.Fatalf("unexpected request URL %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := NewLeverBoardSource("mobileaction-lever-board", "MobileAction", "https://jobs.lever.co/mobile-action/?lever-source=careers#jobs", client)
	if err != nil {
		t.Fatal(err)
	}
	policy := source.AccessPolicy()
	if policy.Scope != "jobs.lever.co" || policy.MinimumInterval != time.Second {
		t.Fatalf("unexpected Lever board access policy: %#v", policy)
	}

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 2 {
		t.Fatalf("listing count=%d, want 2: %#v", len(listings), listings)
	}
	if listings[0].Company != "MobileAction" || listings[0].SourceID != "mobileaction-lever-board" ||
		listings[0].Title != "Software Engineer" ||
		listings[0].URL != "https://jobs.lever.co/mobile-action/11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected first listing: %#v", listings[0])
	}
	if !strings.Contains(listings[0].RawText, "Ankara") || !strings.Contains(listings[1].RawText, "Java") {
		t.Fatalf("posting context missing from raw text: %#v", listings)
	}
}

func TestLeverBoardSourceAcceptsAnEmptyRecognizableBoard(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`<div class="postings-wrapper"></div>`))}, nil
	})}
	source, err := NewLeverBoardSource("board", "Company", "https://jobs.lever.co/company", client)
	if err != nil {
		t.Fatal(err)
	}
	listings, err := source.FetchListings(context.Background())
	if err != nil || len(listings) != 0 {
		t.Fatalf("empty board: listings=%#v err=%v", listings, err)
	}
}

func TestLeverBoardSourceRejectsInvalidURLAndUnexpectedPage(t *testing.T) {
	invalid := []string{
		"http://jobs.lever.co/company",
		"https://lever.co/company",
		"https://jobs.lever.co/",
		"https://jobs.lever.co/company/posting",
	}
	for _, rawURL := range invalid {
		if _, err := NewLeverBoardSource("board", "Company", rawURL, nil); err == nil {
			t.Errorf("expected invalid board URL %q to fail", rawURL)
		}
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`<html><body>not Lever</body></html>`))}, nil
	})}
	source, err := NewLeverBoardSource("board", "Company", "https://jobs.lever.co/company", client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.FetchListings(context.Background()); !errors.Is(err, ErrUnexpectedPage) {
		t.Fatalf("expected ErrUnexpectedPage, got %v", err)
	}
}
