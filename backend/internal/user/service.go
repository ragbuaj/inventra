// Package user implements Superadmin user management (PRD §3.1, FR-1.1).
package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
)

// Service-level errors.
var (
	ErrNotFound         = errors.New("user not found")
	ErrEmailExists      = errors.New("email already in use")
	ErrInvalidReference = errors.New("invalid role, office, or employee reference")
)

// Service provides user CRUD over the identity.users table.
type Service struct {
	q *sqlc.Queries
}

// NewService builds the user Service.
func NewService(q *sqlc.Queries) *Service {
	return &Service{q: q}
}

// CreateInput is the data needed to create a user.
type CreateInput struct {
	Name       string
	Email      string
	Password   string // optional; when empty the user can only sign in via Google
	RoleID     uuid.UUID
	OfficeID   *uuid.UUID
	EmployeeID *uuid.UUID
}

// Create inserts a new user, hashing the password when provided.
func (s *Service) Create(ctx context.Context, in CreateInput) (sqlc.IdentityUser, error) {
	var passwordHash *string
	if in.Password != "" {
		h, err := auth.HashPassword(in.Password)
		if err != nil {
			return sqlc.IdentityUser{}, err
		}
		passwordHash = &h
	}

	u, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
		Name:         in.Name,
		Email:        in.Email,
		PasswordHash: passwordHash,
		RoleID:       in.RoleID,
		OfficeID:     in.OfficeID,
		EmployeeID:   in.EmployeeID,
	})
	if err != nil {
		return sqlc.IdentityUser{}, mapDBError(err)
	}
	return u, nil
}

// UpdateInput replaces the mutable fields of a user (PUT semantics).
type UpdateInput struct {
	Name       string
	RoleID     uuid.UUID
	Status     sqlc.SharedUserStatus
	OfficeID   *uuid.UUID
	EmployeeID *uuid.UUID
}

// Update replaces the user's mutable fields.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (sqlc.IdentityUser, error) {
	u, err := s.q.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:         id,
		Name:       in.Name,
		RoleID:     in.RoleID,
		OfficeID:   in.OfficeID,
		EmployeeID: in.EmployeeID,
		Status:     in.Status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.IdentityUser{}, ErrNotFound
		}
		return sqlc.IdentityUser{}, mapDBError(err)
	}
	return u, nil
}

// Get returns a single non-deleted user.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (sqlc.IdentityUser, error) {
	u, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.IdentityUser{}, ErrNotFound
		}
		return sqlc.IdentityUser{}, err
	}
	return u, nil
}

// Delete soft-deletes a user.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.SoftDeleteUser(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns a page of users plus the total count, optionally filtered by search.
func (s *Service) List(ctx context.Context, search string, limit, offset int32) ([]sqlc.IdentityUser, int64, error) {
	users, err := s.q.ListUsers(ctx, sqlc.ListUsersParams{Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountUsers(ctx, search)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// mapDBError translates Postgres constraint violations to service errors.
func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return ErrEmailExists
		case "23503": // foreign_key_violation
			return ErrInvalidReference
		}
	}
	return err
}
