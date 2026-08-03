package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const defaultGoogleEndpoint = "https://generativelanguage.googleapis.com/v1beta/models"

type GoogleProvider struct {
	apiKey        string
	endpoint      string
	thinkingLevel string
	client        *http.Client
}

func NewGoogleProvider(apiKey string, thinkingLevel string, client *http.Client) (*GoogleProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("Gemini API key is required")
	}
	thinkingLevel = strings.ToLower(strings.TrimSpace(thinkingLevel))
	switch thinkingLevel {
	case "", "minimal", "low", "medium", "high":
	default:
		return nil, fmt.Errorf("invalid Gemini thinking level %q", thinkingLevel)
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &GoogleProvider{
		apiKey: apiKey, endpoint: defaultGoogleEndpoint, thinkingLevel: thinkingLevel, client: client,
	}, nil
}

func (p *GoogleProvider) Name() string { return "google" }

func (p *GoogleProvider) Complete(ctx context.Context, request ProviderRequest) (ProviderResponse, error) {
	model := strings.TrimSpace(request.Model)
	if model == "" || strings.Contains(model, "/") {
		return ProviderResponse{}, errors.New("Gemini model name is invalid")
	}
	inputJSON, err := json.Marshal(request.Input)
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("encode model input: %w", err)
	}
	generationConfig := map[string]any{
		"responseMimeType":   "application/json",
		"responseJsonSchema": request.JSONSchema,
		"maxOutputTokens":    2048,
	}
	if p.thinkingLevel != "" && strings.HasPrefix(model, "gemini-3") {
		generationConfig["thinkingConfig"] = map[string]string{"thinkingLevel": p.thinkingLevel}
	}
	payload := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": request.SystemPrompt}},
		},
		"contents": []map[string]any{{
			"role": "user", "parts": []map[string]string{{"text": string(inputJSON)}},
		}},
		"generationConfig": generationConfig,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("encode Gemini request: %w", err)
	}
	endpoint := strings.TrimRight(p.endpoint, "/") + "/" + url.PathEscape(model) + ":generateContent"
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ProviderResponse{}, err
	}
	httpRequest.Header.Set("x-goog-api-key", p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := p.client.Do(httpRequest)
	if err != nil {
		var netErr net.Error
		return ProviderResponse{}, &ProviderError{
			Temporary: errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded), Err: err,
		}
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(httpResponse.Body, 64<<10))
		return ProviderResponse{}, &ProviderError{
			StatusCode: httpResponse.StatusCode,
			Temporary: httpResponse.StatusCode == http.StatusRequestTimeout ||
				httpResponse.StatusCode == http.StatusTooManyRequests || httpResponse.StatusCode >= 500,
			Err: errors.New(http.StatusText(httpResponse.StatusCode)),
		}
	}

	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			PromptTokens     int `json:"promptTokenCount"`
			CompletionTokens int `json:"candidatesTokenCount"`
			TotalTokens      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, 2<<20))
	if err := decoder.Decode(&decoded); err != nil {
		return ProviderResponse{}, &ProviderError{Temporary: true, Err: fmt.Errorf("decode Gemini response: %w", err)}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values are not allowed")
		}
		return ProviderResponse{}, &ProviderError{Temporary: true, Err: fmt.Errorf("decode Gemini response: %w", err)}
	}
	if len(decoded.Candidates) != 1 {
		return ProviderResponse{}, &ProviderError{Temporary: true, Err: errors.New("Gemini response must contain one candidate")}
	}
	var content strings.Builder
	for _, part := range decoded.Candidates[0].Content.Parts {
		content.WriteString(part.Text)
	}
	if strings.TrimSpace(content.String()) == "" {
		return ProviderResponse{}, &ProviderError{Temporary: true, Err: errors.New("Gemini response candidate is empty")}
	}
	return ProviderResponse{
		Content: content.String(),
		Usage: ProviderUsage{
			PromptTokens: decoded.Usage.PromptTokens, CompletionTokens: decoded.Usage.CompletionTokens,
			TotalTokens: decoded.Usage.TotalTokens,
		},
	}, nil
}
