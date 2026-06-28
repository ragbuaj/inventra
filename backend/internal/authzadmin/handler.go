package authzadmin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/audit"
)

// Handler exposes the authorization-admin HTTP endpoints.
type Handler struct {
	svc *Service
	aud *audit.Service
}

func NewHandler(svc *Service, aud *audit.Service) *Handler { return &Handler{svc: svc, aud: aud} }

func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrSystemRole), errors.Is(err, ErrRoleInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUnknownPermission), errors.Is(err, ErrValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func (h *Handler) catalog(c *gin.Context) { c.JSON(http.StatusOK, CatalogResponse()) }

func (h *Handler) listRoles(c *gin.Context) {
	rows, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, roleToMap(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) getRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	r, err := h.svc.GetRole(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, roleToMap(r))
}

func (h *Handler) createRole(c *gin.Context) {
	var req roleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r, err := h.svc.CreateRole(c.Request.Context(), RoleInput{Code: req.Code, Name: req.Name, Description: req.Description})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "roles", r.ID, nil, audit.Diff(nil, roleToMap(r)))
	c.JSON(http.StatusCreated, roleToMap(r))
}

func (h *Handler) updateRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req roleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, after, err := h.svc.UpdateRole(c.Request.Context(), id, RoleInput{Code: req.Code, Name: req.Name, Description: req.Description})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "roles", after.ID, nil, audit.Diff(roleToMap(before), roleToMap(after)))
	c.JSON(http.StatusOK, roleToMap(after))
}

func (h *Handler) deleteRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	r, err := h.svc.DeleteRole(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "roles", id, nil, audit.Diff(roleToMap(r), nil))
	c.Status(http.StatusNoContent)
}

func (h *Handler) getPermissions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	keys, err := h.svc.GetRolePermissions(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": keys})
}

func (h *Handler) setPermissions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req permissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, _ := h.svc.GetRolePermissions(c.Request.Context(), id)
	if err := h.svc.SetRolePermissions(c.Request.Context(), id, req.Permissions); err != nil {
		h.svcError(c, err)
		return
	}
	after, _ := h.svc.GetRolePermissions(c.Request.Context(), id)
	audit.Record(c, h.aud, audit.ActionUpdate, "role_permissions", id, nil,
		audit.Diff(map[string]any{"permissions": before}, map[string]any{"permissions": after}))
	c.JSON(http.StatusOK, gin.H{"permissions": after})
}

func (h *Handler) getScope(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.svc.GetScopePolicies(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		data = append(data, scopePolicyToMap(p))
	}
	c.JSON(http.StatusOK, gin.H{"policies": data})
}

func (h *Handler) setScope(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req scopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetScopePolicies(c.Request.Context(), id, req.toInputs()); err != nil {
		h.svcError(c, err)
		return
	}
	rows, _ := h.svc.GetScopePolicies(c.Request.Context(), id)
	data := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		data = append(data, scopePolicyToMap(p))
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "data_scope_policies", id, nil, audit.Diff(nil, map[string]any{"policies": data}))
	c.JSON(http.StatusOK, gin.H{"policies": data})
}

func (h *Handler) getFields(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	rows, err := h.svc.GetFieldPermissions(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		data = append(data, fieldPermToMap(f))
	}
	c.JSON(http.StatusOK, gin.H{"fields": data})
}

func (h *Handler) setFields(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req fieldsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.SetFieldPermissions(c.Request.Context(), id, req.toInputs()); err != nil {
		h.svcError(c, err)
		return
	}
	rows, _ := h.svc.GetFieldPermissions(c.Request.Context(), id)
	data := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		data = append(data, fieldPermToMap(f))
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "field_permissions", id, nil, audit.Diff(nil, map[string]any{"fields": data}))
	c.JSON(http.StatusOK, gin.H{"fields": data})
}
