package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// Handler exposes the user-management HTTP endpoints.
type Handler struct {
	svc    *Service
	fields *authz.FieldService
}

// NewHandler builds the user Handler.
func NewHandler(svc *Service, fields *authz.FieldService) *Handler {
	return &Handler{svc: svc, fields: fields}
}

func (h *Handler) list(c *gin.Context) {
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)

	users, total, err := h.svc.List(c.Request.Context(), search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	data := make([]map[string]any, 0, len(users))
	for _, u := range users {
		data = append(data, userToMap(u))
	}
	h.filterMaps(c, data...)
	c.JSON(http.StatusOK, listResponse{Data: data, Total: total, Limit: limit, Offset: offset})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.one(c, u))
}

func (h *Handler) create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	office, employee, err := refs(req.OfficeID, req.EmployeeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Create(c.Request.Context(), CreateInput{
		Name:       req.Name,
		Email:      req.Email,
		Password:   req.Password,
		RoleID:     uuid.MustParse(req.RoleID),
		OfficeID:   office,
		EmployeeID: employee,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusCreated, h.one(c, u))
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	office, employee, err := refs(req.OfficeID, req.EmployeeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Update(c.Request.Context(), id, UpdateInput{
		Name:       req.Name,
		RoleID:     uuid.MustParse(req.RoleID),
		Status:     sqlc.SharedUserStatus(req.Status),
		OfficeID:   office,
		EmployeeID: employee,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.one(c, u))
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.svcError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// one builds a single field-filtered user record.
func (h *Handler) one(c *gin.Context, u sqlc.IdentityUser) map[string]any {
	m := userToMap(u)
	h.filterMaps(c, m)
	return m
}

// filterMaps removes fields the caller's role may not view (field permissions).
func (h *Handler) filterMaps(c *gin.Context, maps ...map[string]any) {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return
	}
	policies, err := h.fields.ForEntity(c.Request.Context(), roleID, "users")
	if err != nil || policies == nil {
		return
	}
	for _, m := range maps {
		authz.FilterView(policies, m)
	}
}

func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrEmailExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidReference):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// refs parses optional office/employee UUID strings (already validated by binding).
func refs(officeID, employeeID *string) (*uuid.UUID, *uuid.UUID, error) {
	office, err := parseUUIDPtr(officeID)
	if err != nil {
		return nil, nil, err
	}
	employee, err := parseUUIDPtr(employeeID)
	if err != nil {
		return nil, nil, err
	}
	return office, employee, nil
}

func parseUUIDPtr(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func clampInt(raw string, def, min, max int32) int32 {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	v := int32(n)
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
