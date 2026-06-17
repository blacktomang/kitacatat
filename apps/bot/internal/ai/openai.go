package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kitacatat/bot/internal/domain"
)

// openaiParser talks to any OpenAI-compatible /chat/completions endpoint
// (OpenAI, Groq, OpenRouter, DeepSeek, Together, local Ollama, ...). The
// provider is chosen purely by BaseURL + Model + APIKey — no extra code.
type openaiParser struct {
	http    *http.Client
	baseURL string
	apiKey  string
	model   string
}

func newOpenAI(apiKey, baseURL, model string) (*openaiParser, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ai: openai-compatible provider requires an API key (AI_API_KEY)")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("ai: openai-compatible provider requires a base URL (AI_BASE_URL)")
	}
	if model == "" {
		return nil, fmt.Errorf("ai: openai-compatible provider requires a model (AI_MODEL)")
	}
	return &openaiParser{
		http:    &http.Client{Timeout: 60 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *openaiParser) ParseTransactions(ctx context.Context, text, extraContext string) ([]domain.Transaction, error) {
	if strings.TrimSpace(text) == "" && strings.TrimSpace(extraContext) == "" {
		return nil, nil
	}

	body, err := json.Marshal(chatRequest{
		Model:          o.model,
		Temperature:    0,
		ResponseFormat: &responseFormat{Type: "json_object"},
		Messages: []chatMessage{
			{Role: "user", Content: buildPrompt(text, extraContext)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai: provider returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("ai: decode chat response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("ai: provider error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("ai: provider returned no choices")
	}

	return decode(parsed.Choices[0].Message.Content)
}
