package user

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// Handler exposes the user-management HTTP endpoints.
type Handler struct {
	svc    *Service
	fields *authz.FieldService
	aud    *audit.Service
}

// NewHandler builds the user Handler.
func NewHandler(svc *Service, fields *authz.FieldService, aud *audit.Service) *Handler {
	return &Handler{svc: svc, fields: fields, aud: aud}
}

// validUserStatuses are the accepted values for the `status` filter on
// GET /users, matching the shared.user_status enum.
var validUserStatuses = map[string]bool{"active": true, "inactive": true, "suspended": true}

func (h *Handler) list(c *gin.Context) {
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)

	roleIDRaw := c.Query("role_id")
	roleID, err := parseUUIDPtr(&roleIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role_id"})
		return
	}
	officeIDRaw := c.Query("office_id")
	officeID, err := parseUUIDPtr(&officeIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid office_id"})
		return
	}
	var status *string
	if raw := c.Query("status"); raw != "" {
		if !validUserStatuses[raw] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		status = &raw
	}

	users, total, err := h.svc.List(c.Request.Context(), search, roleID, officeID, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	data := make([]map[string]any, 0, len(users))
	for _, u := range users {
		data = append(data, userToMap(u))
	}
	if err := h.filterMaps(c, data...); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return
	}
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
	m, ok := h.one(c, u)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, m)
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
	audit.Record(c, h.aud, audit.ActionCreate, "users", u.ID, u.OfficeID, audit.Diff(nil, userToMap(u)))
	m, ok := h.one(c, u)
	if !ok {
		return
	}
	c.JSON(http.StatusCreated, m)
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
	before, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
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
	audit.Record(c, h.aud, audit.ActionUpdate, "users", u.ID, u.OfficeID, audit.Diff(userToMap(before), userToMap(u)))
	m, ok := h.one(c, u)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	before, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "users", id, before.OfficeID, audit.Diff(userToMap(before), nil))
	c.Status(http.StatusNoContent)
}

// one builds a single field-filtered user record. On a field-policy lookup
// error it responds 500 (fail-closed) and returns ok=false; the caller must
// stop and not serve the unfiltered record.
func (h *Handler) one(c *gin.Context, u sqlc.IdentityUser) (m map[string]any, ok bool) {
	m = userToMap(u)
	if err := h.filterMaps(c, m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return nil, false
	}
	return m, true
}

// filterMaps removes fields the caller's role may not view (field permissions),
// delegating to authz.FilterEntity per record. Fail-closed: a policy-lookup
// error (e.g. Redis/Postgres unavailable) is returned to the caller instead of
// being swallowed, so callers refuse to serve unfiltered data rather than
// silently leaking it (previously this fell back to serving unmasked records).
// An unparseable/missing role id (CtxRoleID) is treated the same way.
func (h *Handler) filterMaps(c *gin.Context, maps ...map[string]any) error {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return err
	}
	for _, m := range maps {
		if err := h.fields.FilterEntity(c.Request.Context(), roleID, "users", m); err != nil {
			return err
		}
	}
	return nil
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
