// Package identity implements authentication and user identity (PRD §3.1).
package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrEmailInUse         = errors.New("email is already in use")
	ErrSameEmail          = errors.New("new email must differ from the current email")
	ErrInvalidInput       = errors.New("invalid input")
)

// userStore is the data surface the identity Service needs (seam for tests).
// *sqlc.Queries satisfies it.
type userStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.IdentityUser, error)
	GetUserByEmail(ctx context.Context, email string) (sqlc.IdentityUser, error)
	LinkGoogleID(ctx context.Context, arg sqlc.LinkGoogleIDParams) error
	UpdateUserPassword(ctx context.Context, arg sqlc.UpdateUserPasswordParams) error
	GetUserProfile(ctx context.Context, id uuid.UUID) (sqlc.GetUserProfileRow, error)
	UpdateUserName(ctx context.Context, arg sqlc.UpdateUserNameParams) (sqlc.IdentityUser, error)
	UpdateUserEmail(ctx context.Context, arg sqlc.UpdateUserEmailParams) (sqlc.IdentityUser, error)
	UpdateEmployeePhone(ctx context.Context, arg sqlc.UpdateEmployeePhoneParams) error
}

// mailSender is the account-security mail surface (satisfied by *email.Mailer).
type mailSender interface {
	SendPasswordReset(ctx context.Context, to, name, link string) error
	SendPasswordChanged(ctx context.Context, to, name string) error
	SendEmailChangeVerify(ctx context.Context, to, name, link string) error
	SendEmailChanged(ctx context.Context, to, name, newEmail string) error
}

// Service handles login, token refresh/rotation, logout, current-user lookup,
// and password reset/change.
type Service struct {
	q           userStore
	tm          *auth.TokenManager
	store       *auth.TokenStore
	mail        mailSender
	resetTTL    time.Duration
	frontendURL string
}

// NewService builds the identity Service.
func NewService(q userStore, tm *auth.TokenManager, store *auth.TokenStore, mailer mailSender, resetTTL time.Duration, frontendURL string) *Service {
	return &Service{q: q, tm: tm, store: store, mail: mailer, resetTTL: resetTTL, frontendURL: frontendURL}
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

	// Epoch check: a password change invalidates every token issued before it.
	if user.PasswordChangedAt.Valid && claims.IssuedAt != nil &&
		claims.IssuedAt.Time.Before(user.PasswordChangedAt.Time) {
		return auth.TokenPair{}, ErrInvalidToken
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

// RequestPasswordReset issues a reset token + email when the address maps to an
// active, email-login account. It is intentionally silent (always nil) about
// missing/ineligible accounts to prevent user enumeration.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if user.Status != sqlc.SharedUserStatusActive || user.PasswordHash == nil {
		return nil // inactive or Google-only: no reset, but do not reveal it
	}
	raw, hash, err := auth.GenerateResetToken()
	if err != nil {
		return err
	}
	if err := s.store.SavePasswordReset(ctx, hash, user.ID.String(), s.resetTTL); err != nil {
		return err
	}
	link := s.frontendURL + "/reset-password?token=" + raw
	return s.mail.SendPasswordReset(ctx, user.Email, user.Name, link)
}

// ResetPassword consumes a valid reset token and sets a new password. All
// existing sessions become invalid via the password_changed_at epoch.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) (sqlc.IdentityUser, error) {
	if len(newPassword) < 8 {
		return sqlc.IdentityUser{}, ErrWeakPassword
	}
	userIDStr, err := s.store.ConsumePasswordReset(ctx, auth.HashResetToken(token))
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	if err := s.setPassword(ctx, user.ID, newPassword); err != nil {
		return sqlc.IdentityUser{}, err
	}
	_ = s.mail.SendPasswordChanged(ctx, user.Email, user.Name) // best-effort
	return user, nil
}

// ChangePassword verifies the caller's current password and sets a new one,
// invalidating all sessions (including the caller's) via the epoch.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (sqlc.IdentityUser, error) {
	if len(newPassword) < 8 {
		return sqlc.IdentityUser{}, ErrWeakPassword
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return sqlc.IdentityUser{}, err
	}
	if user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, oldPassword) {
		return sqlc.IdentityUser{}, ErrInvalidCredentials
	}
	if err := s.setPassword(ctx, user.ID, newPassword); err != nil {
		return sqlc.IdentityUser{}, err
	}
	_ = s.mail.SendPasswordChanged(ctx, user.Email, user.Name) // best-effort
	return user, nil
}

// GetProfile returns the caller's profile incl. employee phone.
func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (ProfileView, error) {
	row, err := s.q.GetUserProfile(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	return profileFromRow(row), nil
}

// UpdateProfile sets the display name and (if linked) the employee phone. The
// employee id is never taken from caller input — it is resolved from the
// caller's own profile row, so a user can only ever update their own
// employee's phone number.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, name, phone string) (ProfileView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProfileView{}, ErrInvalidInput
	}
	if _, err := s.q.UpdateUserName(ctx, sqlc.UpdateUserNameParams{ID: userID, Name: name}); err != nil {
		return ProfileView{}, err
	}
	row, err := s.q.GetUserProfile(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	if row.EmployeeID != nil {
		if err := s.q.UpdateEmployeePhone(ctx, sqlc.UpdateEmployeePhoneParams{ID: *row.EmployeeID, Phone: ptrOrNil(phone)}); err != nil {
			return ProfileView{}, err
		}
	}
	return s.GetProfile(ctx, userID)
}

// RequestEmailChange verifies the password, checks the new email is free, and
// emails a verification link to the NEW address.
func (s *Service) RequestEmailChange(ctx context.Context, userID uuid.UUID, newEmail, currentPassword string) error {
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	if strings.EqualFold(newEmail, user.Email) {
		return ErrSameEmail
	}
	if _, err := s.q.GetUserByEmail(ctx, newEmail); err == nil {
		return ErrEmailInUse
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	raw, hash, err := auth.GenerateEmailChangeToken()
	if err != nil {
		return err
	}
	if err := s.store.SaveEmailChange(ctx, hash, user.ID.String(), newEmail, s.resetTTL); err != nil {
		return err
	}
	link := s.frontendURL + "/verify-email?token=" + raw
	return s.mail.SendEmailChangeVerify(ctx, newEmail, user.Name, link)
}

// ConfirmEmailChange consumes the token and updates the email, notifying the old address.
func (s *Service) ConfirmEmailChange(ctx context.Context, token string) (sqlc.IdentityUser, error) {
	userIDStr, newEmail, err := s.store.ConsumeEmailChange(ctx, auth.HashEmailChangeToken(token))
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	// Guard: reject if the target email got taken meanwhile.
	if _, err := s.q.GetUserByEmail(ctx, newEmail); err == nil {
		return sqlc.IdentityUser{}, ErrEmailInUse
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.IdentityUser{}, err
	}
	oldUser, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	updated, err := s.q.UpdateUserEmail(ctx, sqlc.UpdateUserEmailParams{ID: userID, Email: newEmail})
	if err != nil {
		return sqlc.IdentityUser{}, mapDBError(err)
	}
	_ = s.mail.SendEmailChanged(ctx, oldUser.Email, oldUser.Name, newEmail) // best-effort
	return updated, nil
}

// RequestPasswordChange verifies the current password then emails a reset link.
func (s *Service) RequestPasswordChange(ctx context.Context, userID uuid.UUID, currentPassword string) error {
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	raw, hash, err := auth.GenerateResetToken()
	if err != nil {
		return err
	}
	if err := s.store.SavePasswordReset(ctx, hash, user.ID.String(), s.resetTTL); err != nil {
		return err
	}
	link := s.frontendURL + "/reset-password?token=" + raw
	return s.mail.SendPasswordReset(ctx, user.Email, user.Name, link)
}

func (s *Service) setPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: userID, PasswordHash: &hash})
}

// mapDBError translates a Postgres unique-violation on the email-change race
// (two confirmations for the same target email) into ErrEmailInUse.
func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailInUse
	}
	return err
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
