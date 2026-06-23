package identity

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the identity endpoints under the given group.
// authMW protects endpoints that require a valid access token.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc) {
	grp := rg.Group("/auth")
	grp.POST("/login", h.login)
	grp.POST("/refresh", h.refresh)

	authed := grp.Group("")
	authed.Use(authMW)
	authed.POST("/logout", h.logout)
	authed.GET("/me", h.me)
}
