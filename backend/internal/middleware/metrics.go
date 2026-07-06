package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ragbuaj/inventra/internal/observability"
)

// Metrics records RED metrics (rate, errors, duration) per request. The route
// label uses the matched route template (c.FullPath()) to bound cardinality;
// unmatched routes and the /metrics endpoint are skipped.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" || route == "/metrics" {
			return
		}
		observability.RequestDuration.WithLabelValues(c.Request.Method, route).
			Observe(time.Since(start).Seconds())
		observability.RequestsTotal.WithLabelValues(
			c.Request.Method, route, strconv.Itoa(c.Writer.Status()),
		).Inc()
	}
}
