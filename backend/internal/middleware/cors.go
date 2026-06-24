package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS allows browser (XHR/fetch) requests from the configured frontend origin.
// The SPA frontend and the API run on different origins in development
// (http://localhost:3000 vs http://localhost:8080), so without these headers the
// browser blocks every cross-origin call — including login — with a CORS error.
//
// Only the exact configured origin is allowed (echoed back, as required when
// Access-Control-Allow-Credentials is true). Preflight OPTIONS requests are
// answered here with 204 and never reach the route handlers.
func CORS(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" && origin == allowedOrigin {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
