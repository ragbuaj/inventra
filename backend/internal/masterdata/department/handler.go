package department

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

// scopeModule is the data_scope_policies module key for departments.
const scopeModule = "departments"

// Handler maps HTTP to the department service and records audit entries.
type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
	aud    *audit.Service
}

// NewHandler builds the department handler with its service + scope resolver.
func NewHandler(q *sqlc.Queries, scope *authz.ScopeService, aud *audit.Service) *Handler {
	return &Handler{
		svc:    NewService(q),
		scoped: common.ScopedDeps{Q: q, Scope: scope},
		aud:    aud,
	}
}

// svcError routes service sentinel errors to the right HTTP status.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrOfficeOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrFloorOfficeMismatch), errors.Is(err, ErrOfficeRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list departments"})
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		data = append(data, toResponse(d))
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
	d, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(d))
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
	d, err := h.svc.Create(c.Request.Context(), all, ids, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "departments", d.ID, &d.ID, audit.Diff(nil, toResponse(d)))
	c.JSON(http.StatusCreated, toResponse(d))
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
	audit.Record(c, h.aud, audit.ActionUpdate, "departments", after.ID, &after.ID, audit.Diff(toResponse(before), toResponse(after)))
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
	audit.Record(c, h.aud, audit.ActionDelete, "departments", id, &id, audit.Diff(toResponse(before), nil))
	c.Status(http.StatusNoContent)
}
