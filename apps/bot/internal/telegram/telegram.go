// Package telegram wires the bot's message handlers: casual text, photos, and
// image documents all funnel into the same OCR -> Gemini -> validate -> save
// pipeline, with an allowlist guarding every interaction.
package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	ai           ai.Parser
	ocr          *ocr.Engine
	store        *store.Store
	dashboardURL string
	// allowed is the set of Telegram user IDs permitted to use the bot.
	// nil/empty means any linked account is allowed.
	allowed map[int64]struct{}
}

// New constructs a Handler.
func New(parser ai.Parser, ocrEngine *ocr.Engine, st *store.Store, dashboardURL string, allowed map[int64]struct{}) *Handler {
	return &Handler{ai: parser, ocr: ocrEngine, store: st, dashboardURL: dashboardURL, allowed: allowed}
}

// Register attaches handlers. /start and /login are open to the Telegram API,
// so every handler is gated by requireAllowed first — only Telegram user IDs
// in the allowlist (if set) reach the linked-account check.
func (h *Handler) Register(bot *tele.Bot) {
	bot.Handle("/start", h.requireAllowed(h.handleStart))
	bot.Handle("/login", h.requireAllowed(h.handleLogin))
	bot.Handle(tele.OnText, h.requireAllowed(h.handleText), h.requireLinked)
	bot.Handle(tele.OnPhoto, h.requireAllowed(h.handlePhoto), h.requireLinked)
	bot.Handle(tele.OnDocument, h.requireAllowed(h.handleDocument), h.requireLinked)
}

// requireAllowed blocks Telegram users not on the allowlist. When the
// allowlist is empty (not configured) it lets everyone through, preserving
// the existing behavior.
func (h *Handler) requireAllowed(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if len(h.allowed) == 0 {
			return next(c)
		}
		sender := c.Sender()
		if sender == nil {
			return nil
		}
		if _, ok := h.allowed[sender.ID]; !ok {
			return c.Send("Maaf, bot ini hanya untuk pengguna tertentu.")
		}
		return next(c)
	}
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

// handleStart greets the user. Account access is established by logging into
// the dashboard with "Log in with Telegram"; the bot itself no longer links
// accounts, it only checks whether the sender's Telegram id is already linked.
func (h *Handler) handleStart(c tele.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := h.store.ProfileByTelegramID(ctx, sender.ID); err == nil {
		return c.Send("Halo lagi! Akunmu sudah terhubung. Kirim catatan keuangan " +
			"(mis. \"makan siang 50rb\") atau foto struk. 💸")
	}
	return c.Send("Halo! Ketik /login untuk mendapatkan tautan masuk ke dashboard. " +
		"Setelah masuk, kamu bisa langsung mencatat dari sini. 🔐")
}

// loginTokenTTL is how long a /login link stays valid.
const loginTokenTTL = 5 * time.Minute

// handleLogin issues a one-time dashboard login link. Telegram has already
// authenticated the sender, so the bot can vouch for them: it stores a
// short-lived token and DMs a link the dashboard exchanges for a session.
func (h *Handler) handleLogin(c tele.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	token, err := randomToken()
	if err != nil {
		log.Printf("generate login token: %v", err)
		return c.Send("Maaf, gagal membuat tautan login. Coba lagi ya.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.store.CreateLoginToken(ctx, token, sender.ID, sender.Username, displayNameFor(sender), time.Now().Add(loginTokenTTL)); err != nil {
		log.Printf("create login token: %v", err)
		return c.Send("Maaf, gagal membuat tautan login. Coba lagi ya.")
	}

	link := fmt.Sprintf("%s/?token=%s", strings.TrimRight(h.dashboardURL, "/"), token)
	return c.Send("🔐 Tautan masuk dashboard (berlaku 5 menit, sekali pakai):\n" + link)
}

func displayNameFor(u *tele.User) string {
	if u.Username != "" {
		return "@" + u.Username
	}
	if name := strings.TrimSpace(u.FirstName + " " + u.LastName); name != "" {
		return name
	}
	return fmt.Sprintf("tg_%d", u.ID)
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
		if errors.Is(err, ai.ErrRateLimited) {
			return c.Send("⏳ Layanan AI lagi sibuk atau kuotanya penuh. " +
				"Coba lagi beberapa saat lagi ya.")
		}
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
