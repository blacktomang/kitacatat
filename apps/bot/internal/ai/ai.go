// Package ai turns free-form text (a chat message or OCR'd receipt) into
// structured, validated transactions using Gemini in strict JSON mode.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/kitacatat/bot/internal/domain"
)

const model = "gemini-2.0-flash"

// Client wraps the Gemini client and the parsing logic.
type Client struct {
	genai *genai.Client
}

// New constructs an AI client backed by the Gemini Developer API.
func New(ctx context.Context, apiKey string) (*Client, error) {
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("ai: new genai client: %w", err)
	}
	return &Client{genai: c}, nil
}

// parsedTx mirrors the JSON the model returns for one transaction.
type parsedTx struct {
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	OccurredAt  string  `json:"occurred_at"`
}

type parsedResult struct {
	Transactions []parsedTx `json:"transactions"`
}

// ParseTransactions sends text (plus optional extra context, e.g. a photo
// caption) to Gemini and returns the structured transactions it extracts.
// Returned transactions have every field except UserID populated; the caller
// is responsible for setting UserID, normalizing and validating before saving.
func (c *Client) ParseTransactions(ctx context.Context, text, extraContext string) ([]domain.Transaction, error) {
	text = strings.TrimSpace(text)
	if text == "" && strings.TrimSpace(extraContext) == "" {
		return nil, nil
	}

	prompt := buildPrompt(text, extraContext)

	resp, err := c.genai.Models.GenerateContent(ctx, model,
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema:   responseSchema(),
			Temperature:      genai.Ptr[float32](0),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("ai: generate content: %w", err)
	}

	raw := strings.TrimSpace(resp.Text())
	if raw == "" {
		return nil, nil
	}

	var result parsedResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("ai: decode response %q: %w", raw, err)
	}

	now := time.Now()
	out := make([]domain.Transaction, 0, len(result.Transactions))
	for _, p := range result.Transactions {
		out = append(out, domain.Transaction{
			Amount:      p.Amount,
			Type:        domain.Type(strings.ToLower(strings.TrimSpace(p.Type))),
			Category:    domain.Category(strings.ToLower(strings.TrimSpace(p.Category))),
			Description: strings.TrimSpace(p.Description),
			OccurredAt:  parseTime(p.OccurredAt, now),
		})
	}
	return out, nil
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
						"type": {
							Type: genai.TypeString,
							Enum: typeEnum(),
						},
						"category": {
							Type: genai.TypeString,
							Enum: categoryEnum(),
						},
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

func buildPrompt(text, extraContext string) string {
	var b strings.Builder
	b.WriteString(`You extract personal-finance transactions from Indonesian text.

Rules:
- Amounts are Indonesian Rupiah. Normalize all of these to a plain integer number of rupiah:
  "Rp50.000" -> 50000, "50.000" -> 50000, "50rb" -> 50000, "5jt" -> 5000000,
  "1.250.000" -> 1250000, "Rp 1.250.000,00" -> 1250000 (drop cents).
  In Indonesian, "." is a thousands separator and "," is the decimal separator.
- For a single receipt, pick the TOTAL / GRAND TOTAL / TOTAL BAYAR (not subtotal, not change/kembalian, not cash given/tunai).
- Return MULTIPLE items only if the text clearly describes several distinct transactions.
- "type" is "income" for money received (gaji/salary, transfer masuk, refund) and "expense" for money spent.
- "category" must be exactly one of: ` + strings.Join(categoryEnum(), ", ") + `.
- "occurred_at": if the text contains a date/time, output it as ISO 8601 (e.g. 2025-01-31 or 2025-01-31T13:45:00). Otherwise output an empty string.
- "description": a short label in the original language.
- If you cannot find any transaction, return an empty "transactions" array.

`)
	if strings.TrimSpace(extraContext) != "" {
		b.WriteString("Extra context (photo caption): ")
		b.WriteString(strings.TrimSpace(extraContext))
		b.WriteString("\n\n")
	}
	b.WriteString("Text to parse:\n")
	b.WriteString(text)
	return b.String()
}

func categoryEnum() []string {
	out := make([]string, len(domain.Categories))
	for i, c := range domain.Categories {
		out[i] = string(c)
	}
	return out
}

func typeEnum() []string {
	out := make([]string, len(domain.Types))
	for i, t := range domain.Types {
		out[i] = string(t)
	}
	return out
}

// parseTime tries a few common layouts and falls back to fallback (now).
func parseTime(s string, fallback time.Time) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"02/01/2006",
		"02-01-2006",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return fallback
}
