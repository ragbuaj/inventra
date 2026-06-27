package category

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the categories endpoints. Reads are open to any
// authenticated user; writes require the global-manage permission.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage gin.HandlerFunc) {
	g := rg.Group("/categories")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
