package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/kitacatat/bot/internal/domain"
)

// geminiParser uses Google's native genai SDK with a strict response schema.
type geminiParser struct {
	client *genai.Client
	model  string
}

func newGemini(ctx context.Context, apiKey, model string) (*geminiParser, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ai: gemini requires an API key")
	}
	if model == "" {
		return nil, fmt.Errorf("ai: gemini requires a model")
	}
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("ai: new genai client: %w", err)
	}
	return &geminiParser{client: c, model: model}, nil
}

func (g *geminiParser) ParseTransactions(ctx context.Context, text, extraContext string) ([]domain.Transaction, error) {
	if strings.TrimSpace(text) == "" && strings.TrimSpace(extraContext) == "" {
		return nil, nil
	}

	resp, err := g.client.Models.GenerateContent(ctx, g.model,
		genai.Text(buildPrompt(text, extraContext)),
		&genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema:   responseSchema(),
			Temperature:      genai.Ptr[float32](0),
		},
	)
	if err != nil {
		var apiErr genai.APIError
		if errors.As(err, &apiErr) && (apiErr.Code == 429 || apiErr.Code == 503) {
			return nil, fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Message)
		}
		return nil, fmt.Errorf("ai: generate content: %w", err)
	}
	return decode(resp.Text())
}

// responseSchema is the strict JSON schema Gemini must conform to.
func responseSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"transactions": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"amount": {
							Type:        genai.TypeNumber,
							Description: "Amount in Indonesian Rupiah as a plain number, no separators (e.g. 50000).",
						},
						"type":     {Type: genai.TypeString, Enum: typeEnum()},
						"category": {Type: genai.TypeString, Enum: categoryEnum()},
						"description": {
							Type:        genai.TypeString,
							Description: "Short human description, e.g. 'makan siang' or 'gaji bulanan'.",
						},
						"occurred_at": {
							Type:        genai.TypeString,
							Description: "ISO 8601 date or datetime if a date is present in the text, otherwise empty string.",
						},
					},
					Required: []string{"amount", "type", "category", "description", "occurred_at"},
				},
			},
		},
		Required: []string{"transactions"},
	}
}
