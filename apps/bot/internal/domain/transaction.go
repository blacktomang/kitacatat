// Package domain holds the core business types shared across the bot:
// the Transaction value, the closed Category/Type enums, and validation that
// every transaction must pass before it is persisted.
package domain

import (
	"fmt"
	"time"
)

// Type is the direction of money flow.
type Type string

const (
	TypeIncome  Type = "income"
	TypeExpense Type = "expense"
)

// Category is the closed set of spending/earning buckets. Gemini is told to
// pick from exactly these; anything outside the set is rejected in validation.
type Category string

const (
	CategoryFood          Category = "food"
	CategoryTransport     Category = "transport"
	CategoryBills         Category = "bills"
	CategorySalary        Category = "salary"
	CategoryShopping      Category = "shopping"
	CategoryHealth        Category = "health"
	CategoryEntertainment Category = "entertainment"
	CategoryOther         Category = "other"
)

// Categories is the canonical ordered list, handy for prompts and validation.
var Categories = []Category{
	CategoryFood,
	CategoryTransport,
	CategoryBills,
	CategorySalary,
	CategoryShopping,
	CategoryHealth,
	CategoryEntertainment,
	CategoryOther,
}

// Types is the canonical list of transaction directions.
var Types = []Type{TypeIncome, TypeExpense}

var (
	validCategories = func() map[Category]bool {
		m := make(map[Category]bool, len(Categories))
		for _, c := range Categories {
			m[c] = true
		}
		return m
	}()
	validTypes = map[Type]bool{TypeIncome: true, TypeExpense: true}
)

// Transaction is a single parsed financial entry, ready to persist. Ownership
// (which profile it belongs to) is attached by the caller at save time, not
// carried on the parsed value itself.
type Transaction struct {
	Amount      float64
	Type        Type
	Category    Category
	Description string
	OccurredAt  time.Time
}

// Normalize fills in safe defaults: a missing/zero OccurredAt becomes now.
// It is applied before validation.
func (t *Transaction) Normalize(now time.Time) {
	if t.OccurredAt.IsZero() {
		t.OccurredAt = now
	}
}

// Validate enforces the invariants required before saving: a positive amount
// and a type/category from the allowed sets. Invalid items are dropped by the
// caller rather than stored.
func (t Transaction) Validate() error {
	if !(t.Amount > 0) {
		return fmt.Errorf("amount must be > 0, got %v", t.Amount)
	}
	if !validTypes[t.Type] {
		return fmt.Errorf("invalid type %q", t.Type)
	}
	if !validCategories[t.Category] {
		return fmt.Errorf("invalid category %q", t.Category)
	}
	if t.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	return nil
}

// ValidCategory reports whether s is one of the allowed categories.
func ValidCategory(s string) bool { return validCategories[Category(s)] }

// ValidType reports whether s is one of the allowed types.
func ValidType(s string) bool { return validTypes[Type(s)] }
