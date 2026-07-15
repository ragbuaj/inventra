package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/authz"
)

// RequirePermission gates a route on a per-action RBAC permission key.
// It must run after RequireAuth (which populates the role in the context).
func RequirePermission(checker authz.PermissionChecker, key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID, err := uuid.Parse(c.GetString(CtxRoleID))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing role"})
			return
		}
		ok, err := checker.Has(c.Request.Context(), roleID, key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authorization check failed"})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permission": key})
			return
		}
		c.Next()
	}
}

// RequireAnyPermission gates a route on holding at least one of several RBAC
// permission keys (logical OR). It is used to loosen read gates so a capability
// can be delegated independently (for example scope.manage or fieldperm.manage)
// without granting role.manage. Mutations must stay on the strict single-key
// RequirePermission. Like RequirePermission it must run after RequireAuth.
func RequireAnyPermission(checker authz.PermissionChecker, keys ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID, err := uuid.Parse(c.GetString(CtxRoleID))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing role"})
			return
		}
		for _, key := range keys {
			ok, err := checker.Has(c.Request.Context(), roleID, key)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authorization check failed"})
				return
			}
			if ok {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permission_any": keys})
	}
}
