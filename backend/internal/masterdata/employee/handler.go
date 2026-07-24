package employee

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// scopeModule is the data_scope_policies module key for employees.
const scopeModule = "employees"

type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
	fields *authz.FieldService
	aud    *audit.Service
}

func NewHandler(q *sqlc.Queries, scope *authz.ScopeService, aud *audit.Service, fieldSvc *authz.FieldService) *Handler {
	return &Handler{svc: NewService(q), scoped: common.ScopedDeps{Q: q, Scope: scope}, fields: fieldSvc, aud: aud}
}

func (h *Handler) svcError(c *gin.Context, err error) {
	if errors.Is(err, ErrOfficeOutOfScope) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, ErrDepartmentOfficeMismatch) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	common.WriteError(c, err)
}

// filterMap applies field-permission masking for the caller's role on the
// "employees" entity. It delegates to authz.FilterEntity, which fails
// closed: a policy-lookup error (e.g. Redis down) is returned so callers
// refuse to leak unfiltered employee data rather than serving it unmasked.
// An unparseable/missing role id (CtxRoleID) is treated the same way — the
// caller responds 500 instead of falling back to a default-allow uuid.Nil
// lookup that could serve the record unmasked.
func (h *Handler) filterMap(c *gin.Context, m map[string]any) (map[string]any, error) {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return nil, err
	}
	if err := h.fields.FilterEntity(c.Request.Context(), roleID, "employees", m); err != nil {
		return nil, err
	}
	return m, nil
}

func (h *Handler) list(c *gin.Context) {
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	search := c.Query("search")
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, total, err := h.svc.List(c.Request.Context(), all, ids, search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list employees"})
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, e := range rows {
		masked, err := h.filterMap(c, employeeToMap(e))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
			return
		}
		data = append(data, masked)
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	e, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	masked, err := h.filterMap(c, employeeToMap(e))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return
	}
	c.JSON(http.StatusOK, masked)
}

func (h *Handler) create(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	in, err := req.toInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	e, err := h.svc.Create(c.Request.Context(), all, ids, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "employees", e.ID, &e.OfficeID, audit.Diff(nil, toResponse(e)))
	masked, err := h.filterMap(c, employeeToMap(e))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return
	}
	c.JSON(http.StatusCreated, masked)
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	in, err := req.toInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, after, err := h.svc.Update(c.Request.Context(), id, all, ids, UpdateInput{CreateInput: in})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "employees", after.ID, &after.OfficeID, audit.Diff(toResponse(before), toResponse(after)))
	masked, err := h.filterMap(c, employeeToMap(after))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return
	}
	c.JSON(http.StatusOK, masked)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	before, err := h.svc.Delete(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "employees", id, &before.OfficeID, audit.Diff(toResponse(before), nil))
	c.Status(http.StatusNoContent)
}
