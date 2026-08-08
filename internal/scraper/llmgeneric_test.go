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

	"github.com/muzaffer/internship-tracker/internal/domain"
)

// recordingExtractor is a deterministic fake standing in for the LLM extraction
// stage. It records every call and the blocks it received, and turns any block
// mentioning "staj" into one listing using the first LINK: line as the URL.
type recordingExtractor struct {
	calls        int
	blockBatches [][]string
	fail         bool
}

func (e *recordingExtractor) Name() string { return "fake-extractor" }

func (e *recordingExtractor) Extract(_ context.Context, request ExtractionRequest) (ExtractionResult, error) {
	e.calls++
	texts := make([]string, 0, len(request.Blocks))
	for _, block := range request.Blocks {
		texts = append(texts, block.Text)
	}
	e.blockBatches = append(e.blockBatches, texts)
	if e.fail {
		return ExtractionResult{}, errors.New("model returned malformed output")
	}
	result := ExtractionResult{}
	for _, block := range request.Blocks {
		if !strings.Contains(strings.ToLower(block.Text), "staj") {
			continue
		}
		title, link := "", ""
		for _, line := range strings.Split(block.Text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "LINK:") {
				if link == "" {
					link = strings.TrimSpace(strings.TrimPrefix(line, "LINK:"))
				}
				continue
			}
			if title == "" && line != "" {
				title = line
			}
		}
		result.Listings = append(result.Listings, ExtractedListing{
			SourceBlock: block.Index, Title: title, URL: link,
			Summary: block.Text, Confidence: 0.9,
		})
	}
	return result, nil
}

func newBespokeSource(t *testing.T, extractor ListingExtractor) *LLMGenericSource {
	t.Helper()
	fixture, err := os.ReadFile("testdata/llmgeneric/bespoke.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture))}, nil
	})}
	source, err := NewLLMGenericSource("vega-careers", "Vega Yazılım", "https://kariyer.vega.example/kariyer", extractor, client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	source.now = func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }
	return source
}

func TestLLMGenericExtractsJobsAndSkipsNoise(t *testing.T) {
	extractor := &recordingExtractor{}
	source := newBespokeSource(t, extractor)

	listings, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("fetch listings: %v", err)
	}
	if extractor.calls != 1 {
		t.Fatalf("expected exactly one extraction call, got %d", extractor.calls)
	}
	if len(listings) != 2 {
		t.Fatalf("expected two extracted listings, got %d: %#v", len(listings), listings)
	}
	byTitle := map[string]domain.RawListing{}
	for _, listing := range listings {
		byTitle[listing.Title] = listing
	}
	backend, ok := byTitle["Backend Staj Programı 2026"]
	if !ok {
		t.Fatalf("backend internship not extracted: %#v", listings)
	}
	if backend.URL != "https://kariyer.vega.example/kariyer/backend-staj-2026" {
		t.Fatalf("relative apply URL was not resolved against the page: %q", backend.URL)
	}
	if backend.Company != "Vega Yazılım" || backend.SourceID != "vega-careers" {
		t.Fatalf("unexpected listing metadata: %#v", backend)
	}
	if _, ok := byTitle["Veri Bilimi Yaz Stajı"]; !ok {
		t.Fatalf("data internship not extracted: %#v", listings)
	}
	// The reduce stage must not send navigation, footer or the promo aside to the model.
	joined := strings.ToLower(strings.Join(extractor.blockBatches[0], " | "))
	for _, noise := range []string{"anasayfa", "gizlilik", "bültenimize abone"} {
		if strings.Contains(joined, noise) {
			t.Fatalf("reduce leaked noise %q into the model input: %s", noise, joined)
		}
	}
}

func TestLLMGenericHashGateSkipsUnchangedRescan(t *testing.T) {
	extractor := &recordingExtractor{}
	source := newBespokeSource(t, extractor)

	first, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	second, err := source.FetchListings(context.Background())
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if extractor.calls != 1 {
		t.Fatalf("unchanged rescan must not call the model again; calls=%d", extractor.calls)
	}
	if len(first) != len(second) || len(second) != 2 {
		t.Fatalf("cached rescan must return the same listings: first=%d second=%d", len(first), len(second))
	}
}

func TestLLMGenericSurfacesExtractionFailure(t *testing.T) {
	extractor := &recordingExtractor{fail: true}
	source := newBespokeSource(t, extractor)
	if _, err := source.FetchListings(context.Background()); err == nil {
		t.Fatal("malformed model output must surface as an error, not silent zero")
	}
}

func TestLLMGenericErrorsWhenNoCandidateBlocks(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `<html><body><nav><a href="/">Home</a></nav><main><p>Kurumsal tanıtım metni.</p></main></body></html>`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	extractor := &recordingExtractor{}
	source, err := NewLLMGenericSource("test", "Company", "https://careers.example/", extractor, client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := source.FetchListings(context.Background()); err == nil {
		t.Fatal("a page with no job-like blocks must error rather than silently return zero")
	}
	if extractor.calls != 0 {
		t.Fatalf("model must not be called when reduce found no candidates; calls=%d", extractor.calls)
	}
}

func TestLLMGenericRequiresExtractorAndValidURL(t *testing.T) {
	if _, err := NewLLMGenericSource("n", "C", "https://x.example/", nil, nil); err == nil {
		t.Fatal("expected error when extractor is nil")
	}
	if _, err := NewLLMGenericSource("n", "C", "ftp://x.example/", &recordingExtractor{}, nil); err == nil {
		t.Fatal("expected error for non-HTTP(S) URL")
	}
}

func TestLLMGenericReportsAccessProtection(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("just a moment..."))}, nil
	})}
	source, err := NewLLMGenericSource("test", "Company", "https://careers.example/", &recordingExtractor{}, client)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	_, err = source.FetchListings(context.Background())
	var accessErr *AccessError
	if !errors.As(err, &accessErr) || !accessErr.Protective() {
		t.Fatalf("expected protective access error, got %v", err)
	}
}
