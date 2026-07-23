package department

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the departments endpoints at /departments (the same path
// the generic reference engine used, so the frontend is unchanged). Reads are open
// to any authenticated user but data-scoped; writes require the office-manage
// permission and are scope-enforced.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireManage gin.HandlerFunc) {
	g := rg.Group("/departments")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
