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

func TestKariyerNetSourceParsesListings(t *testing.T) {
	source := sourceWithFixture(t, "testdata/kariyernet/meteksan-listings.html", http.StatusOK)
	source.now = func() time.Time { return time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC) }

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("fetch listings: %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("expected two deduplicated listings, got %d", len(listings))
	}
	if listings[0].Title != "Yazılım Geliştirme Stajyeri" {
		t.Fatalf("unexpected title %q", listings[0].Title)
	}
	if listings[0].URL != "https://www.kariyer.net/is-ilani/meteksan-yazilim-stajyeri-123" {
		t.Fatalf("unexpected URL %q", listings[0].URL)
	}
	if listings[0].RawText == "" || listings[0].FetchedAt.IsZero() {
		t.Fatalf("listing was not normalized: %#v", listings[0])
	}
}

func TestKariyerNetSourceAllowsRecognizedPageWithoutListings(t *testing.T) {
	source := sourceWithFixture(t, "testdata/kariyernet/meteksan-empty.html", http.StatusOK)

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("fetch listings: %v", err)
	}
	if len(listings) != 0 {
		t.Fatalf("expected no listings, got %d", len(listings))
	}
}

func TestKariyerNetSourceKeepsGroupCompanyWithDifferentPageName(t *testing.T) {
	source := sourceWithNamedFixture(
		t,
		"testdata/kariyernet/aselsannet-listings.html",
		"ASELSAN",
		"Aselsannet",
	)

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("fetch affiliate listings: %v", err)
	}
	if len(listings) != 2 || listings[0].Company != "ASELSAN" {
		t.Fatalf("unexpected affiliate listings: %#v", listings)
	}
}

func TestKariyerNetSourceRejectsChangedPage(t *testing.T) {
	source := sourceWithFixture(t, "testdata/kariyernet/unrecognized.html", http.StatusOK)

	_, err := source.FetchListings(context.Background())
	if !errors.Is(err, ErrUnexpectedPage) {
		t.Fatalf("expected unexpected page error, got %v", err)
	}
}

func TestKariyerNetSourceRejectsListingWithoutTitle(t *testing.T) {
	source := sourceWithFixture(t, "testdata/kariyernet/missing-title.html", http.StatusOK)

	_, err := source.FetchListings(context.Background())
	if !errors.Is(err, ErrUnexpectedPage) || !strings.Contains(err.Error(), "no title") {
		t.Fatalf("expected missing title error, got %v", err)
	}
}

func TestKariyerNetSourceReportsHTTPFailure(t *testing.T) {
	source := sourceWithFixture(t, "testdata/kariyernet/meteksan-empty.html", http.StatusTooManyRequests)

	_, err := source.FetchListings(context.Background())
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected HTTP status error, got %v", err)
	}
}

func TestKariyerNetSourceHonorsContextCancellation(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	source, err := NewKariyerNetSource(
		"meteksan-kariyer-net",
		"Meteksan",
		"Meteksan",
		"https://www.kariyer.net/firma-profil/meteksan",
		client,
	)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = source.FetchListings(ctx)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func sourceWithFixture(t *testing.T, fixturePath string, status int) *KariyerNetSource {
	t.Helper()
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    request,
		}, nil
	})}

	source, err := NewKariyerNetSource(
		"meteksan-kariyer-net",
		"Meteksan Savunma",
		"Meteksan Savunma",
		"https://www.kariyer.net/firma-profil/meteksan-savunma-7324-220178",
		client,
	)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	return source
}

func sourceWithNamedFixture(t *testing.T, fixturePath, company, pageName string) *KariyerNetSource {
	t.Helper()
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    request,
		}, nil
	})}
	source, err := NewKariyerNetSource(
		"affiliate-kariyer-net",
		company,
		pageName,
		"https://www.kariyer.net/firma-profil/affiliate",
		client,
	)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	return source
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
