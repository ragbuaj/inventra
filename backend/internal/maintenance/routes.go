package maintenance

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts maintenance endpoints. Reads require maintenance.view;
// schedule/record writes require maintenance.manage; the Staf damage-report
// submit requires request.create. Per-asset history is under
// /assets/:id/maintenance.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage, requireView, requireCreate gin.HandlerFunc) {
	g := rg.Group("/maintenance")
	g.GET("/schedules", authMW, requireView, h.listSchedules)
	g.POST("/schedules", authMW, requireManage, h.createSchedule)
	g.PATCH("/schedules/:id", authMW, requireManage, h.updateSchedule)
	g.DELETE("/schedules/:id", authMW, requireManage, h.deleteSchedule)
	g.GET("/records", authMW, requireView, h.listRecords)
	g.POST("/records", authMW, requireManage, h.createRecord)
	g.GET("/records/:id", authMW, requireView, h.getRecord)
	g.PATCH("/records/:id", authMW, requireManage, h.updateRecord)
	g.GET("/attention", authMW, requireView, h.attention)
	g.POST("/reports", authMW, requireCreate, h.submitReport)

	rg.GET("/assets/:id/maintenance", authMW, requireView, h.listByAsset)
}
