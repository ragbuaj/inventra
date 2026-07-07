package assignment

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

const scopeModule = "assignments"

// Handler maps HTTP <-> the assignment service (check-out/check-in + the Staf
// borrow submit path via the generic approval engine).
type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
	q      *sqlc.Queries
	aud    *audit.Service
}

func NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler {
	return &Handler{svc: svc, scoped: common.ScopedDeps{Q: q, Scope: scope}, q: q, aud: aud}
}

func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrNotActive), errors.Is(err, ErrAlreadyAssigned):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAssetNotAvailable), errors.Is(err, ErrInvalidRef), errors.Is(err, ErrNoEmployee):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// caller mirrors transfer/handler.go's caller(): resolves user id + office scope.
func (h *Handler) caller(c *gin.Context) (approval.Caller, bool, []uuid.UUID, error) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return approval.Caller{}, false, nil, err
	}
	rid, _ := uuid.Parse(c.GetString(middleware.CtxRoleID))
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		return approval.Caller{}, false, nil, err
	}
	return approval.Caller{UserID: uid, RoleID: rid, AllScope: all, OfficeIDs: ids}, all, ids, nil
}

func (h *Handler) checkout(c *gin.Context) {
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	assetID, _ := uuid.Parse(req.AssetID)
	employeeID, _ := uuid.Parse(req.EmployeeID)
	a, err := h.svc.Checkout(c.Request.Context(), all, ids, caller.UserID, CheckoutInput{
		AssetID: assetID, EmployeeID: employeeID, CheckoutDate: req.CheckoutDate,
		DueDate: req.DueDate, ConditionOut: req.ConditionOut, Notes: req.Notes,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "assignments", a.ID, nil, audit.Diff(nil, toResponse(a)))
	c.JSON(http.StatusCreated, toResponse(a))
}

func (h *Handler) checkin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req CheckinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	before, after, err := h.svc.Checkin(c.Request.Context(), all, ids, id, CheckinInput{
		CheckinDate: req.CheckinDate, ConditionIn: req.ConditionIn, NeedsMaintenance: req.NeedsMaintenance,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "assignments", after.ID, nil, audit.Diff(toResponse(before), toResponse(after)))
	c.JSON(http.StatusOK, toResponse(after))
}

// borrow handles POST /assignments/borrow (Staf peminjaman submit). Beyond the
// service's own checks, this pre-validates that the caller has a linked employee
// so a Staf with no employee record gets immediate 422 feedback (ErrNoEmployee)
// instead of a request that only fails once approved.
func (h *Handler) borrow(c *gin.Context) {
	var req BorrowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, _, _, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	u, err := h.q.GetUserByID(c.Request.Context(), caller.UserID)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if u.EmployeeID == nil {
		h.svcError(c, ErrNoEmployee)
		return
	}
	assetID, _ := uuid.Parse(req.AssetID)
	out, err := h.svc.SubmitBorrow(c.Request.Context(), caller, BorrowInput{
		AssetID: assetID, DueDate: req.DueDate, ConditionOut: req.ConditionOut, Notes: req.Notes,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "requests", out.ID, out.OfficeID, audit.Diff(nil, map[string]any{"request_id": out.ID.String(), "type": "assignment", "asset_id": req.AssetID}))
	c.JSON(http.StatusCreated, gin.H{"request_id": out.ID.String(), "status": string(out.Status)})
}

func (h *Handler) available(c *gin.Context) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user"})
		return
	}
	u, err := h.q.GetUserByID(c.Request.Context(), uid)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if u.OfficeID == nil {
		c.JSON(http.StatusOK, gin.H{"data": []any{}})
		return
	}
	rows, err := h.svc.Available(c.Request.Context(), *u.OfficeID)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		data = append(data, map[string]any{"id": a.ID.String(), "asset_tag": a.AssetTag, "name": a.Name})
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	r, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, enrichAssignmentMap(toResponse(r.AssignmentAssignment), r.AssetName, r.AssetTag, r.EmployeeName, r.AssignedByName, r.OfficeName))
}

func (h *Handler) list(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	var empID *uuid.UUID
	if e := c.Query("employee_id"); e != "" {
		if id, perr := uuid.Parse(e); perr == nil {
			empID = &id
		}
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.List(c.Request.Context(), all, ids, c.Query("status"), empID, c.Query("search"), limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichAssignmentMap(toResponse(r.AssignmentAssignment), r.AssetName, r.AssetTag, r.EmployeeName, r.AssignedByName, r.OfficeName))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) listByAsset(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	rows, err := h.svc.ListByAsset(c.Request.Context(), assetID, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichAssignmentMap(toResponse(r.AssignmentAssignment), r.AssetName, r.AssetTag, r.EmployeeName, r.AssignedByName, r.OfficeName))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
