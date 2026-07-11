package search

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

type Handler struct {
	svc    *Service
	perms  *authz.PermissionService
	scoped common.ScopedDeps
}

func NewHandler(svc *Service, perms *authz.PermissionService, scoped common.ScopedDeps) *Handler {
	return &Handler{svc: svc, perms: perms, scoped: scoped}
}

// gateScoped resolves one entity gate: optional permission key + scope module.
// A missing permission disables the group silently (never a 403).
func (h *Handler) gateScoped(c *gin.Context, roleID uuid.UUID, permKey, module string) (Gate, error) {
	if permKey != "" {
		ok, err := h.perms.Has(c.Request.Context(), roleID, permKey)
		if err != nil || !ok {
			return Gate{}, err
		}
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, module)
	if err != nil {
		return Gate{}, err
	}
	return Gate{Enabled: true, AllScope: all, OfficeIDs: ids}, nil
}

func (h *Handler) search(c *gin.Context) {
	q := c.Query("q")
	if TooShort(q) {
		c.JSON(http.StatusOK, gin.H{"groups": []Group{}})
		return
	}
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	in := Input{Q: q}
	if in.Assets, err = h.gateScoped(c, roleID, "asset.view", "assets"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Employees, err = h.gateScoped(c, roleID, "", "employees"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Offices, err = h.gateScoped(c, roleID, "", "offices"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Requests, err = h.gateScoped(c, roleID, "", "requests"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Users, err = h.perms.Has(c.Request.Context(), roleID, "user.manage"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve permissions"})
		return
	}

	groups, err := h.svc.Search(c.Request.Context(), in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}
