package disposal

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the disposal endpoints. Reads require disposal.view; writes
// require disposal.manage. Per-asset history is mounted under /assets/:id/disposal.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView gin.HandlerFunc) {
	g := rg.Group("/disposals")
	g.GET("", authMW, requireView, h.list)
	g.GET("/:id", authMW, requireView, h.get)
	g.POST("", authMW, requireManage, h.submit)
	g.POST("/:id/document", authMW, requireManage, h.attachDocument)

	rg.GET("/assets/:id/disposal", authMW, requireView, h.listByAsset)
}
