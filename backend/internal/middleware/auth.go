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

		c.Set(CtxUserID, claims.Subject)
		c.Set(CtxRoleID, claims.RoleID)
		c.Set(CtxAccessJTI, claims.ID)
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
