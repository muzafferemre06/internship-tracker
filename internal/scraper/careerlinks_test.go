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

func TestCareerLinksSourceProductionShapes(t *testing.T) {
	tests := []struct {
		name, fixture, pageURL, containerID, pathPrefix, wantTitle, wantURL string
		allowedHosts                                                        []string
		wantCount                                                           int
	}{
		{name: "Evreka", fixture: "testdata/careerlinks/evreka-career.html", pageURL: "https://evreka.co/career/", pathPrefix: "/career/", wantTitle: "Mandatory Internship", wantURL: "https://evreka.co/career/mandatory-internship/", wantCount: 1},
		{name: "MechSoft", fixture: "testdata/careerlinks/mechsoft-jobs.html", pageURL: "https://www.mechsoft.com.tr/jobs", pathPrefix: "/jobs/detail/", wantTitle: "Yenileme Uzmanı (Renewal Specialist)", wantURL: "https://www.mechsoft.com.tr/jobs/detail/yenileme-uzman-renewal-specialist-114", wantCount: 1},
		{name: "Layermark", fixture: "testdata/careerlinks/layermark-careers.html", pageURL: "https://www.layermark.com/careers/", containerID: "open-positions", pathPrefix: "/", wantTitle: "Business Development Intern", wantURL: "https://www.layermark.com/business-development-intern/", wantCount: 1},
		{name: "Etiya", fixture: "testdata/careerlinks/etiya-positions.html", pageURL: "https://www.etiya.com/en/career/all-open-positions", containerID: "open-positions", pathPrefix: "/portal/open-positions/", allowedHosts: []string{"etiya.peoplebox.biz"}, wantTitle: "Senior Specialist, Data Analytics", wantURL: "https://etiya.peoplebox.biz/portal/open-positions/1074", wantCount: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture, err := os.ReadFile(test.fixture)
			if err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(fixture)))}, nil
			})}
			source, err := NewCareerLinksSourceWithAllowedHosts(test.name, test.name, test.pageURL, test.containerID, test.pathPrefix, test.allowedHosts, client)
			if err != nil {
				t.Fatal(err)
			}
			listings, err := source.FetchListings(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(listings) != test.wantCount || listings[0].Title != test.wantTitle || listings[0].URL != test.wantURL {
				t.Fatalf("production shape mismatch: %#v", listings)
			}
		})
	}
}

func TestCareerLinksSourceAllowsConfiguredExternalApplicationHostWithoutFetchingIt(t *testing.T) {
	fixture, err := os.ReadFile("testdata/careerlinks/innova-careers.html")
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() != "https://www.innova.com.tr/is-ilanlari" {
			t.Fatalf("external application target must never be fetched: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(fixture)))}, nil
	})}
	source, err := NewCareerLinksSourceWithAllowedHosts(
		"innova-official-jobs", "İnnova", "https://www.innova.com.tr/is-ilanlari",
		"open-positions", "/jobs/view/", []string{"www.linkedin.com"}, client,
	)
	if err != nil {
		t.Fatal(err)
	}
	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(listings) != 2 {
		t.Fatalf("expected one official page request and two listings, requests=%d listings=%#v", requests, listings)
	}
	if listings[0].Title != "Sistem Linux Uzmanı" || listings[0].URL != "https://www.linkedin.com/jobs/view/1234567890" ||
		!strings.Contains(listings[0].RawText, "İstanbul") {
		t.Fatalf("unexpected official card extraction: %#v", listings[0])
	}
}

func TestCareerLinksSourceRejectsUnconfiguredExternalHost(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			`<main id="open-positions"><article><h4>Backend Intern</h4><a href="https://evil.example/jobs/view/1">Başvur</a></article></main>`,
		))}, nil
	})}
	source, err := NewCareerLinksSourceWithAllowedHosts("jobs", "Test", "https://example.test/careers", "open-positions", "/jobs/view/", []string{"www.linkedin.com"}, client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.FetchListings(context.Background()); err == nil || !strings.Contains(err.Error(), ErrUnexpectedPage.Error()) {
		t.Fatalf("expected unconfigured host to be rejected, got %v", err)
	}
}
