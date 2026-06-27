package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery converts a panic into a structured error log (with request_id) and a
// clean 500 JSON response, without leaking the stack to the client.
func Recovery(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				rid, _ := c.Get(CtxRequestID)
				base.With(slog.Any("request_id", rid)).Error("panic recovered",
					slog.String("error", fmt.Sprint(r)),
					slog.String("path", c.Request.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}
