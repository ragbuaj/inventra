package assignment

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts assignment endpoints. Reads require assignment.view;
// direct check-out/check-in require assignment.manage; the Staf borrow submit,
// available-asset picker, and "mine" (own held-assets) picker require
// request.create — not assignment.view, which Staf never receives (see
// 000026's data-scope decision). Per-asset history is under
// /assets/:id/assignments.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView, requireCreate gin.HandlerFunc) {
	g := rg.Group("/assignments")
	g.GET("", authMW, requireView, h.list)
	g.GET("/available", authMW, requireCreate, h.available)
	g.GET("/mine", authMW, requireCreate, h.mine)
	g.GET("/:id", authMW, requireView, h.get)
	g.POST("", authMW, requireManage, h.checkout)
	g.POST("/borrow", authMW, requireCreate, h.borrow)
	g.POST("/:id/checkin", authMW, requireManage, h.checkin)

	rg.GET("/assets/:id/assignments", authMW, requireView, h.listByAsset)
}
