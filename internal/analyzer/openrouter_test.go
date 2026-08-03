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

type analyzerRoundTripFunc func(*http.Request) (*http.Response, error)

func (function analyzerRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestOpenRouterProviderSendsSchemaAndReadsUsage(t *testing.T) {
	client := &http.Client{Transport: analyzerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("API key was not sent")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["model"] != "provider/model" {
			t.Fatalf("unexpected model: %#v", body["model"])
		}
		format := body["response_format"].(map[string]any)
		if format["type"] != "json_schema" {
			t.Fatalf("strict schema was not requested: %#v", format)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewBufferString(`{
				"id":"generation-id",
				"choices":[{"message":{"content":"{\"eligibility\":\"uygun\"}"}}],
				"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}
			}`)),
			Request: request,
		}, nil
	})}
	provider, err := NewOpenRouterProvider("secret", client)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	response, err := provider.Complete(context.Background(), ProviderRequest{
		Model: "provider/model", SystemPrompt: "system", Input: map[string]string{"title": "Staj"},
		JSONSchema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if response.Content != `{"eligibility":"uygun"}` || response.Usage.TotalTokens != 17 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestOpenRouterProviderClassifiesHTTPFailures(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := &http.Client{Transport: analyzerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("error")), Request: request}, nil
			})}
			provider, _ := NewOpenRouterProvider("secret", client)
			_, err := provider.Complete(context.Background(), ProviderRequest{Model: "model"})
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
