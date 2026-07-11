package search

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the global search endpoint. Auth only — per-entity
// permission/scope gating happens inside the handler (groups are skipped,
// never 403'd).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc) {
	rg.GET("/search", authMW, h.search)
}
