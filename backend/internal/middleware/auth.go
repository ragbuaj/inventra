// Package middleware holds cross-cutting HTTP middleware (auth, and later
// RBAC / data-scoping / field-permission).
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/auth"
)

// Gin context keys set by RequireAuth.
const (
	CtxUserID    = "user_id"
	CtxRoleID    = "role_id"
	CtxAccessJTI = "access_jti"
	CtxAccessExp = "access_exp"
	CtxSessionID = "session_id"
	// CtxAudience is the token's client audience ("web"/"mobile"); tokens
	// minted before audiences existed resolve to "web" (ADR-0017).
	CtxAudience = "audience"
)

// RequireAuth validates the Bearer access token, rejects revoked tokens, and
// stores the caller's identity in the Gin context.
func RequireAuth(tm *auth.TokenManager, store *auth.TokenStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			abort(c, "missing bearer token")
			return
		}
		claims, err := tm.Parse(token)
		if err != nil || claims.Type != auth.TokenAccess {
			abort(c, "invalid token")
			return
		}
		denied, err := store.AccessDenied(c.Request.Context(), claims.ID)
		if err != nil || denied {
			abort(c, "token revoked")
			return
		}
		// Session-alive check: if the token belongs to a device session that has
		// been revoked (or expired), reject it even though the access token has
		// not yet expired. Tokens minted before device sessions carry no sid and
		// skip this check (they age out at their TTL).
		if claims.SID != "" {
			alive, err := store.SessionAlive(c.Request.Context(), claims.SID)
			if err != nil || !alive {
				abort(c, "session revoked")
				return
			}
		}

		c.Set(CtxUserID, claims.Subject)
		c.Set(CtxRoleID, claims.RoleID)
		c.Set(CtxAccessJTI, claims.ID)
		c.Set(CtxSessionID, claims.SID)
		c.Set(CtxAudience, claims.ClientAudience())
		if claims.ExpiresAt != nil {
			c.Set(CtxAccessExp, claims.ExpiresAt.Time)
		} else {
			c.Set(CtxAccessExp, time.Now())
		}
		c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):]), true
	}
	return "", false
}

func abort(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
}
