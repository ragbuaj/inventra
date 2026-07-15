package authzadmin

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the authorization-admin endpoints under /authz.
// requireRole gates role + role_permissions, requireScope gates data scope,
// requireField gates field permissions.
//
// The catalog and role reads are loosened so scope.manage and fieldperm.manage
// can be delegated independently; every mutation stays strict:
//   - readCatalog gates GET /catalog on any one of role.manage, scope.manage, or
//     fieldperm.manage (the three authz-admin manage capabilities).
//   - readRoles gates GET /roles and GET /roles/:id on any one of those three
//     plus user.manage, so the Users screen can populate its role picker.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireRole, requireScope, requireField, readCatalog, readRoles gin.HandlerFunc) {
	g := rg.Group("/authz")
	g.GET("/catalog", authMW, readCatalog, h.catalog)

	g.GET("/roles", authMW, readRoles, h.listRoles)
	g.POST("/roles", authMW, requireRole, h.createRole)
	g.GET("/roles/:id", authMW, readRoles, h.getRole)
	g.PUT("/roles/:id", authMW, requireRole, h.updateRole)
	g.DELETE("/roles/:id", authMW, requireRole, h.deleteRole)

	g.GET("/roles/:id/permissions", authMW, requireRole, h.getPermissions)
	g.PUT("/roles/:id/permissions", authMW, requireRole, h.setPermissions)

	g.GET("/roles/:id/scope", authMW, requireScope, h.getScope)
	g.PUT("/roles/:id/scope", authMW, requireScope, h.setScope)

	g.GET("/roles/:id/fields", authMW, requireField, h.getFields)
	g.PUT("/roles/:id/fields", authMW, requireField, h.setFields)
}
