package authzadmin

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the authorization-admin endpoints under /authz.
// requireRole gates role + role_permissions, requireScope gates data scope,
// requireField gates field permissions. webOnly is the client-audience gate:
// the whole group is on ADR-0017's aud=mobile deny list, and the middleware
// must run AFTER authMW (it reads the audience RequireAuth put in the context).
//
// The catalog and role reads are loosened so scope.manage and fieldperm.manage
// can be delegated independently; every mutation stays strict:
//   - readCatalog gates GET /catalog on any one of role.manage, scope.manage, or
//     fieldperm.manage (the three authz-admin manage capabilities).
//   - readRoles gates GET /roles and GET /roles/:id on any one of those three
//     plus user.manage, so the Users screen can populate its role picker.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, webOnly, requireRole, requireScope, requireField, readCatalog, readRoles gin.HandlerFunc) {
	g := rg.Group("/authz", authMW, webOnly)
	g.GET("/catalog", readCatalog, h.catalog)

	g.GET("/roles", readRoles, h.listRoles)
	g.POST("/roles", requireRole, h.createRole)
	g.GET("/roles/:id", readRoles, h.getRole)
	g.PUT("/roles/:id", requireRole, h.updateRole)
	g.DELETE("/roles/:id", requireRole, h.deleteRole)

	g.GET("/roles/:id/permissions", requireRole, h.getPermissions)
	g.PUT("/roles/:id/permissions", requireRole, h.setPermissions)

	g.GET("/roles/:id/scope", requireScope, h.getScope)
	g.PUT("/roles/:id/scope", requireScope, h.setScope)

	g.GET("/roles/:id/fields", requireField, h.getFields)
	g.PUT("/roles/:id/fields", requireField, h.setFields)
}
