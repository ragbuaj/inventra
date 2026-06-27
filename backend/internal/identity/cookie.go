package identity

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// refreshCookieName is the HttpOnly cookie that carries the refresh token.
const refreshCookieName = "inventra_refresh"

// refreshCookiePath scopes the cookie to the auth endpoints only, so the
// long-lived refresh token never travels to business endpoints.
const refreshCookiePath = "/api/v1/auth"

// setRefreshCookie writes the refresh token as an HttpOnly, SameSite=Lax cookie.
// secure (TLS-only) is enabled in production; dev/CI run over plain HTTP.
// SameSite=Lax suffices for same-site (incl. localhost and same-registrable-domain
// subdomains); a genuinely cross-site frontend/API deployment would need SameSite=None; Secure.
func setRefreshCookie(c *gin.Context, token string, ttl time.Duration, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, token, int(ttl.Seconds()), refreshCookiePath, "", secure, true)
}

// clearRefreshCookie expires the refresh cookie (logout).
func clearRefreshCookie(c *gin.Context, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", secure, true)
}
