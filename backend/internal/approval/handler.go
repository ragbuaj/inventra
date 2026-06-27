package approval

import (
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
		data = append(data, requestToMap(r))
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
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<30)
	rows, total, err := h.svc.List(c, all, ids, c.Query("status"), c.Query("type"), limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, requestToMap(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

// get handles GET /requests/:id (returns request + its approval steps).
func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	r, steps, err := h.svc.GetWithSteps(c, id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	out := requestToMap(r)
	out["steps"] = steps
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
