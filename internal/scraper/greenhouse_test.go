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

func TestGreenhouseSourceParsesJobs(t *testing.T) {
	fixture, err := os.ReadFile("testdata/greenhouse/jobs.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "boards-api.greenhouse.io" ||
			request.URL.Path != "/v1/boards/acmerobotics/jobs" {
			t.Fatalf("unexpected request URL: %s", request.URL)
		}
		if request.URL.Query().Get("content") != "true" {
			t.Fatalf("expected content=true query to fetch descriptions, got %q", request.URL.RawQuery)
		}
		if !strings.Contains(request.Header.Get("User-Agent"), "personal career monitoring") {
			t.Fatal("expected descriptive user agent")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := NewGreenhouseSource("acme-greenhouse", "Acme Robotics", "https://boards-api.greenhouse.io/v1/boards/acmerobotics/jobs", client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	source.now = func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }
	policy := source.AccessPolicy()
	if policy.Scope != "boards-api.greenhouse.io" {
		t.Fatalf("unexpected access policy: %#v", policy)
	}

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("fetch listings: %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("expected two listings, got %d", len(listings))
	}
	first := listings[0]
	if first.Title != "Backend Engineering Intern" || first.Company != "Acme Robotics" || first.SourceID != "acme-greenhouse" {
		t.Fatalf("unexpected first listing: %#v", first)
	}
	if first.URL != "https://boards.greenhouse.io/acmerobotics/jobs/5501001" {
		t.Fatalf("unexpected URL: %q", first.URL)
	}
	for _, want := range []string{"Backend Engineering Intern", "Istanbul, Turkey", "summer intern", "gRPC"} {
		if !strings.Contains(first.RawText, want) {
			t.Fatalf("RawText missing %q: %q", want, first.RawText)
		}
	}
	if strings.Contains(first.RawText, "<") || strings.Contains(first.RawText, "&lt;") {
		t.Fatalf("RawText still contains markup: %q", first.RawText)
	}
}

func TestGreenhouseSourceErrorsOnMalformedOrIncomplete(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{ not json`},
		{name: "missing jobs key", body: `{"meta":{"total":0}}`},
		{name: "job missing title", body: `{"jobs":[{"id":1,"absolute_url":"https://boards.greenhouse.io/x/jobs/1"}]}`},
		{name: "job missing url", body: `{"jobs":[{"id":1,"title":"Intern"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			source, err := NewGreenhouseSource("test", "Company", "https://boards-api.greenhouse.io/v1/boards/x/jobs", client)
			if err != nil {
				t.Fatalf("create source: %v", err)
			}
			if _, err := source.FetchListings(context.Background()); err == nil {
				t.Fatalf("expected error for %s, got nil", test.name)
			}
		})
	}
}

func TestGreenhouseSourceAllowsEmptyBoard(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"jobs":[]}`))}, nil
	})}
	source, err := NewGreenhouseSource("test", "Company", "https://boards-api.greenhouse.io/v1/boards/x/jobs", client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("empty board must be a valid deterministic result, got %v", err)
	}
	if len(listings) != 0 {
		t.Fatalf("expected zero listings, got %d", len(listings))
	}
}

func TestGreenhouseSourceRejectsInvalidURL(t *testing.T) {
	for _, bad := range []string{
		"https://example.com/v1/boards/x/jobs",
		"http://boards-api.greenhouse.io/v1/boards/x/jobs",
		"https://boards-api.greenhouse.io/v1/boards/x",
	} {
		if _, err := NewGreenhouseSource("test", "Company", bad, nil); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestGreenhouseSourceReportsAccessProtection(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("rate limited"))}, nil
	})}
	source, err := NewGreenhouseSource("test", "Company", "https://boards-api.greenhouse.io/v1/boards/x/jobs", client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	_, err = source.FetchListings(context.Background())
	var accessErr *AccessError
	if !errors.As(err, &accessErr) || !accessErr.Protective() {
		t.Fatalf("expected protective access error, got %v", err)
	}
}
