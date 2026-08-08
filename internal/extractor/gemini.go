// Package extractor implements the Faz 11 listing-extraction port
// (scraper.ListingExtractor) on top of an analyzer.ModelProvider. Keeping it in
// its own package lets the scraper define the port without importing the
// analyzer, and lets production wire any provider (Gemini, OpenRouter) behind it.
package extractor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

// GeminiExtractor turns reduced page blocks into structured listings using a
// strict-JSON model call. It never receives the full page: only the windowed
// candidate blocks the reduce stage selected.
type GeminiExtractor struct {
	provider analyzer.ModelProvider
	model    string
}

func NewGeminiExtractor(provider analyzer.ModelProvider, model string) (*GeminiExtractor, error) {
	if provider == nil {
		return nil, errors.New("model provider is required")
	}
	if strings.TrimSpace(provider.Name()) == "" {
		return nil, errors.New("model provider name is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("model name is required")
	}
	return &GeminiExtractor{provider: provider, model: model}, nil
}

func (e *GeminiExtractor) Name() string { return e.provider.Name() }

const extractionSystemPrompt = `Sana bir kariyer sayfasından indirgenmiş metin blokları verilecek. Her blok bir "index" taşır ve içinde "LINK:" satırlarıyla mutlak başvuru URL'leri olabilir.
Yalnızca gerçek staj/iş ilanı olan bloklardan ilan çıkar. Tanıtım, bülten, iletişim veya gezinme metinlerinden ilan üretme.
Her ilan için url alanını yalnızca o bloğun LINK satırlarından birinden al; URL uydurma. source_block alanı ilanın alındığı bloğun index'i olmalıdır.
Emin olmadığın bloğu atla. confidence 0 ile 1 arasında olsun. Şemaya tam uy.`

type extractionInput struct {
	Company string                `json:"company"`
	PageURL string                `json:"page_url"`
	Blocks  []extractionInputItem `json:"blocks"`
}

type extractionInputItem struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

type extractionOutput struct {
	Listings []struct {
		SourceBlock int     `json:"source_block"`
		Title       string  `json:"title"`
		URL         string  `json:"url"`
		Summary     string  `json:"summary"`
		Confidence  float64 `json:"confidence"`
	} `json:"listings"`
}

func (e *GeminiExtractor) Extract(ctx context.Context, request scraper.ExtractionRequest) (scraper.ExtractionResult, error) {
	input := extractionInput{Company: request.Company, PageURL: request.PageURL}
	for _, block := range request.Blocks {
		input.Blocks = append(input.Blocks, extractionInputItem{Index: block.Index, Text: block.Text})
	}

	response, err := e.provider.Complete(ctx, analyzer.ProviderRequest{
		Model:        e.model,
		SystemPrompt: extractionSystemPrompt,
		Input:        input,
		JSONSchema:   extractionJSONSchema(),
	})
	if err != nil {
		return scraper.ExtractionResult{}, fmt.Errorf("extraction model call failed: %w", err)
	}

	var decoded extractionOutput
	if err := json.Unmarshal([]byte(response.Content), &decoded); err != nil {
		return scraper.ExtractionResult{}, fmt.Errorf("decode extraction output: %w", err)
	}
	result := scraper.ExtractionResult{}
	for _, listing := range decoded.Listings {
		result.Listings = append(result.Listings, scraper.ExtractedListing{
			SourceBlock: listing.SourceBlock,
			Title:       listing.Title,
			URL:         listing.URL,
			Summary:     listing.Summary,
			Confidence:  listing.Confidence,
		})
	}
	return result, nil
}

func extractionJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"listings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"source_block": map[string]any{"type": "integer"},
						"title":        map[string]any{"type": "string"},
						"url":          map[string]any{"type": "string"},
						"summary":      map[string]any{"type": "string"},
						"confidence":   map[string]any{"type": "number"},
					},
					"required":             []string{"source_block", "title", "url", "summary", "confidence"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"listings"},
		"additionalProperties": false,
	}
}
