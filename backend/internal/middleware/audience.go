package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAudience gates a route on the caller's token audience (CtxAudience,
// set by RequireAuth — it MUST run after it). It is the enforcement point for
// ADR-0017's endpoint classification: web-only/admin groups mount
// RequireAudience(auth.AudienceWeb) (equivalent to the ADR's "deny aud=mobile"
// and fail-closed for any future audience), and mobile-only endpoints will
// mount RequireAudience(auth.AudienceMobile).
//
// An absent context audience is NOT treated as web: RequireAuth always sets
// CtxAudience (ClientAudience() maps a no-aud legacy token to web), so an empty
// value here can only mean this middleware was mounted without RequireAuth in
// front of it — a wiring bug, not a legacy session. Failing closed with 401
// surfaces the misconfiguration instead of silently granting web access. The
// legacy-token-is-web rule lives in ClientAudience(), the single place that
// reads the raw claim; it must not be duplicated here off an empty context.
//
// The audience is a blast-radius limiter, not a permission — routes still need
// their own RequirePermission/scope checks.
func RequireAudience(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		aud := c.GetString(CtxAudience)
		if aud == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing audience (RequireAudience mounted without RequireAuth)"})
			return
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
