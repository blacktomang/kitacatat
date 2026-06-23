package telegram

import (
	"testing"

	"github.com/kitacatat/bot/internal/domain"
)

func TestParseRupiah(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"2000000", 2_000_000, true},
		{"2.000.000", 2_000_000, true},
		{"Rp2.000.000", 2_000_000, true},
		{"2jt", 2_000_000, true},
		{"2juta", 2_000_000, true},
		{"1.5jt", 1_500_000, true},
		{"1,5jt", 1_500_000, true},
		{"50rb", 50_000, true},
		{"100ribu", 100_000, true},
		{"100k", 100_000, true},
		{"0", 0, false},
		{"-5", 0, false},
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, err := parseRupiah(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("parseRupiah(%q) = %v, %v; want %v, nil", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("parseRupiah(%q) = %v, nil; want error", c.in, got)
		}
	}
}

func TestParseRecurringRule(t *testing.T) {
	t.Run("monthly", func(t *testing.T) {
		r, err := parseRecurringRule([]string{"income", "5jt", "salary", "monthly", "25", "gaji", "bulanan"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Type != domain.TypeIncome || r.Amount != 5_000_000 || r.Frequency != domain.FreqMonthly ||
			r.DayOfMonth != 25 || r.Description != "gaji bulanan" {
			t.Fatalf("got %+v", r)
		}
		if err := r.Validate(); err != nil {
			t.Fatalf("validate: %v", err)
		}
	})

	t.Run("yearly", func(t *testing.T) {
		r, err := parseRecurringRule([]string{"expense", "1.5jt", "bills", "yearly", "25/12", "asuransi"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Frequency != domain.FreqYearly || r.DayOfMonth != 25 || r.MonthOfYear != 12 {
			t.Fatalf("got %+v", r)
		}
		if err := r.Validate(); err != nil {
			t.Fatalf("validate: %v", err)
		}
	})

	t.Run("weekly", func(t *testing.T) {
		r, err := parseRecurringRule([]string{"expense", "100rb", "food", "weekly", "senin", "jajan"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Frequency != domain.FreqWeekly || r.DayOfWeek != 1 {
			t.Fatalf("got %+v", r)
		}
	})

	t.Run("daily no when token", func(t *testing.T) {
		r, err := parseRecurringRule([]string{"expense", "50rb", "food", "daily", "kopi", "harian"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.Frequency != domain.FreqDaily || r.Description != "kopi harian" {
			t.Fatalf("got %+v", r)
		}
	})

	t.Run("errors", func(t *testing.T) {
		bad := [][]string{
			{"income", "5jt", "salary"},               // too few
			{"foo", "5jt", "salary", "monthly", "1"},  // bad type
			{"income", "5jt", "nope", "monthly", "1"}, // bad category
			{"income", "5jt", "salary", "weekly"},     // missing weekday
			{"income", "5jt", "salary", "monthly", "40"}, // day out of range
			{"income", "5jt", "salary", "yearly", "31/13"}, // month out of range
		}
		for _, args := range bad {
			if _, err := parseRecurringRule(args); err == nil {
				t.Errorf("parseRecurringRule(%v) = nil error; want error", args)
			}
		}
	})
}
