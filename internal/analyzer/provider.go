package analyzer

import (
	"context"
	"errors"
	"fmt"
)

type ProviderRequest struct {
	Model        string
	SystemPrompt string
	Input        any
	JSONSchema   map[string]any
}

type ProviderUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type ProviderResponse struct {
	Content string
	Usage   ProviderUsage
}

type ModelProvider interface {
	Name() string
	Complete(context.Context, ProviderRequest) (ProviderResponse, error)
}

type ProviderError struct {
	StatusCode int
	Temporary  bool
	Err        error
}

func (e *ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("model provider returned HTTP %d: %v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("model provider request failed: %v", e.Err)
}

func (e *ProviderError) Unwrap() error { return e.Err }

func IsTemporary(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Temporary
}
