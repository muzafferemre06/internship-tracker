package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/scraper"
)

type GeminiRecipeLearner struct {
	provider analyzer.ModelProvider
	model    string
}

func NewGeminiRecipeLearner(provider analyzer.ModelProvider, model string) (*GeminiRecipeLearner, error) {
	if provider == nil {
		return nil, errors.New("model provider is required")
	}
	if strings.TrimSpace(provider.Name()) == "" {
		return nil, errors.New("model provider name is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("model name is required")
	}
	return &GeminiRecipeLearner{provider: provider, model: strings.TrimSpace(model)}, nil
}

const recipeLearningSystemPrompt = `Bir kariyer sayfasının temizlenmiş ve boyutu sınırlandırılmış yapısal HTML görünümünden deterministik ilan çıkarım reçetesi üret.
Yalnız şu selector dilini kullan: boşlukla ayrılan en fazla 6 descendant bileşeni; her bileşen tag, #id ve/veya .class içerebilir. Attribute selector, pseudo-selector, virgül ve child combinator kullanma.
identity_selector sayfanın doğru şirkete/kariyer alanına ait olduğunu doğrulayan kararlı bir öğeyi; identity_text o öğede beklenen kısa metni seçsin.
listing_selector her ilan kartını; title_selector ve link_selector her kart içinde tam olarak bir başlık ve href taşıyan bağlantıyı seçsin.
Geçici/hash'li class adlarını, nth-child ve görsel yerleşim ayrıntılarını kullanma. Şemaya tam uy ve yalnız JSON döndür.`

type recipeLearningInput struct {
	SourceKey string                 `json:"source_key"`
	Company   string                 `json:"company"`
	PageURL   string                 `json:"page_url"`
	Document  string                 `json:"document"`
	Reason    string                 `json:"reason"`
	Current   *recipeLearningCurrent `json:"current_recipe,omitempty"`
}

type recipeLearningCurrent struct {
	Version          int    `json:"version"`
	IdentitySelector string `json:"identity_selector"`
	IdentityText     string `json:"identity_text"`
	ListingSelector  string `json:"listing_selector"`
	TitleSelector    string `json:"title_selector"`
	LinkSelector     string `json:"link_selector"`
}

type recipeLearningOutput struct {
	IdentitySelector string `json:"identity_selector"`
	IdentityText     string `json:"identity_text"`
	ListingSelector  string `json:"listing_selector"`
	TitleSelector    string `json:"title_selector"`
	LinkSelector     string `json:"link_selector"`
}

func (l *GeminiRecipeLearner) LearnRecipe(ctx context.Context, request scraper.RecipeLearningRequest) (domain.SourceRecipe, error) {
	input := recipeLearningInput{
		SourceKey: request.SourceKey, Company: request.Company, PageURL: request.PageURL,
		Document: request.Document, Reason: request.Reason,
	}
	if request.CurrentRecipe != nil {
		input.Current = &recipeLearningCurrent{
			Version: request.CurrentRecipe.Version, IdentitySelector: request.CurrentRecipe.IdentitySelector,
			IdentityText: request.CurrentRecipe.IdentityText, ListingSelector: request.CurrentRecipe.ListingSelector,
			TitleSelector: request.CurrentRecipe.TitleSelector, LinkSelector: request.CurrentRecipe.LinkSelector,
		}
	}
	response, err := l.provider.Complete(ctx, analyzer.ProviderRequest{
		Model: l.model, SystemPrompt: recipeLearningSystemPrompt, Input: input, JSONSchema: recipeLearningJSONSchema(),
	})
	if err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("recipe model call failed: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(response.Content))
	decoder.DisallowUnknownFields()
	var output recipeLearningOutput
	if err := decoder.Decode(&output); err != nil {
		return domain.SourceRecipe{}, fmt.Errorf("decode recipe output: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.SourceRecipe{}, errors.New("decode recipe output: multiple JSON values are not allowed")
	}
	for label, value := range map[string]string{
		"identity_selector": output.IdentitySelector, "identity_text": output.IdentityText,
		"listing_selector": output.ListingSelector, "title_selector": output.TitleSelector,
		"link_selector": output.LinkSelector,
	} {
		if strings.TrimSpace(value) == "" {
			return domain.SourceRecipe{}, fmt.Errorf("decode recipe output: %s is required", label)
		}
	}
	return domain.SourceRecipe{
		SourceKey: request.SourceKey, IdentitySelector: output.IdentitySelector, IdentityText: output.IdentityText,
		ListingSelector: output.ListingSelector, TitleSelector: output.TitleSelector, LinkSelector: output.LinkSelector,
	}, nil
}

func recipeLearningJSONSchema() map[string]any {
	properties := map[string]any{}
	for _, field := range []string{"identity_selector", "identity_text", "listing_selector", "title_selector", "link_selector"} {
		properties[field] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type": "object", "properties": properties,
		"required":             []string{"identity_selector", "identity_text", "listing_selector", "title_selector", "link_selector"},
		"additionalProperties": false,
	}
}
