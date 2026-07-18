package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/config"
)

// Token types embedded in the JWT claims.
const (
	TokenAccess  = "access"
	TokenRefresh = "refresh"
)

// Client audiences carried in the JWT `aud` claim (ADR-0017). The audience is
// a blast-radius limiter, not a permission: web-only route groups deny
// aud=mobile, and future mobile-only endpoints require aud=mobile.
const (
	AudienceWeb    = "web"
	AudienceMobile = "mobile"
)

// ErrInvalidToken is returned when a token fails validation.
var ErrInvalidToken = errors.New("invalid token")

// Claims are the JWT claims for both access and refresh tokens.
type Claims struct {
	jwt.RegisteredClaims
	RoleID string `json:"role_id,omitempty"`
	Type   string `json:"typ"`
	// SID is the stable session id, minted at login and carried unchanged
	// through every refresh rotation (the refresh JTI rotates; the sid does
	// not). It links a token to its device-session record. Empty on tokens
	// issued before device sessions existed.
	SID string `json:"sid,omitempty"`
}

// ClientAudience returns the client audience the token was issued for.
// Tokens minted before audiences existed (live production sessions at rollout)
// carry no aud and are treated as web clients — their next refresh issues
// audience-stamped tokens (ADR-0017 rollout compatibility).
func (c *Claims) ClientAudience() string {
	if len(c.Audience) == 0 {
		return AudienceWeb
	}
	return c.Audience[0]
}

// knownAudience reports whether aud is one of the recognized client audiences.
func knownAudience(aud string) bool {
	return aud == AudienceWeb || aud == AudienceMobile
}

// TokenPair is the result of issuing access + refresh tokens.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessJTI        string
	RefreshJTI       string
	SID              string
	// Audience is the client audience ("web"/"mobile") both tokens were
	// stamped with; handlers use it to pick the refresh-token transport
	// (httpOnly cookie for web, response body for mobile — ADR-0017).
	Audience         string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// TokenManager issues and verifies JWTs.
type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

// NewTokenManager builds a TokenManager from configuration.
func NewTokenManager(cfg *config.Config) *TokenManager {
	return &TokenManager{
		secret:     []byte(cfg.JWTSecret),
		accessTTL:  cfg.JWTAccessTTL,
		refreshTTL: cfg.JWTRefreshTTL,
		issuer:     "inventra",
	}
}

// AccessTTL exposes the configured access-token lifetime.
func (tm *TokenManager) AccessTTL() time.Duration  { return tm.accessTTL }
func (tm *TokenManager) RefreshTTL() time.Duration { return tm.refreshTTL }

// Issue creates a fresh access + refresh token pair for the user, both bound to
// the given session id (sid) and stamped with the client audience. At login the
// caller mints a new sid; on refresh it passes the rotating token's existing sid
// so the session identity stays stable. The audience is likewise inherited on
// refresh — a client cannot switch identity without logging in again (ADR-0017).
func (tm *TokenManager) Issue(userID, roleID, sid, audience string) (TokenPair, error) {
	if audience == "" {
		audience = AudienceWeb
	}
	if !knownAudience(audience) {
		// Programming error at the call site; never silently widen to web.
		return TokenPair{}, fmt.Errorf("%w: unknown audience %q", ErrInvalidToken, audience)
	}
	now := time.Now()
	accessJTI := uuid.NewString()
	refreshJTI := uuid.NewString()

	access, err := tm.sign(userID, roleID, sid, audience, TokenAccess, accessJTI, now, tm.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := tm.sign(userID, "", sid, audience, TokenRefresh, refreshJTI, now, tm.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessJTI:        accessJTI,
		RefreshJTI:       refreshJTI,
		SID:              sid,
		Audience:         audience,
		AccessExpiresAt:  now.Add(tm.accessTTL),
		RefreshExpiresAt: now.Add(tm.refreshTTL),
	}, nil
}

func (tm *TokenManager) sign(userID, roleID, sid, audience, typ, jti string, now time.Time, ttl time.Duration) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{audience},
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		RoleID: roleID,
		Type:   typ,
		SID:    sid,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.secret)
}

// Parse validates the token signature, expiry, issuer, and audience, and
// returns its claims. The issuer has always been stamped at issue time and is
// now verified too. A token with no aud is accepted for rollout compatibility
// (pre-audience sessions, treated as web by ClientAudience); a token with an
// unrecognized aud is rejected outright.
func (tm *TokenManager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return tm.secret, nil
	}, jwt.WithIssuer(tm.issuer))
	if err != nil {
		return nil, ErrInvalidToken
	}
	for _, aud := range claims.Audience {
		if !knownAudience(aud) {
			return nil, ErrInvalidToken
		}
	}
	return claims, nil
}
