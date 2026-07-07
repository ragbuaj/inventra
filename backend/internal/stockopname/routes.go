package stockopname

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the stock-opname endpoints. Reads require
// stockopname.view; writes require stockopname.manage.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView gin.HandlerFunc) {
	g := rg.Group("/stock-opname/sessions")
	g.GET("", authMW, requireView, h.list)
	g.POST("", authMW, requireManage, h.create)
	g.GET("/:id", authMW, requireView, h.get)
	g.GET("/:id/items", authMW, requireView, h.listItems)
	g.POST("/:id/start", authMW, requireManage, h.start)
	g.POST("/:id/scan", authMW, requireManage, h.scan)
	g.PATCH("/:id/items/:itemId", authMW, requireManage, h.setResult)
	g.POST("/:id/reconcile", authMW, requireManage, h.reconcile)
	g.POST("/:id/items/:itemId/follow-up", authMW, requireManage, h.followup)
	g.POST("/:id/close", authMW, requireManage, h.close)
	g.GET("/:id/report", authMW, requireView, h.report)
}
