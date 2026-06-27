// Package identity implements authentication and user identity (PRD §3.1).
package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
)

// Service-level errors.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUserInactive       = errors.New("user is not active")
	ErrNotProvisioned     = errors.New("no account exists for this Google email")
	ErrGoogleMismatch     = errors.New("email is linked to a different Google account")
)

// userStore is the data surface the identity Service needs (seam for tests).
// *sqlc.Queries satisfies it.
type userStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.IdentityUser, error)
	GetUserByEmail(ctx context.Context, email string) (sqlc.IdentityUser, error)
	LinkGoogleID(ctx context.Context, arg sqlc.LinkGoogleIDParams) error
}

// Service handles login, token refresh/rotation, logout, and current-user lookup.
type Service struct {
	q     userStore
	tm    *auth.TokenManager
	store *auth.TokenStore
}

// NewService builds the identity Service.
func NewService(q userStore, tm *auth.TokenManager, store *auth.TokenStore) *Service {
	return &Service{q: q, tm: tm, store: store}
}

// Login verifies credentials and issues an access + refresh token pair.
func (s *Service) Login(ctx context.Context, email, password string) (auth.TokenPair, sqlc.IdentityUser, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.TokenPair{}, sqlc.IdentityUser{}, ErrInvalidCredentials
		}
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	if user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, password) {
		return auth.TokenPair{}, sqlc.IdentityUser{}, ErrInvalidCredentials
	}
	if user.Status != sqlc.SharedUserStatusActive {
		return auth.TokenPair{}, sqlc.IdentityUser{}, ErrUserInactive
	}

	pair, err := s.issue(ctx, user)
	if err != nil {
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	return pair, user, nil
}

// LoginWithGoogle links a verified Google identity to an EXISTING user (link-only)
// and issues the same token pair as local login. It never creates a user.
func (s *Service) LoginWithGoogle(ctx context.Context, email, googleSub string) (auth.TokenPair, sqlc.IdentityUser, error) {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.TokenPair{}, sqlc.IdentityUser{}, ErrNotProvisioned
		}
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	// Mismatch and status are checked BEFORE linking, so an inactive or
	// already-differently-linked account is never modified.
	if user.GoogleID != nil && *user.GoogleID != googleSub {
		return auth.TokenPair{}, sqlc.IdentityUser{}, ErrGoogleMismatch
	}
	if user.Status != sqlc.SharedUserStatusActive {
		return auth.TokenPair{}, sqlc.IdentityUser{}, ErrUserInactive
	}
	if user.GoogleID == nil {
		if err := s.q.LinkGoogleID(ctx, sqlc.LinkGoogleIDParams{ID: user.ID, GoogleID: &googleSub}); err != nil {
			return auth.TokenPair{}, sqlc.IdentityUser{}, err
		}
	}
	pair, err := s.issue(ctx, user)
	if err != nil {
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	return pair, user, nil
}

// Refresh validates a refresh token, rotates it, and issues a new token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (auth.TokenPair, error) {
	claims, err := s.tm.Parse(refreshToken)
	if err != nil || claims.Type != auth.TokenRefresh {
		return auth.TokenPair{}, ErrInvalidToken
	}
	valid, err := s.store.RefreshValid(ctx, claims.ID)
	if err != nil {
		return auth.TokenPair{}, err
	}
	if !valid {
		return auth.TokenPair{}, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return auth.TokenPair{}, ErrInvalidToken
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return auth.TokenPair{}, ErrInvalidToken
	}
	if user.Status != sqlc.SharedUserStatusActive {
		return auth.TokenPair{}, ErrUserInactive
	}

	// Rotate: invalidate the old refresh token, issue a fresh pair.
	if err := s.store.DeleteRefresh(ctx, claims.ID); err != nil {
		return auth.TokenPair{}, err
	}
	return s.issue(ctx, user)
}

// Logout revokes the current access token and deletes the supplied refresh token.
func (s *Service) Logout(ctx context.Context, accessJTI string, accessExp time.Time, refreshToken string) error {
	if err := s.store.DenyAccess(ctx, accessJTI, time.Until(accessExp)); err != nil {
		return err
	}
	if refreshToken != "" {
		if claims, err := s.tm.Parse(refreshToken); err == nil && claims.Type == auth.TokenRefresh {
			_ = s.store.DeleteRefresh(ctx, claims.ID)
		}
	}
	return nil
}

// Me returns the user for the given id.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (sqlc.IdentityUser, error) {
	return s.q.GetUserByID(ctx, userID)
}

func (s *Service) issue(ctx context.Context, user sqlc.IdentityUser) (auth.TokenPair, error) {
	pair, err := s.tm.Issue(user.ID.String(), user.RoleID.String())
	if err != nil {
		return auth.TokenPair{}, err
	}
	if err := s.store.SaveRefresh(ctx, pair.RefreshJTI, user.ID.String(), s.tm.RefreshTTL()); err != nil {
		return auth.TokenPair{}, err
	}
	return pair, nil
}
