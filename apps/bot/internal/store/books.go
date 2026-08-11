package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrBookNotFound  = errors.New("buku tidak ditemukan")
	ErrBookNameTaken = errors.New("nama buku sudah ada")
)

// BookWithRole combines a Group row with the current user's membership role.
type BookWithRole struct {
	Group
	Role string
}

// CreateBook inserts a new group owned by ownerID with the given name. The
// on_group_created trigger auto-inserts the owner as a member.
func (s *Store) CreateBook(ctx context.Context, ownerID uuid.UUID, name string) (Group, error) {
	// Check for duplicate name under this owner.
	existing, err := s.q.GetBookByOwnerAndName(ctx, GetBookByOwnerAndNameParams{
		OwnerID: ownerID,
		Name:    name,
	})
	if err == nil {
		return Group{}, fmt.Errorf("%w: %s", ErrBookNameTaken, existing.Name)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Group{}, fmt.Errorf("check duplicate book: %w", err)
	}

	return s.q.CreateBook(ctx, CreateBookParams{OwnerID: ownerID, Name: name})
}

// GetBook returns the group by its ID, or ErrBookNotFound.
func (s *Store) GetBook(ctx context.Context, id uuid.UUID) (Group, error) {
	book, err := s.q.GetBook(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrBookNotFound
	}
	return book, err
}

// GetBookByOwnerAndName finds a book by its owner and name, or
// ErrBookNotFound.
func (s *Store) GetBookByOwnerAndName(ctx context.Context, ownerID uuid.UUID, name string) (Group, error) {
	book, err := s.q.GetBookByOwnerAndName(ctx, GetBookByOwnerAndNameParams{
		OwnerID: ownerID,
		Name:    name,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrBookNotFound
	}
	return book, err
}

// ListBooks returns all groups the user is a member of, with their role.
func (s *Store) ListBooks(ctx context.Context, userID uuid.UUID) ([]BookWithRole, error) {
	rows, err := s.q.ListBooksByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]BookWithRole, len(rows))
	for i, r := range rows {
		result[i] = BookWithRole{
			Group: Group{
				ID:        r.ID,
				OwnerID:   r.OwnerID,
				Name:      r.Name,
				CreatedAt: r.CreatedAt,
			},
			Role: r.Role,
		}
	}
	return result, nil
}

// AddGroupMember adds a user as a member of a group with the given role.
func (s *Store) AddGroupMember(ctx context.Context, groupID, userID uuid.UUID, role string) error {
	return s.q.AddGroupMember(ctx, AddGroupMemberParams{
		GroupID: groupID,
		UserID:  userID,
		Role:    role,
	})
}

// RemoveGroupMember removes a non-owner member from a group.
func (s *Store) RemoveGroupMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return s.q.RemoveGroupMember(ctx, RemoveGroupMemberParams{
		GroupID: groupID,
		UserID:  userID,
	})
}

// GetGroupMember returns the membership row for a user in a group. Returns
// pgx.ErrNoRows if not a member.
func (s *Store) GetGroupMember(ctx context.Context, groupID, userID uuid.UUID) (GroupMember, error) {
	return s.q.GetGroupMember(ctx, GetGroupMemberParams{
		GroupID: groupID,
		UserID:  userID,
	})
}

// SetActiveBook sets the active book on the user's profile. Pass uuid.Nil to
// clear.
func (s *Store) SetActiveBook(ctx context.Context, profileID uuid.UUID, bookID uuid.UUID) error {
	var gid pgtype.UUID
	if bookID != uuid.Nil {
		gid = pgtype.UUID{Bytes: bookID, Valid: true}
	}
	return s.q.SetActiveBook(ctx, SetActiveBookParams{
		ID:           profileID,
		ActiveBookID: gid,
	})
}

// ProfileByTelegramUsername looks up a profile by the linked Telegram
// username (without the @ prefix), or ErrUserNotFound.
func (s *Store) ProfileByTelegramUsername(ctx context.Context, username string) (Profile, error) {
	var u pgtype.Text
	if username != "" {
		u = pgtype.Text{String: username, Valid: true}
	}
	p, err := s.q.GetProfileByTelegramUsername(ctx, u)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrUserNotFound
	}
	return p, err
}

// GetBookForUserByName finds a book by name across all groups the user is a
// member of (both owned and shared). Returns ErrBookNotFound if none match.
func (s *Store) GetBookForUserByName(ctx context.Context, userID uuid.UUID, name string) (BookWithRole, error) {
	books, err := s.ListBooks(ctx, userID)
	if err != nil {
		return BookWithRole{}, err
	}
	for _, b := range books {
		if b.Name == name {
			return b, nil
		}
	}
	return BookWithRole{}, ErrBookNotFound
}
