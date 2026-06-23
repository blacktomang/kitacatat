package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/kitacatat/bot/internal/domain"
	"github.com/kitacatat/bot/internal/store"
)

// recurringUsage documents the positional /recurring format.
const recurringUsage = "🔁 *Langganan* (otomatis tercatat tiap periode)\n\n" +
	"Format:\n" +
	"`/recurring add <income|expense> <jumlah> <kategori> <freq> <kapan> [catatan]`\n\n" +
	"freq & kapan:\n" +
	"• `monthly <tgl 1-31>` — mis. `monthly 25`\n" +
	"• `yearly <tgl/bln>` — mis. `yearly 25/12`\n" +
	"• `weekly <hari>` — mis. `weekly senin`\n" +
	"• `daily` — tanpa tanggal\n\n" +
	"Contoh:\n" +
	"`/recurring add expense 2jt bills monthly 5 sewa kos`\n" +
	"`/recurring add income 5jt salary monthly 25 gaji`\n" +
	"`/recurring add expense 1.5jt bills yearly 25/12 asuransi`\n" +
	"`/recurring add expense 100rb food weekly senin jajan`\n\n" +
	"Lainnya: `/recurring` (daftar) · `/recurring del <nomor>`"

// handleRecurring is the /recurring command: list / add / delete rules.
func (h *Handler) handleRecurring(c tele.Context) error {
	profile, ok := c.Get("profile").(store.Profile)
	if !ok {
		return c.Send("Akun belum terhubung.")
	}

	payload := strings.TrimSpace(c.Message().Payload)
	fields := strings.Fields(payload)

	sub := "list"
	if len(fields) > 0 {
		sub = strings.ToLower(fields[0])
	}

	switch sub {
	case "list", "":
		return h.recurringList(c, profile)
	case "add", "tambah":
		return h.recurringAdd(c, profile, fields[1:])
	case "del", "hapus", "delete":
		return h.recurringDelete(c, profile, fields)
	default:
		return c.Send(recurringUsage, tele.ModeMarkdown)
	}
}

func (h *Handler) recurringList(c tele.Context, profile store.Profile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rules, err := h.store.RecurringRulesByUser(ctx, profile.ID)
	if err != nil {
		log.Printf("list recurring rules: %v", err)
		return c.Send("Maaf, gagal mengambil daftar langganan. Coba lagi ya.")
	}
	if len(rules) == 0 {
		return c.Send("Belum ada langganan.\n\n"+recurringUsage, tele.ModeMarkdown)
	}

	var b strings.Builder
	b.WriteString("🔁 Langganan:\n")
	for i, r := range rules {
		desc := ""
		if r.Description.Valid && r.Description.String != "" {
			desc = " — " + r.Description.String
		}
		status := ""
		if !r.Active {
			status = " (nonaktif)"
		}
		fmt.Fprintf(&b, "%d. %s · %s (%s) · %s%s%s\n",
			i+1, formatRupiah(r.Amount), r.Category, typeLabel(domain.Type(r.Type)),
			scheduleLabel(r), desc, status)
	}
	b.WriteString("\nHapus dengan /recurring del <nomor>.")
	return c.Send(strings.TrimRight(b.String(), "\n"))
}

func (h *Handler) recurringAdd(c tele.Context, profile store.Profile, args []string) error {
	rule, err := parseRecurringRule(args)
	if err != nil {
		return c.Send("⚠️ "+err.Error()+"\n\n"+recurringUsage, tele.ModeMarkdown)
	}
	if err := rule.Validate(); err != nil {
		log.Printf("invalid recurring rule: %v", err)
		return c.Send("⚠️ Langganan tidak valid: "+err.Error()+"\n\n"+recurringUsage, tele.ModeMarkdown)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	saved, err := h.store.SaveRecurringRule(ctx, profile.ID, rule)
	if err != nil {
		log.Printf("save recurring rule: %v", err)
		return c.Send("Maaf, gagal menyimpan langganan. Coba lagi ya.")
	}

	desc := ""
	if saved.Description.Valid && saved.Description.String != "" {
		desc = " — " + saved.Description.String
	}
	return c.Send(fmt.Sprintf("✅ Langganan disimpan:\n%s · %s (%s) · %s%s\n\nAkan tercatat otomatis.",
		formatRupiah(saved.Amount), saved.Category, typeLabel(domain.Type(saved.Type)),
		scheduleLabel(saved), desc))
}

func (h *Handler) recurringDelete(c tele.Context, profile store.Profile, fields []string) error {
	if len(fields) < 2 {
		return c.Send("Sebutkan nomornya, mis. /recurring del 2")
	}
	n, err := strconv.Atoi(fields[1])
	if err != nil || n < 1 {
		return c.Send("Nomor tidak valid. Lihat daftar dengan /recurring.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rules, err := h.store.RecurringRulesByUser(ctx, profile.ID)
	if err != nil {
		log.Printf("list recurring rules: %v", err)
		return c.Send("Maaf, gagal mengambil daftar langganan. Coba lagi ya.")
	}
	if n > len(rules) {
		return c.Send(fmt.Sprintf("Nomor %d tidak ada. Kamu punya %d langganan.", n, len(rules)))
	}

	target := rules[n-1]
	deleted, err := h.store.DeleteRecurringRule(ctx, target.ID, profile.ID)
	if err != nil {
		log.Printf("delete recurring rule: %v", err)
		return c.Send("Maaf, gagal menghapus. Coba lagi ya.")
	}
	if deleted == 0 {
		return c.Send("Langganan itu sudah tidak ada.")
	}
	return c.Send(fmt.Sprintf("🗑️ Dihapus: %s · %s · %s.",
		formatRupiah(target.Amount), target.Category, scheduleLabel(target)))
}

// parseRecurringRule turns the positional `add` arguments into a domain rule.
// Returns a user-facing (Indonesian) error on malformed input.
//
//	<income|expense> <amount> <category> <freq> <when> [description...]
func parseRecurringRule(args []string) (domain.RecurringRule, error) {
	var r domain.RecurringRule
	if len(args) < 4 {
		return r, fmt.Errorf("argumen kurang.")
	}

	typ, err := parseType(args[0])
	if err != nil {
		return r, err
	}
	amount, err := parseRupiah(args[1])
	if err != nil {
		return r, err
	}
	cat := domain.Category(strings.ToLower(args[2]))
	if !domain.ValidCategory(string(cat)) {
		return r, fmt.Errorf("kategori tidak dikenal: %q.", args[2])
	}
	freq, err := parseFrequency(args[3])
	if err != nil {
		return r, err
	}

	r.Type = typ
	r.Amount = amount
	r.Category = cat
	r.Frequency = freq

	descFrom := 4
	switch freq {
	case domain.FreqDaily:
		// no schedule token
	case domain.FreqWeekly:
		if len(args) < 5 {
			return r, fmt.Errorf("sebutkan harinya, mis. `weekly senin`.")
		}
		dow, err := parseWeekday(args[4])
		if err != nil {
			return r, err
		}
		r.DayOfWeek = dow
		descFrom = 5
	case domain.FreqMonthly:
		if len(args) < 5 {
			return r, fmt.Errorf("sebutkan tanggalnya (1-31), mis. `monthly 25`.")
		}
		d, err := strconv.Atoi(args[4])
		if err != nil || d < 1 || d > 31 {
			return r, fmt.Errorf("tanggal harus 1-31, dapat %q.", args[4])
		}
		r.DayOfMonth = d
		descFrom = 5
	case domain.FreqYearly:
		if len(args) < 5 {
			return r, fmt.Errorf("sebutkan tanggal/bulan, mis. `yearly 25/12`.")
		}
		day, month, err := parseDayMonth(args[4])
		if err != nil {
			return r, err
		}
		r.DayOfMonth = day
		r.MonthOfYear = month
		descFrom = 5
	}

	if len(args) > descFrom {
		r.Description = strings.Join(args[descFrom:], " ")
	}
	return r, nil
}

func parseType(s string) (domain.Type, error) {
	switch strings.ToLower(s) {
	case "income", "in", "masuk", "pemasukan":
		return domain.TypeIncome, nil
	case "expense", "out", "keluar", "pengeluaran":
		return domain.TypeExpense, nil
	}
	return "", fmt.Errorf("tipe harus `income` atau `expense`, dapat %q.", s)
}

func parseFrequency(s string) (domain.Frequency, error) {
	switch strings.ToLower(s) {
	case "daily", "harian":
		return domain.FreqDaily, nil
	case "weekly", "mingguan":
		return domain.FreqWeekly, nil
	case "monthly", "bulanan":
		return domain.FreqMonthly, nil
	case "yearly", "annual", "tahunan":
		return domain.FreqYearly, nil
	}
	return "", fmt.Errorf("freq harus daily/weekly/monthly/yearly, dapat %q.", s)
}

// rupiahSuffix maps Indonesian magnitude shorthands to multipliers, checked
// longest-first so "juta" wins over "jt", etc.
var rupiahSuffix = []struct {
	suffix string
	mult   float64
}{
	{"juta", 1_000_000}, {"jt", 1_000_000},
	{"ribu", 1_000}, {"rb", 1_000}, {"k", 1_000},
}

// parseRupiah accepts plain numbers ("2000000", "2.000.000") and shorthand
// ("2jt", "1.5jt", "50rb", "100k"). No AI — pure string handling.
func parseRupiah(s string) (float64, error) {
	orig := s
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSpace(strings.TrimPrefix(s, "rp"))
	if s == "" {
		return 0, fmt.Errorf("jumlah kosong.")
	}

	for _, sfx := range rupiahSuffix {
		if strings.HasSuffix(s, sfx.suffix) {
			num := strings.TrimSpace(strings.TrimSuffix(s, sfx.suffix))
			num = strings.ReplaceAll(num, ",", ".") // allow "1,5jt"
			f, err := strconv.ParseFloat(num, 64)
			if err != nil || f <= 0 {
				return 0, fmt.Errorf("jumlah tidak valid: %q.", orig)
			}
			return f * sfx.mult, nil
		}
	}

	// Plain number: treat "." and "," as thousand separators.
	digits := strings.NewReplacer(".", "", ",", "").Replace(s)
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("jumlah tidak valid: %q.", orig)
	}
	return float64(n), nil
}

// weekdays maps English and Indonesian day names to Postgres dow (0=Sunday).
var weekdays = map[string]int{
	"sun": 0, "sunday": 0, "minggu": 0, "min": 0, "ahad": 0,
	"mon": 1, "monday": 1, "senin": 1, "sen": 1,
	"tue": 2, "tuesday": 2, "selasa": 2, "sel": 2,
	"wed": 3, "wednesday": 3, "rabu": 3, "rab": 3,
	"thu": 4, "thursday": 4, "kamis": 4, "kam": 4,
	"fri": 5, "friday": 5, "jumat": 5, "jum": 5, "jum'at": 5,
	"sat": 6, "saturday": 6, "sabtu": 6, "sab": 6,
}

func parseWeekday(s string) (int, error) {
	if d, ok := weekdays[strings.ToLower(s)]; ok {
		return d, nil
	}
	return 0, fmt.Errorf("hari tidak dikenal: %q (mis. senin, selasa, ...).", s)
}

// parseDayMonth parses "DD/MM" or "DD-MM" into (day, month).
func parseDayMonth(s string) (int, int, error) {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '-' })
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("format tanggal/bulan harus `DD/MM`, dapat %q.", s)
	}
	day, errD := strconv.Atoi(parts[0])
	month, errM := strconv.Atoi(parts[1])
	if errD != nil || errM != nil || day < 1 || day > 31 || month < 1 || month > 12 {
		return 0, 0, fmt.Errorf("tanggal/bulan tidak valid: %q.", s)
	}
	return day, month, nil
}

var idWeekdayNames = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
var idMonthNames = []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// scheduleLabel renders a stored rule's schedule in Indonesian.
func scheduleLabel(r store.RecurringRule) string {
	switch domain.Frequency(r.Frequency) {
	case domain.FreqDaily:
		return "tiap hari"
	case domain.FreqWeekly:
		if d := int(r.DayOfWeek.Int32); r.DayOfWeek.Valid && d >= 0 && d < len(idWeekdayNames) {
			return "tiap " + idWeekdayNames[d]
		}
	case domain.FreqMonthly:
		if r.DayOfMonth.Valid {
			return fmt.Sprintf("tiap tgl %d", r.DayOfMonth.Int32)
		}
	case domain.FreqYearly:
		if m := int(r.MonthOfYear.Int32); r.DayOfMonth.Valid && r.MonthOfYear.Valid && m >= 1 && m < len(idMonthNames) {
			return fmt.Sprintf("tiap %d %s", r.DayOfMonth.Int32, idMonthNames[m])
		}
	}
	return string(r.Frequency)
}
