package guide

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the guide endpoints.
//
// Three different access rules live here, and the split is the feature:
//
//   - The module list is behind optionalAuth: readable with no session at all
//     (the page is public), richer once the caller has one.
//   - The PDF stream is behind authMW alone: any signed-in user may read guide
//     media, no extra permission needed.
//   - Every write is behind authMW + requireManage + webOnly. Authoring is an
//     admin surface and, like authzadmin and importer, is not reachable from a
//     mobile token (ADR-0017).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, optionalAuth, requireManage, webOnly gin.HandlerFunc) {
	g := rg.Group("/guide")

	g.GET("/modules", optionalAuth, h.listModules)
	g.GET("/attachments/:aid/content", authMW, h.downloadAttachment)

	admin := g.Group("", authMW, webOnly, requireManage)
	admin.GET("/modules/:id", h.getModule)
	admin.POST("/modules", h.createModule)
	admin.PATCH("/modules/:id", h.updateModule)
	admin.DELETE("/modules/:id", h.deleteModule)
	admin.POST("/modules/:id/attachments/video", h.addVideo)
	admin.POST("/modules/:id/attachments/document", h.addDocument)
	admin.PATCH("/attachments/:aid", h.patchAttachment)
	admin.DELETE("/attachments/:aid", h.deleteAttachment)
}
