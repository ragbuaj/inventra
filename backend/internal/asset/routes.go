package asset

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the asset endpoints.
// Reads require authMW + requireView; writes require authMW + requireManage.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireView, requireManage gin.HandlerFunc) {
	g := rg.Group("/assets")
	g.GET("", authMW, requireView, h.list)
	g.GET("/:id", authMW, requireView, h.get)
	g.PUT("/:id", authMW, requireManage, h.update)

	a := g.Group("/:id/attachments")
	a.POST("", authMW, requireManage, h.uploadAttachment)
	a.GET("", authMW, requireView, h.listAttachments)
	a.GET("/:aid/content", authMW, requireView, h.downloadAttachment)
	a.GET("/:aid/thumbnail", authMW, requireView, h.downloadThumbnail)
	a.DELETE("/:aid", authMW, requireManage, h.deleteAttachment)
}
