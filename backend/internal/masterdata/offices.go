package masterdata

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
)

type officeHandler struct {
	scopedDeps
}

type officeRequest struct {
	ParentID     *string `json:"parent_id" binding:"omitempty,uuid"`
	OfficeTypeID string  `json:"office_type_id" binding:"required,uuid"`
	ProvinceID   *string `json:"province_id" binding:"omitempty,uuid"`
	CityID       *string `json:"city_id" binding:"omitempty,uuid"`
	Name         string  `json:"name" binding:"required"`
	Code         string  `json:"code" binding:"required"`
	Address      *string `json:"address"`
	IsActive     *bool   `json:"is_active"`
}

type officeResponse struct {
	ID           string  `json:"id"`
	ParentID     *string `json:"parent_id"`
	OfficeTypeID string  `json:"office_type_id"`
	ProvinceID   *string `json:"province_id"`
	CityID       *string `json:"city_id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Address      *string `json:"address"`
	IsActive     bool    `json:"is_active"`
	CreatedAt    *string `json:"created_at"`
	UpdatedAt    *string `json:"updated_at"`
}

func toOfficeResponse(o sqlc.MasterdataOffice) officeResponse {
	return officeResponse{
		ID:           o.ID.String(),
		ParentID:     uuidPtrStr(o.ParentID),
		OfficeTypeID: o.OfficeTypeID.String(),
		ProvinceID:   uuidPtrStr(o.ProvinceID),
		CityID:       uuidPtrStr(o.CityID),
		Name:         o.Name,
		Code:         o.Code,
		Address:      o.Address,
		IsActive:     o.IsActive,
		CreatedAt:    tsStr(o.CreatedAt),
		UpdatedAt:    tsStr(o.UpdatedAt),
	}
}

func (h *officeHandler) list(c *gin.Context) {
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, err := h.q.ListOffices(c.Request.Context(), sqlc.ListOfficesParams{
		AllScope: all, OfficeIds: ids, Search: search, Lim: limit, Off: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list offices"})
		return
	}
	total, err := h.q.CountOffices(c.Request.Context(), sqlc.CountOfficesParams{AllScope: all, OfficeIds: ids, Search: search})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count offices"})
		return
	}
	data := make([]officeResponse, 0, len(rows))
	for _, o := range rows {
		data = append(data, toOfficeResponse(o))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *officeHandler) get(c *gin.Context) {
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
	o, err := h.q.GetOffice(c.Request.Context(), sqlc.GetOfficeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusOK, toOfficeResponse(o))
}

func (h *officeHandler) create(c *gin.Context) {
	var req officeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	parent, _ := parseUUIDPtr(req.ParentID)
	// A scoped caller may only create an office under a parent within their scope.
	if !all && (parent == nil || !inScope(all, ids, *parent)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "office must be placed under an office within your scope"})
		return
	}
	province, _ := parseUUIDPtr(req.ProvinceID)
	city, _ := parseUUIDPtr(req.CityID)
	o, err := h.q.CreateOffice(c.Request.Context(), sqlc.CreateOfficeParams{
		ParentID:     parent,
		OfficeTypeID: uuid.MustParse(req.OfficeTypeID),
		ProvinceID:   province,
		CityID:       city,
		Name:         req.Name,
		Code:         req.Code,
		Address:      req.Address,
		IsActive:     boolOr(req.IsActive, true),
	})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusCreated, toOfficeResponse(o))
}

func (h *officeHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req officeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "offices")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	// Target must be within scope; fetch current to allow keeping its existing parent.
	cur, err := h.q.GetOffice(c.Request.Context(), sqlc.GetOfficeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	parent, _ := parseUUIDPtr(req.ParentID)
	if !all && !samePtr(parent, cur.ParentID) && (parent == nil || !inScope(all, ids, *parent)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot reparent the office outside your scope"})
		return
	}
	province, _ := parseUUIDPtr(req.ProvinceID)
	city, _ := parseUUIDPtr(req.CityID)
	o, err := h.q.UpdateOffice(c.Request.Context(), sqlc.UpdateOfficeParams{
		ParentID:     parent,
		OfficeTypeID: uuid.MustParse(req.OfficeTypeID),
		ProvinceID:   province,
		CityID:       city,
		Name:         req.Name,
		Code:         req.Code,
		Address:      req.Address,
		IsActive:     boolOr(req.IsActive, true),
		ID:           id,
		AllScope:     all,
		OfficeIds:    ids,
	})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusOK, toOfficeResponse(o))
}

func (h *officeHandler) delete(c *gin.Context) {
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
	n, err := h.q.SoftDeleteOffice(c.Request.Context(), sqlc.SoftDeleteOfficeParams{ID: id, AllScope: all, OfficeIds: ids})
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

func registerOffices(rg *gin.RouterGroup, q *sqlc.Queries, scope *authz.ScopeService, authMW, requireManage gin.HandlerFunc) {
	h := &officeHandler{scopedDeps{q: q, scope: scope}}
	g := rg.Group("/offices")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
