package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGoogleProviderSendsSchemaAndReadsUsage(t *testing.T) {
	client := &http.Client{Transport: analyzerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.1-flash-lite:generateContent" {
			t.Fatalf("unexpected endpoint %q", request.URL.String())
		}
		if request.Header.Get("x-goog-api-key") != "secret" {
			t.Fatal("Gemini API key was not sent in the expected header")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if strings.Contains(string(mustJSON(t, body)), "secret") {
			t.Fatal("API key leaked into request body")
		}
		generationConfig := body["generationConfig"].(map[string]any)
		if generationConfig["responseMimeType"] != "application/json" || generationConfig["responseJsonSchema"] == nil {
			t.Fatalf("structured output was not requested: %#v", generationConfig)
		}
		thinkingConfig := generationConfig["thinkingConfig"].(map[string]any)
		if thinkingConfig["thinkingLevel"] != "minimal" {
			t.Fatalf("unexpected thinking config: %#v", thinkingConfig)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
				"candidates":[{"content":{"parts":[{"text":"{\"eligibility\":\"uygun\"}"}]}}],
				"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":5,"totalTokenCount":17}
			}`)),
			Request: request,
		}, nil
	})}
	provider, err := NewGoogleProvider("secret", "minimal", client)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	response, err := provider.Complete(context.Background(), ProviderRequest{
		Model: "gemini-3.1-flash-lite", SystemPrompt: "system",
		Input: map[string]string{"title": "Staj"}, JSONSchema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if response.Content != `{"eligibility":"uygun"}` || response.Usage.TotalTokens != 17 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestGoogleProviderOmitsGeminiThinkingConfigForGemma(t *testing.T) {
	client := &http.Client{Transport: analyzerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body struct {
			GenerationConfig map[string]any `json:"generationConfig"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, exists := body.GenerationConfig["thinkingConfig"]; exists {
			t.Fatalf("Gemma request must not contain Gemini thinking config: %#v", body.GenerationConfig)
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header), Request: request,
			Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{}"}]}}]}`)),
		}, nil
	})}
	provider, _ := NewGoogleProvider("secret", "minimal", client)
	if _, err := provider.Complete(context.Background(), ProviderRequest{
		Model: "gemma-4-31b-it", JSONSchema: map[string]any{"type": "object"},
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}
}

func TestGoogleProviderClassifiesHTTPFailures(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := &http.Client{Transport: analyzerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: status, Header: make(http.Header),
					Body: io.NopCloser(strings.NewReader("error")), Request: request,
				}, nil
			})}
			provider, _ := NewGoogleProvider("secret", "minimal", client)
			_, err := provider.Complete(context.Background(), ProviderRequest{Model: "gemini-3.1-flash-lite"})
			if err == nil {
				t.Fatal("expected provider error")
			}
			wantTemporary := status == http.StatusTooManyRequests || status >= 500
			if IsTemporary(err) != wantTemporary {
				t.Fatalf("status %d temporary=%v, want %v", status, IsTemporary(err), wantTemporary)
			}
		})
	}
}

func TestNewGoogleProviderValidatesConfiguration(t *testing.T) {
	if _, err := NewGoogleProvider("", "minimal", nil); err == nil {
		t.Fatal("expected missing key to fail")
	}
	if _, err := NewGoogleProvider("secret", "extreme", nil); err == nil {
		t.Fatal("expected invalid thinking level to fail")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
	return encoded
}
