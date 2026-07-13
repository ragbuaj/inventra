package office

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the offices endpoints. Reads are open to any authenticated
// user; writes require the office-manage permission (applied via requireManage).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage gin.HandlerFunc) {
	g := rg.Group("/offices")
	g.GET("/map", authMW, h.mapList)
	g.GET("/tree", authMW, h.tree)
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
