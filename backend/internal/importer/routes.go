package importer

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the bulk-import endpoints under /imports. Unlike most
// modules there is no single static RequirePermission middleware here — the
// guarding permission depends on the request's target (asset/employee/
// office/reference:*), which is only known once the handler resolves it (from
// the query/form field, or from the loaded job), so every handler performs
// its own per-target permission check (see Handler.checkTargetPermission).
//
// webOnly is the client-audience gate: the importer is on ADR-0017's
// aud=mobile deny list; it must run after authMW.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, webOnly gin.HandlerFunc) {
	g := rg.Group("/imports", authMW, webOnly)
	g.GET("/template", h.template)
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.GET("/:id/rows", h.rows)
	g.POST("/:id/confirm", h.confirm)
	g.POST("/:id/cancel", h.cancel)
	g.GET("/:id/error-report", h.errorReport)
}
