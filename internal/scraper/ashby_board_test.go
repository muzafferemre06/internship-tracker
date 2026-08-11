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

func TestAshbyBoardSourceFetchesListedCanonicalPostings(t *testing.T) {
	fixture, err := os.ReadFile("testdata/ashby/binalyze-board.json")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://api.ashbyhq.com/posting-api/job-board/binalyze" {
			t.Fatalf("unexpected request URL %s", request.URL)
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Fatalf("unexpected Accept header %q", request.Header.Get("Accept"))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := NewAshbyBoardSource("binalyze-ashby-board", "Binalyze", "https://api.ashbyhq.com/posting-api/job-board/binalyze", client)
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) }
	policy := source.AccessPolicy()
	if policy.Scope != "api.ashbyhq.com" || policy.MinimumInterval != time.Second {
		t.Fatalf("unexpected Ashby access policy: %#v", policy)
	}

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 2 {
		t.Fatalf("listing count=%d, want 2: %#v", len(listings), listings)
	}
	first := listings[0]
	if first.Company != "Binalyze" || first.SourceID != "binalyze-ashby-board" || first.Title != "Detection Engineer" ||
		first.URL != "https://jobs.ashbyhq.com/binalyze/11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected first listing: %#v", first)
	}
	for _, want := range []string{"CERT", "Türkiye", "Remote", "Python and SQL"} {
		if !strings.Contains(first.RawText, want) {
			t.Fatalf("raw text missing %q: %q", want, first.RawText)
		}
	}
}

func TestAshbyBoardSourceAcceptsEmptyBoard(t *testing.T) {
	client := ashbyFixtureClient(`{"jobs":[]}`, http.StatusOK)
	source, err := NewAshbyBoardSource("board", "Company", "https://api.ashbyhq.com/posting-api/job-board/company", client)
	if err != nil {
		t.Fatal(err)
	}
	listings, err := source.FetchListings(context.Background())
	if err != nil || len(listings) != 0 {
		t.Fatalf("empty board: listings=%#v err=%v", listings, err)
	}
}

func TestAshbyBoardSourceRejectsInvalidURLAndPayload(t *testing.T) {
	for _, rawURL := range []string{
		"http://api.ashbyhq.com/posting-api/job-board/company",
		"https://jobs.ashbyhq.com/company",
		"https://api.ashbyhq.com/posting-api/job-board/",
		"https://api.ashbyhq.com/posting-api/job-board/company/extra",
	} {
		if _, err := NewAshbyBoardSource("board", "Company", rawURL, nil); err == nil {
			t.Errorf("expected invalid board URL %q to fail", rawURL)
		}
	}
	for name, body := range map[string]string{
		"malformed":     `{not-json`,
		"missing jobs":  `{"meta":{}}`,
		"missing title": `{"jobs":[{"isListed":true,"jobUrl":"https://jobs.ashbyhq.com/company/11111111-1111-1111-1111-111111111111"}]}`,
		"foreign job":   `{"jobs":[{"title":"Intern","isListed":true,"jobUrl":"https://jobs.ashbyhq.com/other/11111111-1111-1111-1111-111111111111"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			source, err := NewAshbyBoardSource("board", "Company", "https://api.ashbyhq.com/posting-api/job-board/company", ashbyFixtureClient(body, http.StatusOK))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := source.FetchListings(context.Background()); !errors.Is(err, ErrUnexpectedPage) {
				t.Fatalf("expected ErrUnexpectedPage, got %v", err)
			}
		})
	}
}

func TestAshbyBoardSourceReportsAccessProtection(t *testing.T) {
	source, err := NewAshbyBoardSource("board", "Company", "https://api.ashbyhq.com/posting-api/job-board/company", ashbyFixtureClient("rate limited", http.StatusTooManyRequests))
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.FetchListings(context.Background())
	var accessErr *AccessError
	if !errors.As(err, &accessErr) || !accessErr.Protective() {
		t.Fatalf("expected protective access error, got %v", err)
	}
}

func ashbyFixtureClient(body string, status int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}
