package masterdata

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
)

type floorHandler struct {
	scopedDeps
}

type floorRequest struct {
	OfficeID string `json:"office_id" binding:"required,uuid"`
	Name     string `json:"name" binding:"required"`
	Level    *int32 `json:"level"`
}

type floorResponse struct {
	ID        string  `json:"id"`
	OfficeID  string  `json:"office_id"`
	Name      string  `json:"name"`
	Level     *int32  `json:"level"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

func toFloorResponse(f sqlc.MasterdataFloor) floorResponse {
	return floorResponse{
		ID:        f.ID.String(),
		OfficeID:  f.OfficeID.String(),
		Name:      f.Name,
		Level:     f.Level,
		CreatedAt: tsStr(f.CreatedAt),
		UpdatedAt: tsStr(f.UpdatedAt),
	}
}

func (h *floorHandler) list(c *gin.Context) {
	officeID, err := uuid.Parse(c.Query("office_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "office_id query parameter is required"})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if !inScope(all, ids, officeID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "office is outside your scope"})
		return
	}
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, err := h.q.ListFloorsByOffice(c.Request.Context(), sqlc.ListFloorsByOfficeParams{OfficeID: officeID, Search: search, Lim: limit, Off: offset})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list floors"})
		return
	}
	total, err := h.q.CountFloorsByOffice(c.Request.Context(), sqlc.CountFloorsByOfficeParams{OfficeID: officeID, Search: search})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count floors"})
		return
	}
	data := make([]floorResponse, 0, len(rows))
	for _, f := range rows {
		data = append(data, toFloorResponse(f))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *floorHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	f, err := h.q.GetFloor(c.Request.Context(), sqlc.GetFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusOK, toFloorResponse(f))
}

func (h *floorHandler) create(c *gin.Context) {
	var req floorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	officeID := uuid.MustParse(req.OfficeID)
	if !inScope(all, ids, officeID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "office is outside your scope"})
		return
	}
	f, err := h.q.CreateFloor(c.Request.Context(), sqlc.CreateFloorParams{OfficeID: officeID, Name: req.Name, Level: req.Level})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "floors", f.ID, &f.OfficeID, audit.Diff(nil, toFloorResponse(f)))
	c.JSON(http.StatusCreated, toFloorResponse(f))
}

func (h *floorHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req floorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	officeID := uuid.MustParse(req.OfficeID)
	if !inScope(all, ids, officeID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "office is outside your scope"})
		return
	}
	cur, err := h.q.GetFloor(c.Request.Context(), sqlc.GetFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	f, err := h.q.UpdateFloor(c.Request.Context(), sqlc.UpdateFloorParams{
		OfficeID: officeID, Name: req.Name, Level: req.Level, ID: id, AllScope: all, OfficeIds: ids,
	})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "floors", f.ID, &f.OfficeID, audit.Diff(toFloorResponse(cur), toFloorResponse(f)))
	c.JSON(http.StatusOK, toFloorResponse(f))
}

func (h *floorHandler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	cur, err := h.q.GetFloor(c.Request.Context(), sqlc.GetFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	n, err := h.q.SoftDeleteFloor(c.Request.Context(), sqlc.SoftDeleteFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "floors", id, &cur.OfficeID, audit.Diff(toFloorResponse(cur), nil))
	c.Status(http.StatusNoContent)
}

func registerFloors(rg *gin.RouterGroup, q *sqlc.Queries, scope *authz.ScopeService, aud *audit.Service, authMW, requireManage gin.HandlerFunc) {
	h := &floorHandler{scopedDeps{q: q, scope: scope, aud: aud}}
	g := rg.Group("/floors")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
