package masterdata

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
)

// --- service --------------------------------------------------------------

type categoryService struct {
	q *sqlc.Queries
}

type categoryInput struct {
	Name         string
	Code         *string
	ParentID     *uuid.UUID
	DeprMethod   *sqlc.SharedDepreciationMethod
	UsefulLifeMo *int32
	SalvageRate  *string
	IsActive     bool
}

func (s *categoryService) create(ctx context.Context, in categoryInput) (sqlc.MasterdataCategory, error) {
	c, err := s.q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name:                      in.Name,
		Code:                      in.Code,
		ParentID:                  in.ParentID,
		DefaultDepreciationMethod: in.DeprMethod,
		DefaultUsefulLifeMonths:   in.UsefulLifeMo,
		DefaultSalvageRate:        in.SalvageRate,
		IsActive:                  in.IsActive,
	})
	if err != nil {
		return sqlc.MasterdataCategory{}, mapDBError(err)
	}
	return c, nil
}

func (s *categoryService) update(ctx context.Context, id uuid.UUID, in categoryInput) (sqlc.MasterdataCategory, error) {
	c, err := s.q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		ID:                        id,
		Name:                      in.Name,
		Code:                      in.Code,
		ParentID:                  in.ParentID,
		DefaultDepreciationMethod: in.DeprMethod,
		DefaultUsefulLifeMonths:   in.UsefulLifeMo,
		DefaultSalvageRate:        in.SalvageRate,
		IsActive:                  in.IsActive,
	})
	if err != nil {
		return sqlc.MasterdataCategory{}, mapDBError(err)
	}
	return c, nil
}

// --- dto ------------------------------------------------------------------

type categoryRequest struct {
	Name                      string  `json:"name" binding:"required"`
	Code                      *string `json:"code"`
	ParentID                  *string `json:"parent_id" binding:"omitempty,uuid"`
	DefaultDepreciationMethod *string `json:"default_depreciation_method" binding:"omitempty,oneof=straight_line declining_balance"`
	DefaultUsefulLifeMonths   *int32  `json:"default_useful_life_months"`
	DefaultSalvageRate        *string `json:"default_salvage_rate"`
	IsActive                  *bool   `json:"is_active"`
}

type categoryResponse struct {
	ID                        string  `json:"id"`
	Name                      string  `json:"name"`
	Code                      *string `json:"code"`
	ParentID                  *string `json:"parent_id"`
	DefaultDepreciationMethod *string `json:"default_depreciation_method"`
	DefaultUsefulLifeMonths   *int32  `json:"default_useful_life_months"`
	DefaultSalvageRate        *string `json:"default_salvage_rate"`
	IsActive                  bool    `json:"is_active"`
	CreatedAt                 *string `json:"created_at"`
	UpdatedAt                 *string `json:"updated_at"`
}

func toCategoryResponse(c sqlc.MasterdataCategory) categoryResponse {
	var method *string
	if c.DefaultDepreciationMethod != nil {
		s := string(*c.DefaultDepreciationMethod)
		method = &s
	}
	return categoryResponse{
		ID:                        c.ID.String(),
		Name:                      c.Name,
		Code:                      c.Code,
		ParentID:                  uuidPtrStr(c.ParentID),
		DefaultDepreciationMethod: method,
		DefaultUsefulLifeMonths:   c.DefaultUsefulLifeMonths,
		DefaultSalvageRate:        c.DefaultSalvageRate,
		IsActive:                  c.IsActive,
		CreatedAt:                 tsStr(c.CreatedAt),
		UpdatedAt:                 tsStr(c.UpdatedAt),
	}
}

type categoryListResponse struct {
	Data   []categoryResponse `json:"data"`
	Total  int64              `json:"total"`
	Limit  int32              `json:"limit"`
	Offset int32              `json:"offset"`
}

func (r categoryRequest) toInput() (categoryInput, error) {
	parent, err := parseUUIDPtr(r.ParentID)
	if err != nil {
		return categoryInput{}, err
	}
	var method *sqlc.SharedDepreciationMethod
	if r.DefaultDepreciationMethod != nil {
		m := sqlc.SharedDepreciationMethod(*r.DefaultDepreciationMethod)
		method = &m
	}
	return categoryInput{
		Name:         r.Name,
		Code:         r.Code,
		ParentID:     parent,
		DeprMethod:   method,
		UsefulLifeMo: r.DefaultUsefulLifeMonths,
		SalvageRate:  r.DefaultSalvageRate,
		IsActive:     boolOr(r.IsActive, true),
	}, nil
}

// --- handler --------------------------------------------------------------

type categoryHandler struct {
	svc *categoryService
	aud *audit.Service
}

func (h *categoryHandler) list(c *gin.Context) {
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, err := h.svc.q.ListCategories(c.Request.Context(), sqlc.ListCategoriesParams{Search: search, Lim: limit, Off: offset})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list categories"})
		return
	}
	total, err := h.svc.q.CountCategories(c.Request.Context(), search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count categories"})
		return
	}
	data := make([]categoryResponse, 0, len(rows))
	for _, c := range rows {
		data = append(data, toCategoryResponse(c))
	}
	c.JSON(http.StatusOK, categoryListResponse{Data: data, Total: total, Limit: limit, Offset: offset})
}

func (h *categoryHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	cat, err := h.svc.q.GetCategory(c.Request.Context(), id)
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusOK, toCategoryResponse(cat))
}

func (h *categoryHandler) create(c *gin.Context) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := req.toInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cat, err := h.svc.create(c.Request.Context(), in)
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "categories", cat.ID, nil, audit.Diff(nil, toCategoryResponse(cat)))
	c.JSON(http.StatusCreated, toCategoryResponse(cat))
}

func (h *categoryHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := req.toInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cur, err := h.svc.q.GetCategory(c.Request.Context(), id)
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	cat, err := h.svc.update(c.Request.Context(), id, in)
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "categories", cat.ID, nil, audit.Diff(toCategoryResponse(cur), toCategoryResponse(cat)))
	c.JSON(http.StatusOK, toCategoryResponse(cat))
}

func (h *categoryHandler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	cur, err := h.svc.q.GetCategory(c.Request.Context(), id)
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	n, err := h.svc.q.SoftDeleteCategory(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "categories", id, nil, audit.Diff(toCategoryResponse(cur), nil))
	c.Status(http.StatusNoContent)
}

// --- routes ---------------------------------------------------------------

func registerCategories(rg *gin.RouterGroup, q *sqlc.Queries, aud *audit.Service, authMW, requireManage gin.HandlerFunc) {
	h := &categoryHandler{svc: &categoryService{q: q}, aud: aud}
	g := rg.Group("/categories")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
