package authzadmin

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the authorization-admin endpoints under /authz.
// requireRole gates role + role_permissions, requireScope gates data scope,
// requireField gates field permissions.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireRole, requireScope, requireField gin.HandlerFunc) {
	g := rg.Group("/authz")
	g.GET("/catalog", authMW, requireRole, h.catalog)

	g.GET("/roles", authMW, requireRole, h.listRoles)
	g.POST("/roles", authMW, requireRole, h.createRole)
	g.GET("/roles/:id", authMW, requireRole, h.getRole)
	g.PUT("/roles/:id", authMW, requireRole, h.updateRole)
	g.DELETE("/roles/:id", authMW, requireRole, h.deleteRole)

	g.GET("/roles/:id/permissions", authMW, requireRole, h.getPermissions)
	g.PUT("/roles/:id/permissions", authMW, requireRole, h.setPermissions)

	g.GET("/roles/:id/scope", authMW, requireScope, h.getScope)
	g.PUT("/roles/:id/scope", authMW, requireScope, h.setScope)

	g.GET("/roles/:id/fields", authMW, requireField, h.getFields)
	g.PUT("/roles/:id/fields", authMW, requireField, h.setFields)
}
