package report

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the reporting endpoints. Read-only module:
// report.view gates the JSON reads, report.export gates every file download.
//
// The two EXPORT routes also carry webOnly (RequireAudience(web)): ADR-0017
// keputusan 4 classifies report export as web-only, so aud=mobile is denied
// there. The JSON reads (dashboard summary + per-type report) stay shared —
// mobile may VIEW reports, it just may not export them. webOnly runs after
// authMW because it reads the audience RequireAuth sets on the context.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, webOnly, requireView, requireExport gin.HandlerFunc) {
	d := rg.Group("/dashboard")
	d.GET("/summary", authMW, requireView, h.dashboardSummary)
	d.GET("/export", authMW, webOnly, requireExport, h.dashboardExport)

	r := rg.Group("/reports")
	r.GET("/:type", authMW, requireView, h.run)
	r.GET("/:type/export", authMW, webOnly, requireExport, h.runExport)
}
