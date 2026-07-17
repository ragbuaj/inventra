package notification

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts the notification endpoints under rg. No permission key
// gates them: the feed is per-user, and every handler scopes its query to the
// authenticated caller's own rows.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc) {
	g := rg.Group("/notifications", authMW)
	g.GET("", h.list)
	g.GET("/unread-count", h.unreadCount)
	g.POST("/read-all", h.markAllRead)
	g.POST("/:id/read", h.markRead)
}
