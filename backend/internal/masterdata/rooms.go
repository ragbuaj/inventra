package masterdata

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
)

type roomHandler struct {
	scopedDeps
}

type roomRequest struct {
	FloorID string  `json:"floor_id" binding:"required,uuid"`
	Name    string  `json:"name" binding:"required"`
	Code    *string `json:"code"`
}

type roomResponse struct {
	ID        string  `json:"id"`
	FloorID   string  `json:"floor_id"`
	Name      string  `json:"name"`
	Code      *string `json:"code"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

func toRoomResponse(r sqlc.MasterdataRoom) roomResponse {
	return roomResponse{
		ID:        r.ID.String(),
		FloorID:   r.FloorID.String(),
		Name:      r.Name,
		Code:      r.Code,
		CreatedAt: tsStr(r.CreatedAt),
		UpdatedAt: tsStr(r.UpdatedAt),
	}
}

// floorInScope reports whether the floor exists within the caller's office scope.
func (h *roomHandler) floorInScope(c *gin.Context, floorID uuid.UUID, all bool, ids []uuid.UUID) bool {
	_, err := h.q.GetFloor(c.Request.Context(), sqlc.GetFloorParams{ID: floorID, AllScope: all, OfficeIds: ids})
	return err == nil
}

func (h *roomHandler) list(c *gin.Context) {
	floorID, err := uuid.Parse(c.Query("floor_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "floor_id query parameter is required"})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if !h.floorInScope(c, floorID, all, ids) {
		c.JSON(http.StatusForbidden, gin.H{"error": "floor is outside your scope"})
		return
	}
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, err := h.q.ListRoomsByFloor(c.Request.Context(), sqlc.ListRoomsByFloorParams{FloorID: floorID, Search: search, Lim: limit, Off: offset})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list rooms"})
		return
	}
	total, err := h.q.CountRoomsByFloor(c.Request.Context(), sqlc.CountRoomsByFloorParams{FloorID: floorID, Search: search})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count rooms"})
		return
	}
	data := make([]roomResponse, 0, len(rows))
	for _, r := range rows {
		data = append(data, toRoomResponse(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *roomHandler) get(c *gin.Context) {
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
	r, err := h.q.GetRoom(c.Request.Context(), sqlc.GetRoomParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusOK, toRoomResponse(r))
}

func (h *roomHandler) create(c *gin.Context) {
	var req roomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	floorID := uuid.MustParse(req.FloorID)
	if !h.floorInScope(c, floorID, all, ids) {
		c.JSON(http.StatusForbidden, gin.H{"error": "floor is outside your scope"})
		return
	}
	r, err := h.q.CreateRoom(c.Request.Context(), sqlc.CreateRoomParams{FloorID: floorID, Name: req.Name, Code: req.Code})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusCreated, toRoomResponse(r))
}

func (h *roomHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req roomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	floorID := uuid.MustParse(req.FloorID)
	if !h.floorInScope(c, floorID, all, ids) {
		c.JSON(http.StatusForbidden, gin.H{"error": "floor is outside your scope"})
		return
	}
	r, err := h.q.UpdateRoom(c.Request.Context(), sqlc.UpdateRoomParams{
		FloorID: floorID, Name: req.Name, Code: req.Code, ID: id, AllScope: all, OfficeIds: ids,
	})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusOK, toRoomResponse(r))
}

func (h *roomHandler) delete(c *gin.Context) {
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
	n, err := h.q.SoftDeleteRoom(c.Request.Context(), sqlc.SoftDeleteRoomParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func registerRooms(rg *gin.RouterGroup, q *sqlc.Queries, scope *authz.ScopeService, authMW, requireManage gin.HandlerFunc) {
	h := &roomHandler{scopedDeps{q: q, scope: scope}}
	g := rg.Group("/rooms")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
