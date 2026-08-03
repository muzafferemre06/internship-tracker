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
	"strings"
)

const defaultOpenRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"

type OpenRouterProvider struct {
	apiKey   string
	endpoint string
	client   *http.Client
}

func NewOpenRouterProvider(apiKey string, client *http.Client) (*OpenRouterProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("OpenRouter API key is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenRouterProvider{apiKey: apiKey, endpoint: defaultOpenRouterEndpoint, client: client}, nil
}

func (p *OpenRouterProvider) Name() string { return "openrouter" }

func (p *OpenRouterProvider) Complete(ctx context.Context, request ProviderRequest) (ProviderResponse, error) {
	inputJSON, err := json.Marshal(request.Input)
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("encode model input: %w", err)
	}
	payload := map[string]any{
		"model": request.Model,
		"messages": []map[string]string{
			{"role": "system", "content": request.SystemPrompt},
			{"role": "user", "content": string(inputJSON)},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name": "listing_analysis", "strict": true, "schema": request.JSONSchema,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("encode OpenRouter request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return ProviderResponse{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := p.client.Do(httpRequest)
	if err != nil {
		var netErr net.Error
		return ProviderResponse{}, &ProviderError{Temporary: errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded), Err: err}
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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, 2<<20))
	if err := decoder.Decode(&decoded); err != nil {
		return ProviderResponse{}, &ProviderError{Temporary: true, Err: fmt.Errorf("decode OpenRouter response: %w", err)}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values are not allowed")
		}
		return ProviderResponse{}, &ProviderError{Temporary: true, Err: fmt.Errorf("decode OpenRouter response: %w", err)}
	}
	if len(decoded.Choices) != 1 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return ProviderResponse{}, &ProviderError{Temporary: true, Err: errors.New("OpenRouter response must contain one non-empty choice")}
	}
	return ProviderResponse{
		Content: decoded.Choices[0].Message.Content,
		Usage: ProviderUsage{
			PromptTokens: decoded.Usage.PromptTokens, CompletionTokens: decoded.Usage.CompletionTokens,
			TotalTokens: decoded.Usage.TotalTokens,
		},
	}, nil
}
