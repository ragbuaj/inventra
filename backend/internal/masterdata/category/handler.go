package category

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Handler maps HTTP ↔ the category service and records audit entries.
type Handler struct {
	svc *Service
	aud *audit.Service
}

func NewHandler(q *sqlc.Queries, aud *audit.Service) *Handler {
	return &Handler{svc: NewService(q), aud: aud}
}

func (h *Handler) tree(c *gin.Context) {
	rows, err := h.svc.Tree(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list category tree"})
		return
	}
	data := make([]Response, 0, len(rows))
	for _, cat := range rows {
		data = append(data, toResponse(cat))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) list(c *gin.Context) {
	search := c.Query("search")
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, total, err := h.svc.List(c.Request.Context(), search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list categories"})
		return
	}
	data := make([]Response, 0, len(rows))
	for _, cat := range rows {
		data = append(data, toResponse(cat))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	cat, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(cat))
}

func (h *Handler) create(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := req.toInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cat, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "categories", cat.ID, nil, audit.Diff(nil, toResponse(cat)))
	c.JSON(http.StatusCreated, toResponse(cat))
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
	in, err := req.toInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, after, err := h.svc.Update(c.Request.Context(), id, UpdateInput{CreateInput: in})
	if err != nil {
		common.WriteError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "categories", after.ID, nil, audit.Diff(toResponse(before), toResponse(after)))
	c.JSON(http.StatusOK, toResponse(after))
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	before, err := h.svc.Delete(c.Request.Context(), id)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "categories", id, nil, audit.Diff(toResponse(before), nil))
	c.Status(http.StatusNoContent)
}
