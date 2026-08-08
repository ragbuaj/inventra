package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/auth"
)

// OptionalAuth resolves the caller when a usable Bearer token is present and
// otherwise lets the request continue as a guest. It NEVER rejects.
//
// This exists for the one public-but-richer-when-signed-in surface in the app:
// the Panduan Penggunaan page is reachable without a session (see the frontend's
// publicPaths), its text is public, and its media is not. Serving both from one
// endpoint keeps the "guests see less" rule in a single serializer instead of
// splitting it across two endpoints and a client-side merge.
//
// Failing open to guest is the whole point, and it is a deliberate difference
// from RequireAuth. A stale, malformed, revoked, or expired token means "not
// signed in", not 401 — a 401 here would make the browser client clear its
// session and bounce a reader of a PUBLIC page to the login screen.
//
// Handlers behind this middleware MUST treat the absence of CtxUserID as
// "anonymous, show the public projection", never as "trusted". Anything that
// actually requires a session belongs behind RequireAuth instead.
func OptionalAuth(tm *auth.TokenManager, store *auth.TokenStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.Next()
			return
		}
		claims, err := tm.Parse(token)
		if err != nil || claims.Type != auth.TokenAccess {
			c.Next()
			return
		}
		if denied, err := store.AccessDenied(c.Request.Context(), claims.ID); err != nil || denied {
			c.Next()
			return
		}
		if claims.SID != "" {
			if alive, err := store.SessionAlive(c.Request.Context(), claims.SID); err != nil || !alive {
				c.Next()
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
