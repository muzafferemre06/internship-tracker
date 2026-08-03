package scraper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	for _, status := range []int{
		http.StatusForbidden,
		http.StatusTeapot,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			source := sourceWithFixture(t, "testdata/kariyernet/meteksan-empty.html", status)

			_, err := source.FetchListings(context.Background())
			if err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) {
				t.Fatalf("expected HTTP %d status error, got %v", status, err)
			}
		})
	}
}

func TestKariyerNetSourceReportsTimeout(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
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

	_, err = source.FetchListings(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestKariyerNetSourceReturnsAccessDiagnostics(t *testing.T) {
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header: http.Header{
				"Retry-After": []string{"3600"},
				"Server":      []string{"cloudflare"},
				"Cf-Ray":      []string{"test-ray-IST"},
			},
			Body:    io.NopCloser(strings.NewReader("rate limited")),
			Request: request,
		}, nil
	})}
	source, err := NewKariyerNetSource(
		"meteksan-kariyer-net", "Meteksan", "Meteksan",
		"https://www.kariyer.net/firma-profil/meteksan", client,
	)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	source.now = func() time.Time { return now }

	_, err = source.FetchListings(context.Background())
	var accessErr *AccessError
	if !errors.As(err, &accessErr) {
		t.Fatalf("expected typed access error, got %v", err)
	}
	if !accessErr.Protective() || accessErr.StatusCode != 429 || accessErr.Server != "cloudflare" || accessErr.CFRay != "test-ray-IST" {
		t.Fatalf("unexpected access diagnostics: %#v", accessErr)
	}
	if accessErr.RetryAfter == nil || !accessErr.RetryAfter.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected retry time: %#v", accessErr.RetryAfter)
	}
}

func TestKariyerNetSourceDetectsSuccessfulChallengePage(t *testing.T) {
	source := sourceWithFixture(t, "testdata/kariyernet/access-challenge.html", http.StatusOK)

	_, err := source.FetchListings(context.Background())
	var accessErr *AccessError
	if !errors.As(err, &accessErr) || !accessErr.Protective() || !accessErr.Challenge {
		t.Fatalf("expected protective challenge error, got %v", err)
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
