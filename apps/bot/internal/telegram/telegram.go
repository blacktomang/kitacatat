// Package telegram wires the bot's message handlers: casual text, photos, and
// image documents all funnel into the same OCR -> Gemini -> validate -> save
// pipeline, with an allowlist guarding every interaction.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/kitacatat/bot/internal/ai"
	"github.com/kitacatat/bot/internal/domain"
	"github.com/kitacatat/bot/internal/ocr"
	"github.com/kitacatat/bot/internal/store"
)

// processTimeout bounds OCR + Gemini + DB work per message.
const processTimeout = 60 * time.Second

// Handler holds the dependencies the message handlers need.
type Handler struct {
	ai    *ai.Client
	ocr   *ocr.Engine
	store *store.Store
}

// New constructs a Handler.
func New(aiClient *ai.Client, ocrEngine *ocr.Engine, st *store.Store) *Handler {
	return &Handler{ai: aiClient, ocr: ocrEngine, store: st}
}

// Register attaches handlers. /start is open (it handles account linking),
// while the catch-all message handlers are gated by requireLinked so only
// Telegram accounts linked to a dashboard profile are served — no hardcoded
// allowlist.
func (h *Handler) Register(bot *tele.Bot) {
	bot.Handle("/start", h.handleStart)
	bot.Handle(tele.OnText, h.handleText, h.requireLinked)
	bot.Handle(tele.OnPhoto, h.handlePhoto, h.requireLinked)
	bot.Handle(tele.OnDocument, h.handleDocument, h.requireLinked)
}

// requireLinked looks up the profile linked to the sender's Telegram id and
// stashes it on the context. Unlinked senders are told to link via the
// dashboard; everyone else proceeds.
func (h *Handler) requireLinked(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		sender := c.Sender()
		if sender == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		profile, err := h.store.ProfileByTelegramID(ctx, sender.ID)
		if errors.Is(err, store.ErrNotLinked) {
			return c.Send("Akun Telegram-mu belum terhubung. Buka dashboard, login, lalu " +
				"hubungkan akun Telegram untuk mulai mencatat. 🔗")
		}
		if err != nil {
			log.Printf("lookup profile: %v", err)
			return c.Send("Maaf, ada masalah. Coba lagi ya.")
		}
		c.Set("profile", profile)
		return next(c)
	}
}

// handleStart handles both a plain /start and the deep-link /start <code> used
// to link a Telegram account to a dashboard profile.
func (h *Handler) handleStart(c tele.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code := strings.TrimSpace(c.Message().Payload)
	if code == "" {
		if _, err := h.store.ProfileByTelegramID(ctx, sender.ID); err == nil {
			return c.Send("Halo lagi! Akunmu sudah terhubung. Kirim catatan keuangan " +
				"(mis. \"makan siang 50rb\") atau foto struk. 💸")
		}
		return c.Send("Halo! Untuk mulai: buka dashboard, login, lalu tekan " +
			"\"Hubungkan Telegram\". Kamu akan diarahkan ke chat ini dengan kode tautan.")
	}

	profile, err := h.store.LinkTelegram(ctx, code, sender.ID)
	if errors.Is(err, store.ErrInvalidCode) {
		return c.Send("Kode tautan tidak valid atau sudah kadaluarsa. Buat kode baru di dashboard ya.")
	}
	if err != nil {
		// Most likely a unique-constraint violation: this Telegram account is
		// already linked to a different profile.
		log.Printf("link telegram: %v", err)
		return c.Send("Gagal menghubungkan akun. Mungkin Telegram ini sudah tertaut ke akun lain.")
	}

	name := profile.DisplayName.String
	if name == "" {
		name = "kamu"
	}
	return c.Send(fmt.Sprintf("✅ Berhasil terhubung sebagai %s! "+
		"Sekarang kirim catatan keuangan atau foto struk. 💸", name))
}

// handleText handles casual text messages like "makan siang 50rb".
func (h *Handler) handleText(c tele.Context) error {
	return h.parseAndReply(c, c.Text(), "")
}

// handlePhoto handles photos: download the (largest) image, OCR it, then parse.
// Telegram/telebot expose the highest-resolution size as Message.Photo.
func (h *Handler) handlePhoto(c tele.Context) error {
	photo := c.Message().Photo
	if photo == nil {
		return c.Send("Tidak ada gambar yang bisa dibaca.")
	}
	return h.handleImage(c, &photo.File, c.Message().Caption)
}

// handleDocument handles images sent "as file": only image/* mime types are
// treated as receipts; anything else is ignored politely.
func (h *Handler) handleDocument(c tele.Context) error {
	doc := c.Message().Document
	if doc == nil {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(doc.MIME), "image/") {
		return c.Send("File ini bukan gambar, jadi tidak bisa kubaca sebagai struk.")
	}
	return h.handleImage(c, &doc.File, c.Message().Caption)
}

// handleImage downloads a Telegram file, runs OCR, and parses the result.
func (h *Handler) handleImage(c tele.Context, file *tele.File, caption string) error {
	// OCR + AI take a moment — show activity to the user.
	_ = c.Notify(tele.Typing)
	if err := c.Send("📸 Sedang membaca..."); err != nil {
		log.Printf("send reading notice: %v", err)
	}

	img, err := h.downloadFile(c, file)
	if err != nil {
		log.Printf("download file: %v", err)
		return c.Send("Gagal mengunduh gambar. Coba lagi ya.")
	}

	text, err := h.ocr.Extract(img)
	if err != nil {
		log.Printf("ocr extract: %v", err)
		return c.Send("Gagal membaca teks dari gambar.")
	}

	return h.parseAndReply(c, text, caption)
}

// downloadFile fetches the bytes of a Telegram file via the bot API.
func (h *Handler) downloadFile(c tele.Context, file *tele.File) ([]byte, error) {
	rc, err := c.Bot().File(file)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// parseAndReply is the shared tail of every handler: send text to Gemini,
// validate each candidate, save the valid ones, and reply with a summary.
func (h *Handler) parseAndReply(c tele.Context, text, caption string) error {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	profile, ok := c.Get("profile").(store.Profile)
	if !ok {
		// requireLinked always sets this; guard defensively.
		return c.Send("Akun belum terhubung.")
	}

	candidates, err := h.ai.ParseTransactions(ctx, text, caption)
	if err != nil {
		log.Printf("parse transactions: %v", err)
		return c.Send("Maaf, ada masalah saat memproses. Coba lagi ya.")
	}

	now := time.Now()
	var saved []domain.Transaction
	for _, t := range candidates {
		t.Normalize(now)
		if err := t.Validate(); err != nil {
			log.Printf("dropping invalid transaction: %v", err)
			continue
		}
		if _, err := h.store.SaveTransaction(ctx, profile.ID, t); err != nil {
			log.Printf("save transaction: %v", err)
			continue
		}
		saved = append(saved, t)
	}

	if len(saved) == 0 {
		return c.Send("Hmm, aku tidak menemukan transaksi yang bisa dicatat. " +
			"Coba tulis lebih jelas, mis. \"makan siang 50rb\".")
	}

	return c.Send(formatSummary(saved))
}

// formatSummary builds the Indonesian confirmation message.
func formatSummary(txs []domain.Transaction) string {
	var b strings.Builder
	if len(txs) == 1 {
		b.WriteString("✅ Dicatat:\n")
	} else {
		fmt.Fprintf(&b, "✅ %d transaksi dicatat:\n", len(txs))
	}
	for _, t := range txs {
		desc := t.Description
		if desc != "" {
			desc = " — " + desc
		}
		fmt.Fprintf(&b, "• %s · %s (%s)%s\n",
			formatRupiah(t.Amount), t.Category, typeLabel(t.Type), desc)
	}
	return strings.TrimRight(b.String(), "\n")
}

func typeLabel(t domain.Type) string {
	switch t {
	case domain.TypeIncome:
		return "pemasukan"
	case domain.TypeExpense:
		return "pengeluaran"
	default:
		return string(t)
	}
}

// formatRupiah renders 1250000 as "Rp1.250.000".
func formatRupiah(amount float64) string {
	n := int64(amount)
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := "Rp" + strings.Join(parts, ".")
	if neg {
		out = "-" + out
	}
	return out
}
