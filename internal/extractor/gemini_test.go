package extractor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

type fakeProvider struct {
	lastRequest analyzer.ProviderRequest
	response    string
	err         error
}

func (*fakeProvider) Name() string { return "fake-google" }

func (p *fakeProvider) Complete(_ context.Context, request analyzer.ProviderRequest) (analyzer.ProviderResponse, error) {
	p.lastRequest = request
	if p.err != nil {
		return analyzer.ProviderResponse{}, p.err
	}
	return analyzer.ProviderResponse{Content: p.response}, nil
}

func TestGeminiExtractorMapsModelOutput(t *testing.T) {
	provider := &fakeProvider{response: `{"listings":[
		{"source_block":2,"title":"Backend Staj","url":"https://x.example/j/1","summary":"Ankara","confidence":0.88}
	]}`}
	extractor, err := NewGeminiExtractor(provider, "gemini-3.1-flash-lite")
	if err != nil {
		t.Fatalf("create extractor: %v", err)
	}
	result, err := extractor.Extract(context.Background(), scraper.ExtractionRequest{
		Company: "X", PageURL: "https://x.example/careers",
		Blocks: []scraper.ExtractionBlock{{Index: 2, Text: "Backend Staj\nLINK: https://x.example/j/1"}},
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.Listings) != 1 || result.Listings[0].SourceBlock != 2 ||
		result.Listings[0].Title != "Backend Staj" || result.Listings[0].URL != "https://x.example/j/1" {
		t.Fatalf("unexpected extraction result: %#v", result.Listings)
	}

	// The model must receive only the reduced blocks, wrapped with the company
	// and page URL — never raw page HTML.
	encoded, _ := json.Marshal(provider.lastRequest.Input)
	if !strings.Contains(string(encoded), `"page_url":"https://x.example/careers"`) ||
		!strings.Contains(string(encoded), `"index":2`) {
		t.Fatalf("model input was not the reduced structure: %s", encoded)
	}
	if provider.lastRequest.JSONSchema == nil {
		t.Fatal("extraction must send a strict JSON schema")
	}
}

func TestGeminiExtractorSurfacesProviderError(t *testing.T) {
	provider := &fakeProvider{err: errors.New("boom")}
	extractor, _ := NewGeminiExtractor(provider, "gemini-3.1-flash-lite")
	if _, err := extractor.Extract(context.Background(), scraper.ExtractionRequest{}); err == nil {
		t.Fatal("expected provider error to surface")
	}
}

func TestGeminiExtractorRejectsMalformedOutput(t *testing.T) {
	provider := &fakeProvider{response: `{ not json`}
	extractor, _ := NewGeminiExtractor(provider, "gemini-3.1-flash-lite")
	if _, err := extractor.Extract(context.Background(), scraper.ExtractionRequest{}); err == nil {
		t.Fatal("expected malformed output to error")
	}
}

func TestNewGeminiExtractorValidates(t *testing.T) {
	if _, err := NewGeminiExtractor(nil, "m"); err == nil {
		t.Fatal("expected error for nil provider")
	}
	if _, err := NewGeminiExtractor(&fakeProvider{}, ""); err == nil {
		t.Fatal("expected error for empty model")
	}
}
