package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// NoStore marks every response under prefix as uncacheable.
//
// The whole of /api/v1 is authenticated bank asset data, so none of it may be
// written to the browser's HTTP cache — that cache is a second door to the same
// device disk the PWA spec (docs/superpowers/specs/2026-08-27-web-pwa-design.md,
// keputusan 2) already closes for Cache Storage by shipping a service worker with
// zero runtime caching rules. An installed PWA is long-lived on personal and
// shared branch devices, which widens the exposure window. There is no endpoint
// under /api/v1 that wants to be cached: every handler that sets the header today
// already sets no-store (guide, avatar), so the rule is blanket with no exemptions.
//
// It is mounted globally rather than on the /api/v1 route group, and gates on the
// request path, because group middleware never runs for a request that matches no
// route: a 404 or 405 under /api/v1 would escape it. Gating here covers those, the
// rate limiter's 429, and Recovery's 500 alike — "any status code", as the
// invariant requires.
//
// The header is set BEFORE the handler runs, so a handler that wants to be more
// specific still wins: gin's c.Header uses Set semantics, so the avatar and guide
// handlers' "private, no-store" replaces this value instead of appending to it.
func NoStore(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	}
}
