package transfer

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the transfer endpoints. Reads require transfer.view; writes
// require transfer.manage. Per-asset history is mounted under /assets/:id/transfers.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView gin.HandlerFunc) {
	g := rg.Group("/transfers")
	g.GET("", authMW, requireView, h.list)
	g.GET("/:id", authMW, requireView, h.get)
	g.POST("", authMW, requireManage, h.submit)
	g.POST("/:id/ship", authMW, requireManage, h.ship)
	g.POST("/:id/receive", authMW, requireManage, h.receive)

	rg.GET("/assets/:id/transfers", authMW, requireView, h.listByAsset)
}
