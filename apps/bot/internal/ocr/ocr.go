// Package ocr wraps Tesseract (via gosseract) to turn image bytes into text.
// It is configured for Indonesian receipts: both "ind" and "eng" trained data
// are loaded so mixed-language receipts read cleanly.
package ocr

import (
	"fmt"
	"strings"

	"github.com/otiai10/gosseract/v2"
)

// Engine performs OCR. It is safe to construct once and reuse; each Extract
// call uses its own gosseract client so concurrent handlers don't collide.
type Engine struct {
	languages string
	// tessdataPrefix, when set, points gosseract at a specific tessdata dir.
	// When empty, gosseract falls back to TESSDATA_PREFIX / the system default.
	tessdataPrefix string
}

// New returns an OCR engine loading the Indonesian + English language models.
// tessdataPrefix may be empty to use the Tesseract default lookup.
func New(tessdataPrefix string) *Engine {
	return &Engine{
		languages:      "ind+eng",
		tessdataPrefix: tessdataPrefix,
	}
}

// Extract runs OCR over the given image bytes and returns the recognized text.
func (e *Engine) Extract(img []byte) (string, error) {
	if len(img) == 0 {
		return "", fmt.Errorf("ocr: empty image")
	}

	client := gosseract.NewClient()
	defer client.Close()

	if e.tessdataPrefix != "" {
		// SetTessdataPrefix tells gosseract where ind/eng .traineddata live.
		client.SetTessdataPrefix(e.tessdataPrefix)
	}
	if err := client.SetLanguage(strings.Split(e.languages, "+")...); err != nil {
		return "", fmt.Errorf("ocr: set language: %w", err)
	}
	if err := client.SetImageFromBytes(img); err != nil {
		return "", fmt.Errorf("ocr: set image: %w", err)
	}

	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("ocr: recognize: %w", err)
	}
	return strings.TrimSpace(text), nil
}
