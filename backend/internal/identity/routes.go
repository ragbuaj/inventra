package identity

import (
	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// RegisterRoutes mounts the identity endpoints. authMW protects authed routes;
// the limiter applies per-IP throttles on the unauthenticated auth endpoints.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc, limiter ratelimit.Allower, loginIPPerMin, refreshPerMin int) {
	grp := rg.Group("/auth")
	grp.POST("/login", middleware.PerIP(limiter, loginIPPerMin, "auth_login", true), h.login)
	grp.POST("/refresh", middleware.PerIP(limiter, refreshPerMin, "auth_refresh", true), h.refresh)

	authed := grp.Group("")
	authed.Use(authMW)
	authed.POST("/logout", h.logout)
	authed.GET("/me", h.me)
	authed.GET("/permissions", h.permissions)
	authed.GET("/scope/:module", h.scope)
}
