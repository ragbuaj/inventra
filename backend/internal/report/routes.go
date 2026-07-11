package report

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the reporting endpoints. Read-only module:
// report.view gates the JSON reads, report.export gates every file download.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireView, requireExport gin.HandlerFunc) {
	d := rg.Group("/dashboard")
	d.GET("/summary", authMW, requireView, h.dashboardSummary)
	d.GET("/export", authMW, requireExport, h.dashboardExport)

	r := rg.Group("/reports")
	r.GET("/:type", authMW, requireView, h.run)
	r.GET("/:type/export", authMW, requireExport, h.runExport)
}
