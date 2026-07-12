package approval

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// Handler is the HTTP handler for the approval module.
type Handler struct {
	svc      *Service
	fieldSvc *authz.FieldService
	scoped   common.ScopedDeps
	aud      *audit.Service
}

// NewHandler constructs a Handler with all dependencies.
func NewHandler(svc *Service, fieldSvc *authz.FieldService, scoped common.ScopedDeps, aud *audit.Service) *Handler {
	return &Handler{svc: svc, fieldSvc: fieldSvc, scoped: scoped, aud: aud}
}

// callerFromCtx builds a Caller from the Gin context (auth + scope).
func (h *Handler) callerFromCtx(c *gin.Context) (Caller, error) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return Caller{}, err
	}
	rid, _ := uuid.Parse(c.GetString(middleware.CtxRoleID))
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: uid, RoleID: rid, AllScope: all, OfficeIDs: ids}, nil
}

// filterMap applies field-permission masking for the caller's role on the
// "requests" entity. Delegates to authz.FilterEntity, which fails closed on
// ForEntity errors so sensitive amounts are never leaked when the policy
// store is unavailable.
func (h *Handler) filterMap(c *gin.Context, m map[string]any) (map[string]any, error) {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return m, nil
	}
	if err := h.fieldSvc.FilterEntity(c.Request.Context(), roleID, "requests", m); err != nil {
		return nil, err
	}
	return m, nil
}

// svcError maps approval sentinel errors to HTTP status codes.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch err {
	case ErrNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case ErrSelfApproval, ErrNotEligible, ErrForbidden:
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case ErrInvalidState:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case ErrNoThreshold:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case ErrConflict:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case ErrInvalidRef:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// submit handles POST /requests.
func (h *Handler) submit(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, _ := uuid.Parse(c.GetString(middleware.CtxUserID))
	officeID, _ := uuid.Parse(req.OfficeID)

	// Enforce data scope: the maker may only route requests through an office
	// that falls within their own office data scope.
	all, ids, err := h.scoped.CallerOfficeScope(c, "assets")
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if !common.InScope(all, ids, officeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}

	in := SubmitInput{
		Type:     sqlc.SharedRequestType(req.Type),
		Amount:   req.Amount,
		OfficeID: officeID,
		Payload:  []byte(req.Payload),
		Reason:   req.Reason,
		Maker:    uid,
	}
	if req.TargetID != nil {
		tid, _ := uuid.Parse(*req.TargetID)
		in.TargetID = &tid
	}
	out, err := h.svc.Submit(c, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "requests", out.ID, out.OfficeID, audit.Diff(nil, requestToMap(out)))
	c.JSON(http.StatusCreated, requestToMap(out))
}

// approve handles POST /requests/:id/approve.
func (h *Handler) approve(c *gin.Context) { h.decide(c, true) }

// reject handles POST /requests/:id/reject.
func (h *Handler) reject(c *gin.Context) { h.decide(c, false) }

// decide is the shared implementation for approve and reject.
func (h *Handler) decide(c *gin.Context, isApprove bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body DecideRequest
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, err := h.callerFromCtx(c)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	out, err := h.svc.Decide(c, id, caller, isApprove, body.Note)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "requests", out.ID, out.OfficeID, audit.Diff(nil, requestToMap(out)))
	c.JSON(http.StatusOK, requestToMap(out))
}

// cancel handles POST /requests/:id/cancel.
func (h *Handler) cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	uid, _ := uuid.Parse(c.GetString(middleware.CtxUserID))
	out, err := h.svc.Cancel(c, id, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, requestToMap(out))
}

// inbox handles GET /requests/inbox.
func (h *Handler) inbox(c *gin.Context) {
	caller, err := h.callerFromCtx(c)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	rows, err := h.svc.Inbox(c, caller)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m, err := h.filterMap(c, enrichRequestMap(requestToMap(r.ApprovalRequest), r.RequestedByName, r.RequestedByRole, r.OfficeName))
		if err != nil {
			common.WriteError(c, err)
			return
		}
		data = append(data, m)
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

// list handles GET /requests.
func (h *Handler) list(c *gin.Context) {
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests")
	if err != nil {
		common.WriteError(c, err)
		return
	}
	var requestedBy *uuid.UUID
	if c.Query("mine") == "true" {
		// Own submitted requests: filter by requester and bypass office scope
		// (a caller can always see their own requests regardless of scope config).
		if uid, perr := uuid.Parse(c.GetString(middleware.CtxUserID)); perr == nil {
			requestedBy = &uid
			all, ids = true, nil
		}
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<30)
	rows, total, err := h.svc.List(c, all, ids, c.Query("status"), c.Query("type"), limit, offset, requestedBy)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m, err := h.filterMap(c, enrichRequestMap(requestToMap(r.ApprovalRequest), r.RequestedByName, r.RequestedByRole, r.OfficeName))
		if err != nil {
			common.WriteError(c, err)
			return
		}
		data = append(data, m)
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

// get handles GET /requests/:id (returns enriched request + payload + its approval steps).
func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	row, steps, err := h.svc.GetWithSteps(c, id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	r := row.ApprovalRequest
	// Enforce data scope: the caller may only view requests within their office scope.
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests")
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if r.OfficeID == nil || !common.InScope(all, ids, *r.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	out := enrichRequestMap(requestToMap(r), row.RequestedByName, row.RequestedByRole, row.OfficeName)
	var payload any
	if len(r.Payload) > 0 {
		_ = json.Unmarshal(r.Payload, &payload)
	}
	out["payload"] = payload
	stepMaps := make([]map[string]any, 0, len(steps))
	for _, st := range steps {
		stepMaps = append(stepMaps, stepToMap(st))
	}
	out["steps"] = stepMaps
	out, err = h.filterMap(c, out)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// listThresholds handles GET /approval-thresholds.
func (h *Handler) listThresholds(c *gin.Context) {
	rows, err := h.svc.ListThresholds(c)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		data = append(data, thresholdToMap(t))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

// createThreshold handles POST /approval-thresholds.
func (h *Handler) createThreshold(c *gin.Context) {
	var req ThresholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.svc.CreateThreshold(c, req.toCreateParams())
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusCreated, thresholdToMap(out))
}

// updateThreshold handles PUT /approval-thresholds/:id.
func (h *Handler) updateThreshold(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req ThresholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.validateUpdate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.svc.UpdateThreshold(c, req.toUpdateParams(id))
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, thresholdToMap(out))
}

// previewThresholds handles GET /approval-thresholds/preview.
func (h *Handler) previewThresholds(c *gin.Context) {
	rt := c.Query("request_type")
	if !validRequestTypes[rt] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request_type"})
		return
	}
	amount := c.Query("amount")
	if _, ok := parsePlainDecimal(amount); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
		return
	}
	steps, err := h.svc.PreviewChain(c, sqlc.SharedRequestType(rt), amount)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

// deleteThreshold handles DELETE /approval-thresholds/:id.
func (h *Handler) deleteThreshold(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.DeleteThreshold(c, id); err != nil {
		h.svcError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
