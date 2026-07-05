package approval

import (
	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// RegisterRoutes mounts all approval endpoints under rg.
// Permission keys: request.create, request.decide, approval.config.manage.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc, permSvc *authz.PermissionService) {
	create := middleware.RequirePermission(permSvc, "request.create")
	decide := middleware.RequirePermission(permSvc, "request.decide")
	cfg := middleware.RequirePermission(permSvc, "approval.config.manage")

	g := rg.Group("/requests")
	g.POST("", authMW, create, h.submit)
	g.GET("", authMW, h.list)
	g.GET("/inbox", authMW, decide, h.inbox)
	g.GET("/:id", authMW, h.get)
	g.POST("/:id/approve", authMW, decide, h.approve)
	g.POST("/:id/reject", authMW, decide, h.reject)
	g.POST("/:id/cancel", authMW, h.cancel)

	t := rg.Group("/approval-thresholds")
	t.GET("/preview", authMW, create, h.previewThresholds)
	t.GET("", authMW, cfg, h.listThresholds)
	t.POST("", authMW, cfg, h.createThreshold)
	t.PUT("/:id", authMW, cfg, h.updateThreshold)
	t.DELETE("/:id", authMW, cfg, h.deleteThreshold)
}
