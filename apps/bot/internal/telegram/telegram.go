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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
}

// New constructs a Handler.
func New(parser ai.Parser, ocrEngine *ocr.Engine, st *store.Store, dashboardURL string) *Handler {
	return &Handler{ai: parser, ocr: ocrEngine, store: st, dashboardURL: dashboardURL}
}

// Register attaches handlers. /start is open (it handles account linking),
// while the catch-all message handlers are gated by requireLinked so only
// Telegram accounts linked to a dashboard profile are served — no hardcoded
// allowlist.
func (h *Handler) Register(bot *tele.Bot) {
	bot.Handle("/start", h.handleStart)
	bot.Handle("/login", h.handleLogin)
	bot.Handle("/buku", h.handleBuku)
	bot.Handle(tele.OnText, h.handleText, h.requireLinked)
	bot.Handle(tele.OnPhoto, h.handlePhoto, h.requireLinked)
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

// handleBuku routes /buku subcommands: list, switch/create, share, unshare,
// info, leave.
func (h *Handler) handleBuku(c tele.Context) error {
	sender := c.Sender()
	if sender == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	profile, err := h.store.ProfileByTelegramID(ctx, sender.ID)
	if errors.Is(err, store.ErrNotLinked) {
		return c.Send("Akun Telegram-mu belum terhubung. Ketik /login dulu ya.")
	}
	if err != nil {
		log.Printf("buku: lookup profile: %v", err)
		return c.Send("Maaf, ada masalah. Coba lagi ya.")
	}

	arg := strings.TrimSpace(strings.TrimPrefix(c.Text(), "/buku"))
	parts := strings.Fields(arg)

	// /buku (no args) → list
	if len(parts) == 0 {
		return h.bukuList(c, profile.ID)
	}

	cmd := parts[0]

	switch cmd {
	case "bagikan", "share":
		return h.bukuShare(c, profile.ID, parts[1:])
	case "hapus-bagian", "unshare":
		return h.bukuUnshare(c, profile.ID, parts[1:])
	case "info":
		return h.bukuInfo(c, profile.ID, parts[1:])
	case "keluar", "leave":
		return h.bukuLeave(c, profile.ID, parts[1:])
	default:
		// /buku <name> → switch to existing or create
		return h.bukuSwitchOrCreate(c, profile, arg)
	}
}

func (h *Handler) bukuList(c tele.Context, profileID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	books, err := h.store.ListBooks(ctx, profileID)
	if err != nil {
		log.Printf("buku list: %v", err)
		return c.Send("Gagal memuat daftar buku.")
	}
	if len(books) == 0 {
		return c.Send("Kamu belum punya buku. Kirim /buku <nama> untuk membuat.")
	}

	// Fetch fresh profile for active book ID.
	p, err := h.store.ProfileByTelegramID(ctx, c.Sender().ID)
	if err != nil {
		log.Printf("buku list: refresh profile: %v", err)
		return c.Send("Gagal memuat daftar buku.")
	}
	activeID := uuid.Nil
	if p.ActiveBookID.Valid {
		activeID = uuid.UUID(p.ActiveBookID.Bytes)
	}

	var b strings.Builder
	b.WriteString("📚 Buku-mu:\n")
	for _, book := range books {
		marker := ""
		shared := ""
		if book.ID == activeID {
			marker = " (aktif)"
		}
		if book.Role != "owner" {
			shared = " (dibagikan)"
		}
		fmt.Fprintf(&b, "• %s%s%s\n", book.Name, marker, shared)
	}
	b.WriteString("\nKirim /buku <nama> untuk beralih.")
	return c.Send(b.String())
}

func (h *Handler) bukuSwitchOrCreate(c tele.Context, profile store.Profile, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to find an existing book by this name — first by ownership, then by
	// shared membership.
	book, err := h.store.GetBookByOwnerAndName(ctx, profile.ID, name)
	if errors.Is(err, store.ErrBookNotFound) {
		// Not an owned book — check shared memberships.
		bwr, sbErr := h.store.GetBookForUserByName(ctx, profile.ID, name)
		if sbErr != nil {
			// Book doesn't exist at all — create it.
			return h.bukuCreate(ctx, profile, name, c)
		}
		book = bwr.Group
	} else if err != nil {
		log.Printf("buku switch: %v", err)
		return c.Send("Gagal beralih buku.")
	}

	if err := h.store.SetActiveBook(ctx, profile.ID, book.ID); err != nil {
		log.Printf("buku switch: %v", err)
		return c.Send("Gagal beralih buku.")
	}
	return c.Send(fmt.Sprintf("✅ Buku \"%s\" aktif. Transaksi selanjutnya akan dicatat di sini.", book.Name))
}

func (h *Handler) bukuCreate(ctx context.Context, profile store.Profile, name string, c tele.Context) error {
	book, err := h.store.CreateBook(ctx, profile.ID, name)
	if errors.Is(err, store.ErrBookNameTaken) {
		return c.Send("Kamu sudah punya buku dengan nama itu.")
	}
	if err != nil {
		log.Printf("buku create: %v", err)
		return c.Send("Gagal membuat buku.")
	}

	if err := h.store.SetActiveBook(ctx, profile.ID, book.ID); err != nil {
		log.Printf("buku set active after create: %v", err)
	}
	return c.Send(fmt.Sprintf("✅ Buku \"%s\" dibuat dan jadi aktif.", book.Name))
}

func (h *Handler) bukuShare(c tele.Context, profileID uuid.UUID, args []string) error {
	if len(args) < 2 {
		return c.Send("Gunakan: /buku bagikan <nama-buku> @username")
	}
	bookName, username := args[0], strings.TrimPrefix(args[1], "@")
	if username == "" {
		return c.Send("Gunakan: /buku bagikan <nama-buku> @username")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	book, err := h.store.GetBookByOwnerAndName(ctx, profileID, bookName)
	if errors.Is(err, store.ErrBookNotFound) {
		return c.Send("Buku tidak ditemukan.")
	}
	if err != nil {
		log.Printf("buku share: %v", err)
		return c.Send("Gagal mencari buku.")
	}

	if book.OwnerID != profileID {
		return c.Send("Hanya pemilik yang bisa membagikan buku.")
	}

	targetProfile, err := h.store.ProfileByTelegramUsername(ctx, username)
	if errors.Is(err, store.ErrUserNotFound) {
		return c.Send(fmt.Sprintf("Pengguna @%s tidak ditemukan. Pastikan mereka sudah login ke dashboard.", username))
	}
	if err != nil {
		log.Printf("buku share: lookup user: %v", err)
		return c.Send("Gagal mencari pengguna.")
	}

	if _, err := h.store.GetGroupMember(ctx, book.ID, targetProfile.ID); err == nil {
		return c.Send(fmt.Sprintf("@%s sudah jadi anggota buku ini.", username))
	}

	if err := h.store.AddGroupMember(ctx, book.ID, targetProfile.ID, "viewer"); err != nil {
		log.Printf("buku share: add member: %v", err)
		return c.Send("Gagal membagikan buku.")
	}

	return c.Send(fmt.Sprintf("✅ Buku \"%s\" dibagikan ke @%s (read-only).", book.Name, username))
}

func (h *Handler) bukuUnshare(c tele.Context, profileID uuid.UUID, args []string) error {
	if len(args) < 2 {
		return c.Send("Gunakan: /buku hapus-bagian <nama-buku> @username")
	}
	bookName, username := args[0], strings.TrimPrefix(args[1], "@")
	if username == "" {
		return c.Send("Gunakan: /buku hapus-bagian <nama-buku> @username")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	book, err := h.store.GetBookByOwnerAndName(ctx, profileID, bookName)
	if errors.Is(err, store.ErrBookNotFound) {
		return c.Send("Buku tidak ditemukan.")
	}
	if err != nil {
		log.Printf("buku unshare: %v", err)
		return c.Send("Gagal mencari buku.")
	}

	if book.OwnerID != profileID {
		return c.Send("Hanya pemilik yang bisa mengelola anggota.")
	}

	targetProfile, err := h.store.ProfileByTelegramUsername(ctx, username)
	if errors.Is(err, store.ErrUserNotFound) {
		return c.Send(fmt.Sprintf("Pengguna @%s tidak ditemukan.", username))
	}
	if err != nil {
		log.Printf("buku unshare: lookup user: %v", err)
		return c.Send("Gagal mencari pengguna.")
	}

	if err := h.store.RemoveGroupMember(ctx, book.ID, targetProfile.ID); err != nil {
		log.Printf("buku unshare: %v", err)
		return c.Send("Gagal menghapus anggota.")
	}

	return c.Send(fmt.Sprintf("✅ Akses @%s ke buku \"%s\" dihapus.", username, book.Name))
}

func (h *Handler) bukuInfo(c tele.Context, profileID uuid.UUID, args []string) error {
	if len(args) < 1 {
		return c.Send("Gunakan: /buku info <nama-buku>")
	}
	_ = profileID
	name := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	book, err := h.store.GetBookByOwnerAndName(ctx, profileID, name)
	if errors.Is(err, store.ErrBookNotFound) {
		// Could be a shared book — search all books.
		books, listErr := h.store.ListBooks(ctx, profileID)
		if listErr != nil {
			return c.Send("Buku tidak ditemukan.")
		}
		found := false
		for _, b := range books {
			if b.Name == name {
				book = b.Group
				found = true
				break
			}
		}
		if !found {
			return c.Send("Buku tidak ditemukan.")
		}
	}
	if err != nil {
		log.Printf("buku info: %v", err)
		return c.Send("Gagal mencari buku.")
	}

	// Get members — we only have group_members but need profile display names.
	// The sqlc Query doesn't have a "list members" query. For now show minimal info.
	return c.Send(fmt.Sprintf("📚 \"%s\"\nPemilik: kamu", book.Name))
}

func (h *Handler) bukuLeave(c tele.Context, profileID uuid.UUID, args []string) error {
	if len(args) < 1 {
		return c.Send("Gunakan: /buku keluar <nama-buku>")
	}
	name := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	books, err := h.store.ListBooks(ctx, profileID)
	if err != nil {
		log.Printf("buku leave: %v", err)
		return c.Send("Gagal memuat buku.")
	}

	var book store.BookWithRole
	found := false
	for _, b := range books {
		if b.Name == name {
			book = b
			found = true
			break
		}
	}
	if !found {
		return c.Send("Buku tidak ditemukan.")
	}
	if book.Role == "owner" {
		return c.Send("Kamu adalah pemilik buku ini. Kalau mau menghapus, hapus dari dashboard atau hubungi admin.")
	}

	if err := h.store.RemoveGroupMember(ctx, book.ID, profileID); err != nil {
		log.Printf("buku leave: %v", err)
		return c.Send("Gagal keluar dari buku.")
	}

	// Clear active book if it was this one.
	p, _ := h.store.ProfileByTelegramID(ctx, c.Sender().ID)
	if p.ActiveBookID.Valid && uuid.UUID(p.ActiveBookID.Bytes) == book.ID {
		_ = h.store.SetActiveBook(ctx, profileID, uuid.Nil)
	}

	return c.Send(fmt.Sprintf("✅ Kamu keluar dari buku \"%s\".", book.Name))
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
		return c.Send("Akun belum terhubung.")
	}

	// Resolve active book.
	groupID := uuid.Nil
	var groupName string
	if profile.ActiveBookID.Valid {
		bid := uuid.UUID(profile.ActiveBookID.Bytes)
		book, err := h.store.GetBook(ctx, bid)
		if errors.Is(err, store.ErrBookNotFound) {
			_ = h.store.SetActiveBook(ctx, profile.ID, uuid.Nil)
		} else if err != nil {
			log.Printf("resolve active book: %v", err)
		} else {
			member, err := h.store.GetGroupMember(ctx, book.ID, profile.ID)
			if errors.Is(err, pgx.ErrNoRows) {
				_ = h.store.SetActiveBook(ctx, profile.ID, uuid.Nil)
			} else if err != nil {
				log.Printf("resolve active book member: %v", err)
			} else if member.Role == "viewer" {
				return c.Send("Kamu hanya bisa melihat buku \"" + book.Name + "\", tidak bisa menambah transaksi.")
			} else {
				groupID = book.ID
				groupName = book.Name
			}
		}
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
		if _, err := h.store.SaveTransaction(ctx, profile.ID, groupID, t); err != nil {
			log.Printf("save transaction: %v", err)
			continue
		}
		saved = append(saved, t)
	}

	if len(saved) == 0 {
		return c.Send("Hmm, aku tidak menemukan transaksi yang bisa dicatat. " +
			"Coba tulis lebih jelas, mis. \"makan siang 50rb\".")
	}

	return c.Send(formatSummary(saved, groupName))
}

// formatSummary builds the Indonesian confirmation message.
func formatSummary(txs []domain.Transaction, bookName string) string {
	var b strings.Builder
	if bookName != "" {
		fmt.Fprintf(&b, "✅ Dicatat ke \"%s\":\n", bookName)
	} else if len(txs) == 1 {
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
