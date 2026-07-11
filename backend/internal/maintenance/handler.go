package maintenance

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

const scopeModule = "maintenance"

// Handler maps HTTP <-> the maintenance service (schedules, records, the Staf
// damage-report submit path via the generic approval engine).
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
	case errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrTerminal), errors.Is(err, ErrDuplicatePending):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAssetNotMaintainable), errors.Is(err, ErrAssetBusy), errors.Is(err, ErrInvalidRef),
		errors.Is(err, ErrScheduleMismatch), errors.Is(err, ErrInvalidInterval),
		errors.Is(err, asset.ErrUnsupportedType), errors.Is(err, asset.ErrTooLarge):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// caller mirrors assignment/handler.go's caller(): resolves user id + office scope.
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

func parseUUIDPtr(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	v, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &v
}

// --- Schedules ---

func (h *Handler) createSchedule(c *gin.Context) {
	var req CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	assetID, _ := uuid.Parse(req.AssetID)
	sch, err := h.svc.CreateSchedule(c.Request.Context(), all, ids, ScheduleInput{
		AssetID:               assetID,
		MaintenanceCategoryID: parseUUIDPtr(req.MaintenanceCategoryID),
		IntervalMonths:        req.IntervalMonths,
		StartDate:             req.StartDate,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "maintenance_schedules", sch.ID, nil, audit.Diff(nil, toScheduleResponse(sch)))
	c.JSON(http.StatusCreated, toScheduleResponse(sch))
}

func (h *Handler) listSchedules(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	var isActive *bool
	if v := c.Query("is_active"); v != "" {
		b := v == "true"
		isActive = &b
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.ListSchedules(c.Request.Context(), all, ids, isActive, limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichScheduleMap(toScheduleResponse(r.MaintenanceMaintenanceSchedule), r.AssetName, r.AssetTag, r.OfficeName, r.CategoryName))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) updateSchedule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	// Best-effort snapshot for the audit diff; if this fails the record either
	// doesn't exist or is out of scope, and the service call below returns the
	// authoritative (mapped) error.
	before, _ := h.q.GetMaintScheduleScoped(c.Request.Context(), sqlc.GetMaintScheduleScopedParams{ID: id, AllScope: all, OfficeIds: ids})
	after, err := h.svc.UpdateSchedule(c.Request.Context(), all, ids, id, ScheduleUpdateInput{
		MaintenanceCategoryID: parseUUIDPtr(req.MaintenanceCategoryID),
		IntervalMonths:        req.IntervalMonths,
		IsActive:              req.IsActive,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "maintenance_schedules", after.ID, nil, audit.Diff(toScheduleResponse(before), toScheduleResponse(after)))
	c.JSON(http.StatusOK, toScheduleResponse(after))
}

func (h *Handler) deleteSchedule(c *gin.Context) {
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
	before, _ := h.q.GetMaintScheduleScoped(c.Request.Context(), sqlc.GetMaintScheduleScopedParams{ID: id, AllScope: all, OfficeIds: ids})
	if err := h.svc.DeleteSchedule(c.Request.Context(), all, ids, id); err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "maintenance_schedules", id, nil, audit.Diff(toScheduleResponse(before), nil))
	c.Status(http.StatusNoContent)
}

// --- Records ---

func (h *Handler) createRecord(c *gin.Context) {
	var req CreateRecordRequest
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
	rec, err := h.svc.CreateRecord(c.Request.Context(), all, ids, caller.UserID, RecordInput{
		AssetID:               assetID,
		ScheduleID:            parseUUIDPtr(req.ScheduleID),
		MaintenanceCategoryID: parseUUIDPtr(req.MaintenanceCategoryID),
		ProblemCategoryID:     parseUUIDPtr(req.ProblemCategoryID),
		Type:                  sqlc.SharedMaintenanceType(req.Type),
		Status:                sqlc.SharedMaintenanceStatus(req.Status),
		ScheduledDate:         req.ScheduledDate,
		CompletedDate:         req.CompletedDate,
		Cost:                  req.Cost,
		VendorID:              parseUUIDPtr(req.VendorID),
		Description:           req.Description,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "maintenance_records", rec.ID, nil, audit.Diff(nil, toRecordResponse(rec)))
	c.JSON(http.StatusCreated, toRecordResponse(rec))
}

func (h *Handler) listRecords(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.ListRecords(c.Request.Context(), all, ids, c.Query("status"), c.Query("type"), c.Query("q"), limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichRecordMap(toRecordResponse(r.MaintenanceMaintenanceRecord), r.AssetName, r.AssetTag, r.OfficeName, r.CategoryName, r.ProblemName, r.VendorName, r.ReportedByName))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) getRecord(c *gin.Context) {
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
	r, err := h.svc.GetRecord(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, enrichRecordMap(toRecordResponse(r.MaintenanceMaintenanceRecord), r.AssetName, r.AssetTag, r.OfficeName, r.CategoryName, r.ProblemName, r.VendorName, r.ReportedByName))
}

func (h *Handler) updateRecord(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req UpdateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	before, _ := h.q.GetMaintRecordScoped(c.Request.Context(), sqlc.GetMaintRecordScopedParams{ID: id, AllScope: all, OfficeIds: ids})
	var status *sqlc.SharedMaintenanceStatus
	if req.Status != nil {
		v := sqlc.SharedMaintenanceStatus(*req.Status)
		status = &v
	}
	after, err := h.svc.UpdateRecord(c.Request.Context(), all, ids, id, RecordUpdateInput{
		Status:                status,
		MaintenanceCategoryID: parseUUIDPtr(req.MaintenanceCategoryID),
		ScheduledDate:         req.ScheduledDate,
		CompletedDate:         req.CompletedDate,
		Cost:                  req.Cost,
		VendorID:              parseUUIDPtr(req.VendorID),
		Description:           req.Description,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "maintenance_records", after.ID, nil, audit.Diff(toRecordResponse(before), toRecordResponse(after)))
	c.JSON(http.StatusOK, toRecordResponse(after))
}

func (h *Handler) attention(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	rows, err := h.svc.Attention(c.Request.Context(), all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		data = append(data, map[string]any{
			"id":          a.ID.String(),
			"asset_tag":   a.AssetTag,
			"name":        a.Name,
			"office_id":   a.OfficeID.String(),
			"office_name": a.OfficeName,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// submitReport handles POST /maintenance/reports (Staf damage report,
// multipart form with an optional "photo" file).
func (h *Handler) submitReport(c *gin.Context) {
	// Cap the request body to maxBytes+1 so we detect oversize before buffering
	// unbounded data — mirrors asset.Handler.uploadAttachment.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.ReportMaxBytes()+1)

	var form ReportForm
	if err := c.ShouldBind(&form); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, _, _, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	assetID, _ := uuid.Parse(form.AssetID)
	problemID, _ := uuid.Parse(form.ProblemCategoryID)
	in := ReportInput{AssetID: assetID, ProblemCategoryID: problemID, Description: form.Description}
	if fh, ferr := c.FormFile("photo"); ferr == nil && fh != nil {
		f, oerr := fh.Open()
		if oerr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo"})
			return
		}
		data, rerr := io.ReadAll(f)
		f.Close()
		if rerr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid photo"})
			return
		}
		in.Photo = &PhotoInput{Filename: fh.Filename, ContentType: fh.Header.Get("Content-Type"), Data: data}
	}
	out, err := h.svc.SubmitReport(c.Request.Context(), caller, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "requests", out.ID, out.OfficeID, audit.Diff(nil, map[string]any{"request_id": out.ID.String(), "type": "maintenance", "asset_id": form.AssetID}))
	c.JSON(http.StatusCreated, gin.H{"request_id": out.ID.String(), "status": string(out.Status)})
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
		data = append(data, enrichRecordMap(toRecordResponse(r.MaintenanceMaintenanceRecord), r.AssetName, r.AssetTag, r.OfficeName, r.CategoryName, r.ProblemName, r.VendorName, r.ReportedByName))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
