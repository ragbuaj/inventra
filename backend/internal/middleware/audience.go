package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/auth"
)

// RequireAudience gates a route on the caller's token audience (CtxAudience,
// set by RequireAuth — it must run after it). It is the enforcement point for
// ADR-0017's endpoint classification: web-only/admin groups mount
// RequireAudience(auth.AudienceWeb) (equivalent to the ADR's "deny aud=mobile"
// and fail-closed for any future audience), and mobile-only endpoints will
// mount RequireAudience(auth.AudienceMobile).
//
// An absent context value is treated as web, mirroring the token rule (tokens
// minted before audiences existed carry no aud and are web sessions). The
// audience is a blast-radius limiter, not a permission — routes still need
// their own RequirePermission/scope checks.
func RequireAudience(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		aud := c.GetString(CtxAudience)
		if aud == "" {
			aud = auth.AudienceWeb
		}
		for _, a := range allowed {
			if aud == a {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "allowed_audience": allowed})
	}
}
