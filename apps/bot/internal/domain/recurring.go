package domain

import "fmt"

// Frequency is how often a recurring rule fires.
type Frequency string

const (
	FreqDaily   Frequency = "daily"
	FreqWeekly  Frequency = "weekly"
	FreqMonthly Frequency = "monthly"
	FreqYearly  Frequency = "yearly"
)

// Frequencies is the canonical list of supported recurrence frequencies.
var Frequencies = []Frequency{FreqDaily, FreqWeekly, FreqMonthly, FreqYearly}

var validFrequencies = map[Frequency]bool{
	FreqDaily: true, FreqWeekly: true, FreqMonthly: true, FreqYearly: true,
}

// RecurringRule is a validated template for an auto-posted transaction. Which
// schedule fields are meaningful depends on Frequency:
//
//	daily   — none
//	weekly  — DayOfWeek (0=Sunday .. 6=Saturday)
//	monthly — DayOfMonth (1-31)
//	yearly  — DayOfMonth (1-31) + MonthOfYear (1-12)
type RecurringRule struct {
	Amount      float64
	Type        Type
	Category    Category
	Description string
	Frequency   Frequency
	DayOfMonth  int
	MonthOfYear int
	DayOfWeek   int
}

// Validate enforces the invariants required before saving: a positive amount,
// a valid type/category/frequency, and schedule fields consistent with the
// frequency (mirroring the DB's recurring_rules_schedule_ck constraint).
func (r RecurringRule) Validate() error {
	if !(r.Amount > 0) {
		return fmt.Errorf("amount must be > 0, got %v", r.Amount)
	}
	if !validTypes[r.Type] {
		return fmt.Errorf("invalid type %q", r.Type)
	}
	if !validCategories[r.Category] {
		return fmt.Errorf("invalid category %q", r.Category)
	}
	if !validFrequencies[r.Frequency] {
		return fmt.Errorf("invalid frequency %q", r.Frequency)
	}

	switch r.Frequency {
	case FreqWeekly:
		if r.DayOfWeek < 0 || r.DayOfWeek > 6 {
			return fmt.Errorf("day_of_week must be 0-6, got %d", r.DayOfWeek)
		}
	case FreqMonthly:
		if r.DayOfMonth < 1 || r.DayOfMonth > 31 {
			return fmt.Errorf("day_of_month must be 1-31, got %d", r.DayOfMonth)
		}
	case FreqYearly:
		if r.DayOfMonth < 1 || r.DayOfMonth > 31 {
			return fmt.Errorf("day_of_month must be 1-31, got %d", r.DayOfMonth)
		}
		if r.MonthOfYear < 1 || r.MonthOfYear > 12 {
			return fmt.Errorf("month_of_year must be 1-12, got %d", r.MonthOfYear)
		}
	}
	return nil
}

// ValidFrequency reports whether s is one of the allowed frequencies.
func ValidFrequency(s string) bool { return validFrequencies[Frequency(s)] }
