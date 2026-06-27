package middleware

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/logging"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// PerIP limits requests per client IP. prefix namespaces the key; withBackstop
// enables the in-memory fallback (auth paths). It sets RateLimit-* on allow and
// returns 429 (+ Retry-After) on deny.
func PerIP(l ratelimit.Allower, perMin int, prefix string, withBackstop bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := prefix + ":ip:" + c.ClientIP()
		res := l.Allow(c.Request.Context(), key, perMin, withBackstop)
		if !res.Allowed {
			logging.FromContext(c.Request.Context()).Warn("rate limit exceeded", "prefix", prefix, "ip", c.ClientIP())
			WriteRateLimited(c, res)
			return
		}
		SetRateLimitHeaders(c, res)
		c.Next()
	}
}

// SetRateLimitHeaders writes the IETF draft RateLimit-* response headers.
func SetRateLimitHeaders(c *gin.Context, res ratelimit.Result) {
	h := c.Writer.Header()
	h.Set("RateLimit-Limit", strconv.Itoa(res.Limit))
	remaining := res.Remaining
	if remaining < 0 {
		remaining = 0
	}
	h.Set("RateLimit-Remaining", strconv.Itoa(remaining))
	h.Set("RateLimit-Reset", strconv.Itoa(int(math.Ceil(res.ResetAfter.Seconds()))))
}

// WriteRateLimited aborts with 429 + Retry-After (≥1s) + RateLimit-* headers.
// Shared by PerIP and the login account-key check so the 429 shape is identical.
func WriteRateLimited(c *gin.Context, res ratelimit.Result) {
	SetRateLimitHeaders(c, res)
	retry := int(math.Ceil(res.RetryAfter.Seconds()))
	if retry < 1 {
		retry = 1
	}
	c.Writer.Header().Set("Retry-After", strconv.Itoa(retry))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
}
