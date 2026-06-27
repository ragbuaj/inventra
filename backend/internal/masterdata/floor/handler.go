package floor

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// scopeModule is the data_scope_policies module key floors resolve against.
const scopeModule = "offices"

type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
	aud    *audit.Service
}

func NewHandler(q *sqlc.Queries, scope *authz.ScopeService, aud *audit.Service) *Handler {
	return &Handler{svc: NewService(q), scoped: common.ScopedDeps{Q: q, Scope: scope}, aud: aud}
}

func (h *Handler) svcError(c *gin.Context, err error) {
	if errors.Is(err, ErrOfficeOutOfScope) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	common.WriteError(c, err)
}

func (h *Handler) list(c *gin.Context) {
	officeID, err := uuid.Parse(c.Query("office_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "office_id query parameter is required"})
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	search := c.Query("search")
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, total, err := h.svc.List(c.Request.Context(), all, ids, officeID, search, limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]Response, 0, len(rows))
	for _, f := range rows {
		data = append(data, toResponse(f))
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
	f, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(f))
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
	f, err := h.svc.Create(c.Request.Context(), all, ids, req.toInput())
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "floors", f.ID, &f.OfficeID, audit.Diff(nil, toResponse(f)))
	c.JSON(http.StatusCreated, toResponse(f))
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
	before, after, err := h.svc.Update(c.Request.Context(), id, all, ids, UpdateInput{CreateInput: req.toInput()})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "floors", after.ID, &after.OfficeID, audit.Diff(toResponse(before), toResponse(after)))
	c.JSON(http.StatusOK, toResponse(after))
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
	audit.Record(c, h.aud, audit.ActionDelete, "floors", id, &before.OfficeID, audit.Diff(toResponse(before), nil))
	c.Status(http.StatusNoContent)
}
