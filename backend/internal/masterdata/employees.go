package masterdata

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
)

type employeeHandler struct {
	scopedDeps
}

type employeeRequest struct {
	Code         string  `json:"code" binding:"required"`
	Name         string  `json:"name" binding:"required"`
	Email        *string `json:"email" binding:"omitempty,email"`
	AvatarKey    *string `json:"avatar_key"`
	DepartmentID *string `json:"department_id" binding:"omitempty,uuid"`
	PositionID   *string `json:"position_id" binding:"omitempty,uuid"`
	OfficeID     string  `json:"office_id" binding:"required,uuid"`
	Status       *string `json:"status" binding:"omitempty,oneof=active inactive suspended"`
}

type employeeResponse struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Email        *string `json:"email"`
	AvatarKey    *string `json:"avatar_key"`
	DepartmentID *string `json:"department_id"`
	PositionID   *string `json:"position_id"`
	OfficeID     string  `json:"office_id"`
	Status       string  `json:"status"`
	CreatedAt    *string `json:"created_at"`
	UpdatedAt    *string `json:"updated_at"`
}

func toEmployeeResponse(e sqlc.MasterdataEmployee) employeeResponse {
	return employeeResponse{
		ID:           e.ID.String(),
		Code:         e.Code,
		Name:         e.Name,
		Email:        e.Email,
		AvatarKey:    e.AvatarKey,
		DepartmentID: uuidPtrStr(e.DepartmentID),
		PositionID:   uuidPtrStr(e.PositionID),
		OfficeID:     e.OfficeID.String(),
		Status:       string(e.Status),
		CreatedAt:    tsStr(e.CreatedAt),
		UpdatedAt:    tsStr(e.UpdatedAt),
	}
}

func statusOr(p *string, def sqlc.SharedUserStatus) sqlc.SharedUserStatus {
	if p == nil || *p == "" {
		return def
	}
	return sqlc.SharedUserStatus(*p)
}

func (h *employeeHandler) list(c *gin.Context) {
	all, ids, err := h.callerOfficeScope(c, "employees")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)

	rows, err := h.q.ListEmployees(c.Request.Context(), sqlc.ListEmployeesParams{
		AllScope: all, OfficeIds: ids, Search: search, Lim: limit, Off: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list employees"})
		return
	}
	total, err := h.q.CountEmployees(c.Request.Context(), sqlc.CountEmployeesParams{AllScope: all, OfficeIds: ids, Search: search})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count employees"})
		return
	}
	data := make([]employeeResponse, 0, len(rows))
	for _, e := range rows {
		data = append(data, toEmployeeResponse(e))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *employeeHandler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "employees")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	e, err := h.q.GetEmployee(c.Request.Context(), sqlc.GetEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	c.JSON(http.StatusOK, toEmployeeResponse(e))
}

func (h *employeeHandler) create(c *gin.Context) {
	var req employeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "employees")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	officeID := uuid.MustParse(req.OfficeID)
	if !inScope(all, ids, officeID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "employee office must be within your scope"})
		return
	}
	dept, _ := parseUUIDPtr(req.DepartmentID)
	pos, _ := parseUUIDPtr(req.PositionID)
	e, err := h.q.CreateEmployee(c.Request.Context(), sqlc.CreateEmployeeParams{
		Code:         req.Code,
		Name:         req.Name,
		Email:        req.Email,
		AvatarKey:    req.AvatarKey,
		DepartmentID: dept,
		PositionID:   pos,
		OfficeID:     officeID,
		Status:       statusOr(req.Status, sqlc.SharedUserStatusActive),
	})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "employees", e.ID, &e.OfficeID, audit.Diff(nil, toEmployeeResponse(e)))
	c.JSON(http.StatusCreated, toEmployeeResponse(e))
}

func (h *employeeHandler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req employeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "employees")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	officeID := uuid.MustParse(req.OfficeID)
	if !inScope(all, ids, officeID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "employee office must be within your scope"})
		return
	}
	cur, err := h.q.GetEmployee(c.Request.Context(), sqlc.GetEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	dept, _ := parseUUIDPtr(req.DepartmentID)
	pos, _ := parseUUIDPtr(req.PositionID)
	e, err := h.q.UpdateEmployee(c.Request.Context(), sqlc.UpdateEmployeeParams{
		Code:         req.Code,
		Name:         req.Name,
		Email:        req.Email,
		AvatarKey:    req.AvatarKey,
		DepartmentID: dept,
		PositionID:   pos,
		OfficeID:     officeID,
		Status:       statusOr(req.Status, sqlc.SharedUserStatusActive),
		ID:           id,
		AllScope:     all,
		OfficeIds:    ids,
	})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "employees", e.ID, &e.OfficeID, audit.Diff(toEmployeeResponse(cur), toEmployeeResponse(e)))
	c.JSON(http.StatusOK, toEmployeeResponse(e))
}

func (h *employeeHandler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	all, ids, err := h.callerOfficeScope(c, "employees")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	cur, err := h.q.GetEmployee(c.Request.Context(), sqlc.GetEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		writeError(c, mapDBError(err))
		return
	}
	n, err := h.q.SoftDeleteEmployee(c.Request.Context(), sqlc.SoftDeleteEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "employees", id, &cur.OfficeID, audit.Diff(toEmployeeResponse(cur), nil))
	c.Status(http.StatusNoContent)
}

func registerEmployees(rg *gin.RouterGroup, q *sqlc.Queries, scope *authz.ScopeService, aud *audit.Service, authMW, requireManage gin.HandlerFunc) {
	h := &employeeHandler{scopedDeps{q: q, scope: scope, aud: aud}}
	g := rg.Group("/employees")
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
}
