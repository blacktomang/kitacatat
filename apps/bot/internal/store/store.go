package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitacatat/bot/internal/domain"
)

// ErrNotLinked is returned when no profile is linked to a given Telegram id.
var ErrNotLinked = errors.New("telegram account not linked to any profile")

// Store wraps the pgx connection pool and the sqlc-generated Queries, exposing
// a small domain-oriented API to the rest of the app.
type Store struct {
	pool *pgxpool.Pool
	q    *Queries
}

// Connect opens a pgx pool against databaseURL and pings it.
func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool, q: New(pool)}, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// SaveTransaction persists a single validated domain.Transaction owned by the
// given profile, returning the stored row (with its generated id/created_at).
func (s *Store) SaveTransaction(ctx context.Context, profileID uuid.UUID, t domain.Transaction) (Transaction, error) {
	var desc pgtype.Text
	if t.Description != "" {
		desc = pgtype.Text{String: t.Description, Valid: true}
	}
	return s.q.CreateTransaction(ctx, CreateTransactionParams{
		UserID:      profileID,
		Amount:      t.Amount,
		Type:        string(t.Type),
		Category:    string(t.Category),
		Description: desc,
		OccurredAt:  t.OccurredAt,
	})
}

// CreateLoginToken stores a short-lived, single-use dashboard login code for
// the given Telegram user.
func (s *Store) CreateLoginToken(ctx context.Context, token string, telegramID int64, username, displayName string, expiresAt time.Time) error {
	return s.q.CreateLoginToken(ctx, CreateLoginTokenParams{
		Token:            token,
		TelegramID:       telegramID,
		TelegramUsername: optText(username),
		DisplayName:      optText(displayName),
		ExpiresAt:        expiresAt,
	})
}

func optText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ProfileByTelegramID returns the profile linked to the given Telegram user id,
// or ErrNotLinked if none exists.
func (s *Store) ProfileByTelegramID(ctx context.Context, telegramID int64) (Profile, error) {
	p, err := s.q.GetProfileByTelegramID(ctx, pgtype.Int8{Int64: telegramID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotLinked
	}
	return p, err
}

