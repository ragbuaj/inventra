package audit

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the read-only audit endpoints. Callers pass RequireAuth
// and RequirePermission("audit.view") as middleware.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, mws ...gin.HandlerFunc) {
	g := rg.Group("/audit", mws...)
	g.GET("", h.list)
}
