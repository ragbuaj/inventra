package user

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the user-management endpoints under /users.
// The middlewares (auth + RequirePermission) gate the whole group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, mws ...gin.HandlerFunc) {
	g := rg.Group("/users")
	g.Use(mws...)
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PUT("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.POST("/:id/reset-password", h.resetPassword)
}
