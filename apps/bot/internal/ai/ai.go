// Package ai turns free-form text (a chat message or OCR'd receipt) into
// structured, validated transactions using an LLM.
//
// It is provider-agnostic: the Parser interface is implemented by a native
// Gemini client (gemini.go) and an OpenAI-compatible client (openai.go) that
// works with OpenAI, Groq, OpenRouter, DeepSeek, Together, local Ollama, etc.
// Pick one via Settings (wired from env in cmd/bot).
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kitacatat/bot/internal/domain"
)

// Parser turns text into structured transactions. Returned transactions have
// every field except ownership populated; the caller validates and saves them.
type Parser interface {
	ParseTransactions(ctx context.Context, text, extraContext string) ([]domain.Transaction, error)
}

// Settings selects and configures the LLM provider.
type Settings struct {
	Provider string // "gemini" (default) or "openai" (OpenAI-compatible)
	Model    string
	APIKey   string
	BaseURL  string // OpenAI-compatible providers only
}

// New builds a Parser for the configured provider.
func New(ctx context.Context, s Settings) (Parser, error) {
	switch strings.ToLower(strings.TrimSpace(s.Provider)) {
	case "", "gemini":
		return newGemini(ctx, s.APIKey, s.Model)
	case "openai", "openai-compatible":
		return newOpenAI(s.APIKey, s.BaseURL, s.Model)
	default:
		return nil, fmt.Errorf("ai: unknown provider %q (want \"gemini\" or \"openai\")", s.Provider)
	}
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

// buildPrompt is the shared, provider-independent instruction. It both states
// the extraction rules and pins the exact JSON shape, so it works with plain
// JSON-mode providers as well as schema-constrained ones.
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

Return ONLY a JSON object of this exact shape, with no markdown and no commentary:
{"transactions":[{"amount":50000,"type":"expense","category":"food","description":"makan siang","occurred_at":""}]}
If you cannot find any transaction, return {"transactions":[]}.

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

// decode parses the model's JSON response into domain transactions. It tolerates
// a stray ```json code fence in case a provider ignores JSON mode.
func decode(raw string) ([]domain.Transaction, error) {
	raw = stripCodeFence(strings.TrimSpace(raw))
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

func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimPrefix(s, "JSON")
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
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
