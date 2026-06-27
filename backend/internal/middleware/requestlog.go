package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/logging"
)

// CtxRequestID is the gin context key (and log attribute name) for the request id.
const CtxRequestID = "request_id"

// RequestHeaderID is the inbound/outbound correlation header.
const RequestHeaderID = "X-Request-ID"

// healthPaths are noisy probes excluded from request logging.
var healthPaths = map[string]struct{}{"/health": {}, "/health/ready": {}, "/api/v1/health": {}}

// RequestID reads an inbound X-Request-ID or generates one, stores it on the gin
// context, and echoes it in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestHeaderID)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(CtxRequestID, id)
		c.Writer.Header().Set(RequestHeaderID, id)
		c.Next()
	}
}

// RequestLogger binds a request-scoped logger (carrying request_id) into the
// request context and emits one structured line per request on completion.
// Level scales with status; /health probes are skipped.
func RequestLogger(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID, _ := c.Get(CtxRequestID)
		id, _ := reqID.(string)
		reqLog := base.With(slog.String("request_id", id))
		c.Request = c.Request.WithContext(logging.WithLogger(c.Request.Context(), reqLog))

		start := time.Now()
		// A panicked request is recovered by the Recovery middleware (registered
		// downstream). After recovery aborts with 500, execution returns here and
		// this completion line still fires — Recovery's panic line and this line
		// share request_id and are complementary: one carries the stack, the other
		// carries latency/status.
		c.Next()

		if _, skip := healthPaths[c.Request.URL.Path]; skip {
			return
		}
		status := c.Writer.Status()
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
		}
		if uid, ok := c.Get(CtxUserID); ok {
			attrs = append(attrs, slog.Any("user_id", uid))
		}
		if rid, ok := c.Get(CtxRoleID); ok {
			attrs = append(attrs, slog.Any("role_id", rid))
		}
		switch {
		case status >= 500:
			reqLog.Error("request", attrs...)
		case status >= 400:
			reqLog.Warn("request", attrs...)
		default:
			reqLog.Info("request", attrs...)
		}
	}
}
