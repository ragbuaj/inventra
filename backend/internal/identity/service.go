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
	"github.com/ragbuaj/inventra/internal/geoip"
	"github.com/ragbuaj/inventra/internal/storage"
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
	ErrUnsupportedType    = errors.New("unsupported image type")
	ErrTooLarge           = errors.New("image is too large")
	ErrNoAvatar           = errors.New("no avatar set")
	ErrAvatarUnavailable  = errors.New("avatar storage is not configured")
	ErrNotFound           = errors.New("not found")
	ErrNoPasswordLogin    = errors.New("account has no password login (Google-only)")
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
	UpdateUserAvatarKey(ctx context.Context, arg sqlc.UpdateUserAvatarKeyParams) error
}

// sqlcUpdateAvatarParams builds the avatar-key update params; key == nil clears it.
func sqlcUpdateAvatarParams(userID uuid.UUID, key *string) sqlc.UpdateUserAvatarKeyParams {
	return sqlc.UpdateUserAvatarKeyParams{ID: userID, AvatarKey: key}
}

// mailSender is the account-security mail surface (satisfied by *email.Mailer).
type mailSender interface {
	SendPasswordReset(ctx context.Context, to, name, link string) error
	SendPasswordChanged(ctx context.Context, to, name string) error
	SendEmailChangeVerify(ctx context.Context, to, name, link string) error
	SendEmailChanged(ctx context.Context, to, name, newEmail string) error
}

// Service handles login, token refresh/rotation, logout, current-user lookup,
// password reset/change, and device-session management.
type Service struct {
	q              userStore
	tm             *auth.TokenManager
	store          *auth.TokenStore
	mail           mailSender
	locator        geoip.Locator
	resetTTL       time.Duration
	frontendURL    string
	storage        storage.Storage
	avatarMaxBytes int64
}

// NewService builds the identity Service. locator may be nil (a no-op locator is
// substituted), so callers without GeoIP configured still function.
// objStore may be nil (deployments without MinIO); avatar endpoints then fail
// with ErrAvatarUnavailable rather than panicking.
func NewService(q userStore, tm *auth.TokenManager, store *auth.TokenStore, mailer mailSender, locator geoip.Locator, resetTTL time.Duration, frontendURL string, objStore storage.Storage, avatarMaxBytes int64) *Service {
	if locator == nil {
		locator = geoip.New("", nil)
	}
	return &Service{q: q, tm: tm, store: store, mail: mailer, locator: locator, resetTTL: resetTTL, frontendURL: frontendURL, storage: objStore, avatarMaxBytes: avatarMaxBytes}
}

// Login verifies credentials and issues an access + refresh token pair stamped
// with the client audience, opening a new device session tagged with the
// caller's user-agent and IP.
func (s *Service) Login(ctx context.Context, email, password, userAgent, ip, audience string) (auth.TokenPair, sqlc.IdentityUser, error) {
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

	pair, err := s.startSession(ctx, user, userAgent, ip, audience)
	if err != nil {
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	return pair, user, nil
}

// LoginWithGoogle links a verified Google identity to an EXISTING user (link-only)
// and issues the same token pair as local login. It never creates a user. The
// flow is browser-only (redirect + consent screen), so the audience is always web.
func (s *Service) LoginWithGoogle(ctx context.Context, email, googleSub, userAgent, ip string) (auth.TokenPair, sqlc.IdentityUser, error) {
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
	pair, err := s.startSession(ctx, user, userAgent, ip, auth.AudienceWeb)
	if err != nil {
		return auth.TokenPair{}, sqlc.IdentityUser{}, err
	}
	return pair, user, nil
}

// Refresh validates a refresh token, rotates it, and issues a new token pair on
// the SAME session id (so the device session survives rotation) and the SAME
// audience, updating the session's last-seen. A legacy token with no sid is
// promoted into a managed session so pre-Spec-B logins become visible/revocable
// after their first refresh.
func (s *Service) Refresh(ctx context.Context, refreshToken, userAgent, ip string) (auth.TokenPair, error) {
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
	// Session-alive check, parallel to RequireAuth (ADR-0017 M-3
	// defense-in-depth): a refresh token whose device session was revoked must
	// not rotate even if its JTI is somehow still whitelisted — the two Redis
	// structures could drift. Legacy tokens without a sid skip this and are
	// promoted into a managed session below.
	if claims.SID != "" {
		alive, err := s.store.SessionAlive(ctx, claims.SID)
		if err != nil {
			return auth.TokenPair{}, err
		}
		if !alive {
			return auth.TokenPair{}, ErrInvalidToken
		}
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

	// Rotate: invalidate the old refresh token first.
	if err := s.store.DeleteRefresh(ctx, claims.ID); err != nil {
		return auth.TokenPair{}, err
	}
	// Legacy (pre-session) token: mint a fresh managed session, inheriting the
	// token's audience (no-aud legacy tokens resolve to web).
	if claims.SID == "" {
		return s.startSession(ctx, user, userAgent, ip, claims.ClientAudience())
	}
	// Normal rotation: reuse the sid so the session record is preserved, and
	// propagate the audience — a client cannot switch identity on refresh.
	pair, err := s.tm.Issue(user.ID.String(), user.RoleID.String(), claims.SID, claims.ClientAudience())
	if err != nil {
		return auth.TokenPair{}, err
	}
	ttl := s.tm.RefreshTTL()
	if err := s.store.SaveRefresh(ctx, pair.RefreshJTI, user.ID.String(), ttl); err != nil {
		return auth.TokenPair{}, err
	}
	if err := s.store.TouchSession(ctx, claims.SID, user.ID.String(), pair.RefreshJTI, time.Now(), ttl); err != nil {
		return auth.TokenPair{}, err
	}
	return pair, nil
}

// Logout revokes the current access token, deletes the supplied refresh token,
// and tears down the caller's device session. userID/sid come from the caller's
// access token (may be empty for legacy tokens — the session delete is then a
// no-op).
func (s *Service) Logout(ctx context.Context, accessJTI string, accessExp time.Time, refreshToken, userID, sid string) error {
	if err := s.store.DenyAccess(ctx, accessJTI, time.Until(accessExp)); err != nil {
		return err
	}
	if refreshToken != "" {
		if claims, err := s.tm.Parse(refreshToken); err == nil && claims.Type == auth.TokenRefresh {
			_ = s.store.DeleteRefresh(ctx, claims.ID)
		}
	}
	if userID != "" && sid != "" {
		_, _ = s.store.DeleteSession(ctx, userID, sid) // best-effort
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

// AdminInitiatePasswordReset issues a single-use reset token and emails the
// reset link to the TARGET user, on behalf of an administrator acting from the
// User Management screen (gated by user.manage at the route). Unlike the
// self-service RequestPasswordReset it is NOT silent — the caller already knows
// the user exists, so it returns clear errors (ErrNotFound / ErrNoPasswordLogin)
// for the admin UI to surface, and returns the notified email on success.
//
// It is deliberately permissive about status: an inactive/suspended user that
// still has a password login is emailed anyway (the status gate is enforced at
// login time; an admin may legitimately reset a password before reactivating).
// A Google-only account (no password hash) has nothing to reset and is rejected.
func (s *Service) AdminInitiatePasswordReset(ctx context.Context, targetUserID uuid.UUID) (string, error) {
	user, err := s.q.GetUserByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if user.PasswordHash == nil {
		return "", ErrNoPasswordLogin
	}
	raw, hash, err := auth.GenerateResetToken()
	if err != nil {
		return "", err
	}
	if err := s.store.SavePasswordReset(ctx, hash, user.ID.String(), s.resetTTL); err != nil {
		return "", err
	}
	link := s.frontendURL + "/reset-password?token=" + raw
	if err := s.mail.SendPasswordReset(ctx, user.Email, user.Name, link); err != nil {
		return "", err
	}
	return user.Email, nil
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
	s.revokeAllSessions(ctx, user.ID.String()) // clear every device session (uniform logout)
	_ = s.mail.SendPasswordChanged(ctx, user.Email, user.Name) // best-effort
	return user, nil
}

// ChangePassword verifies the caller's current password and sets a new one,
// invalidating all sessions (including the caller's) via the epoch and by
// clearing every device-session record.
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
	s.revokeAllSessions(ctx, user.ID.String())
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
		if err := s.q.UpdateEmployeePhone(ctx, sqlc.UpdateEmployeePhoneParams{ID: *row.EmployeeID, Phone: ptrOrNil(strings.TrimSpace(phone))}); err != nil {
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

// startSession issues a token pair for a NEW login session and records its
// device metadata: browser/OS (derived on read from the user-agent), IP, and a
// best-effort GeoIP location. The session's stable sid is minted here and
// embedded in both tokens, along with the client audience.
func (s *Service) startSession(ctx context.Context, user sqlc.IdentityUser, userAgent, ip, audience string) (auth.TokenPair, error) {
	sid := uuid.NewString()
	pair, err := s.tm.Issue(user.ID.String(), user.RoleID.String(), sid, audience)
	if err != nil {
		return auth.TokenPair{}, err
	}
	ttl := s.tm.RefreshTTL()
	if err := s.store.SaveRefresh(ctx, pair.RefreshJTI, user.ID.String(), ttl); err != nil {
		return auth.TokenPair{}, err
	}
	city, country := s.locator.Lookup(ip)
	if err := s.store.SaveSession(ctx, sid, auth.SessionMeta{
		UserID:     user.ID.String(),
		UserAgent:  userAgent,
		IP:         ip,
		Location:   joinLocation(city, country),
		RefreshJTI: pair.RefreshJTI,
	}, ttl); err != nil {
		return auth.TokenPair{}, err
	}
	return pair, nil
}

// ListSessions returns the caller's active device sessions, current one flagged.
func (s *Service) ListSessions(ctx context.Context, userID uuid.UUID, currentSID string) ([]SessionView, error) {
	sessions, err := s.store.ListSessions(ctx, userID.String())
	if err != nil {
		return nil, err
	}
	views := make([]SessionView, 0, len(sessions))
	for _, se := range sessions {
		browser, os, deviceType := parseUserAgent(se.UserAgent)
		views = append(views, SessionView{
			ID:         se.ID,
			Browser:    browser,
			OS:         os,
			DeviceType: deviceType,
			IPAddress:  se.IP,
			Location:   se.Location,
			CreatedAt:  se.CreatedAt,
			LastSeenAt: se.LastSeenAt,
			Current:    se.ID == currentSID,
		})
	}
	return views, nil
}

// RevokeSession kills one of the caller's own sessions. It is SoD-gated: a sid
// that is not in the caller's session index yields ErrNotFound (never touches
// another user's session). The revoked device fails its next request because
// both its refresh token and its session record are gone.
func (s *Service) RevokeSession(ctx context.Context, userID uuid.UUID, sid string) error {
	owned, err := s.store.SessionOwnedBy(ctx, userID.String(), sid)
	if err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}
	jti, err := s.store.DeleteSession(ctx, userID.String(), sid)
	if err != nil {
		return err
	}
	if jti != "" {
		_ = s.store.DeleteRefresh(ctx, jti)
	}
	return nil
}

// RevokeOtherSessions kills every session except the caller's current one
// ("log out of all other devices"). Returns how many were revoked.
func (s *Service) RevokeOtherSessions(ctx context.Context, userID uuid.UUID, currentSID string) (int, error) {
	sessions, err := s.store.ListSessions(ctx, userID.String())
	if err != nil {
		return 0, err
	}
	revoked := 0
	for _, se := range sessions {
		if se.ID == currentSID {
			continue
		}
		jti, err := s.store.DeleteSession(ctx, userID.String(), se.ID)
		if err != nil {
			continue
		}
		if jti != "" {
			_ = s.store.DeleteRefresh(ctx, jti)
		}
		revoked++
	}
	return revoked, nil
}

// revokeAllSessions clears every device session for a user (best-effort), used
// on password change/reset so the whole account is logged out everywhere.
func (s *Service) revokeAllSessions(ctx context.Context, userID string) {
	jtis, err := s.store.DeleteAllSessions(ctx, userID)
	if err != nil {
		return
	}
	for _, jti := range jtis {
		_ = s.store.DeleteRefresh(ctx, jti)
	}
}

// joinLocation renders a GeoIP city/country into a display string:
// "City, Country", just the country, or "" when nothing resolved.
func joinLocation(city, country string) string {
	switch {
	case city != "" && country != "":
		return city + ", " + country
	case country != "":
		return country
	default:
		return city
	}
}
