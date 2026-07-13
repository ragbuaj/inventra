package identity

import (
	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// RegisterRoutes mounts the identity endpoints. authMW protects authed routes;
// the limiter applies per-IP throttles on the unauthenticated auth endpoints.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc, limiter ratelimit.Allower, loginIPPerMin, refreshPerMin, googleIPPerMin, forgotPerMin int) {
	grp := rg.Group("/auth")
	grp.POST("/login", middleware.PerIP(limiter, loginIPPerMin, "auth_login", true), h.login)
	grp.POST("/refresh", middleware.PerIP(limiter, refreshPerMin, "auth_refresh", true), h.refresh)
	grp.GET("/google", middleware.PerIP(limiter, googleIPPerMin, "auth_google", true), h.googleStart)
	grp.GET("/google/callback", middleware.PerIP(limiter, googleIPPerMin, "auth_google", true), h.googleCallback)
	grp.POST("/password/forgot", middleware.PerIP(limiter, forgotPerMin, "auth_pwforgot", true), h.forgotPassword)
	grp.POST("/password/reset", middleware.PerIP(limiter, forgotPerMin, "auth_pwreset", true), h.resetPassword)

	authed := grp.Group("")
	authed.Use(authMW)
	authed.POST("/logout", h.logout)
	authed.GET("/me", h.me)
	authed.GET("/permissions", h.permissions)
	authed.GET("/scope/:module", h.scope)
	authed.PUT("/password", h.changePassword)
}
