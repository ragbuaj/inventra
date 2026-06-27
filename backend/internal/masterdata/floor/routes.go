package floor

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the floors endpoints. Reads are open to any authenticated
// user; writes require the office-manage permission.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage gin.HandlerFunc) {
	g := rg.Group("/floors")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
