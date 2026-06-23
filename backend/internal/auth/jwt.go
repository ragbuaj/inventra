package auth

import (
	"errors"
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

// ErrInvalidToken is returned when a token fails validation.
var ErrInvalidToken = errors.New("invalid token")

// Claims are the JWT claims for both access and refresh tokens.
type Claims struct {
	jwt.RegisteredClaims
	RoleID string `json:"role_id,omitempty"`
	Type   string `json:"typ"`
}

// TokenPair is the result of issuing access + refresh tokens.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessJTI        string
	RefreshJTI       string
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

// Issue creates a fresh access + refresh token pair for the user.
func (tm *TokenManager) Issue(userID, roleID string) (TokenPair, error) {
	now := time.Now()
	accessJTI := uuid.NewString()
	refreshJTI := uuid.NewString()

	access, err := tm.sign(userID, roleID, TokenAccess, accessJTI, now, tm.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := tm.sign(userID, "", TokenRefresh, refreshJTI, now, tm.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessJTI:        accessJTI,
		RefreshJTI:       refreshJTI,
		AccessExpiresAt:  now.Add(tm.accessTTL),
		RefreshExpiresAt: now.Add(tm.refreshTTL),
	}, nil
}

func (tm *TokenManager) sign(userID, roleID, typ, jti string, now time.Time, ttl time.Duration) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.issuer,
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		RoleID: roleID,
		Type:   typ,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.secret)
}

// Parse validates the token signature and expiry and returns its claims.
func (tm *TokenManager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
