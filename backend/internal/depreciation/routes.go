package depreciation

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the depreciation endpoints under rg, plus the
// asset-schedule read gated on requireAssetView and mounted on the SAME
// "/assets" path prefix the asset module uses (a second RouterGroup rooted
// at "/assets" from a different package is fine — Gin groups are just path
// prefixes, and the paths themselves don't collide).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView, requireAssetView gin.HandlerFunc) {
	g := rg.Group("/depreciation")
	g.GET("/periods", authMW, requireView, h.listPeriods)
	g.POST("/periods/:period/compute", authMW, requireManage, h.compute)
	g.POST("/periods/:period/close", authMW, requireManage, h.close)
	g.GET("/schedule", authMW, requireView, h.schedule)
	g.GET("/journal", authMW, requireView, h.journal)

	a := rg.Group("/assets")
	a.GET("/:id/depreciation", authMW, requireAssetView, h.assetSchedule)
}
