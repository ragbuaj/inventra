package asset

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the asset endpoints.
// Reads require authMW + requireView; writes require authMW + requireManage.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireView, requireManage gin.HandlerFunc) {
	g := rg.Group("/assets")
	g.GET("", authMW, requireView, h.list)
	g.GET("/by-tag/:tag", authMW, requireView, h.getByTag)
	g.GET("/:id", authMW, requireView, h.get)
	g.GET("/:id/barcode", authMW, requireView, h.getBarcode)
	g.POST("/labels", authMW, requireView, h.generateLabels)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.GET("/:id/location-history", authMW, requireView, h.listLocationHistory)
	g.GET("/:id/pic-history", authMW, requireView, h.listPICHistory)

	a := g.Group("/:id/attachments")
	a.POST("", authMW, requireManage, h.uploadAttachment)
	a.GET("", authMW, requireView, h.listAttachments)
	a.GET("/:aid/content", authMW, requireView, h.downloadAttachment)
	a.GET("/:aid/thumbnail", authMW, requireView, h.downloadThumbnail)
	a.DELETE("/:aid", authMW, requireManage, h.deleteAttachment)

	d := g.Group("/:id/documents")
	d.POST("", authMW, requireManage, h.createDocument)
	d.GET("", authMW, requireView, h.listDocuments)
	d.GET("/:docId", authMW, requireView, h.getDocument)
	d.PUT("/:docId", authMW, requireManage, h.updateDocument)
	d.DELETE("/:docId", authMW, requireManage, h.deleteDocument)
	d.PUT("/:docId/file", authMW, requireManage, h.uploadDocumentFile)
	d.GET("/:docId/file", authMW, requireView, h.downloadDocumentFile)
}
