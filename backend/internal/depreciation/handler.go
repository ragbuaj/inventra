package depreciation

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// scopeModule is the data_scope_policies module for the periods/schedule/
// journal endpoints (migration 000023 seeds it per role).
const scopeModule = "depreciation"

// assetEntity is the field_permissions/data-scope entity reused from the
// asset module for GET /assets/:id/depreciation (it is a view onto asset data,
// gated by the SAME "assets" scope + book_value field policy as the asset
// module itself).
const assetEntity = "assets"

// Handler is the HTTP handler for the depreciation module.
type Handler struct {
	svc      *Service
	fieldSvc *authz.FieldService
	scoped   common.ScopedDeps
	aud      *audit.Service
}

// NewHandler builds a Handler with all dependencies.
func NewHandler(svc *Service, fieldSvc *authz.FieldService, scoped common.ScopedDeps, aud *audit.Service) *Handler {
	return &Handler{svc: svc, fieldSvc: fieldSvc, scoped: scoped, aud: aud}
}

// svcError maps depreciation sentinel errors to HTTP status codes.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case err == ErrPeriodClosed:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err == ErrPeriodNotComputed, err == ErrPriorPeriodOpen, err == ErrPeriodBeforeWatermark:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case err == ErrNoBookValue, err == ErrInvalidRecoverable:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case err == ErrNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// periodEntityID derives a deterministic uuid for a period's audit-log
// entity_id (depreciation_periods rows are keyed by `period` date, not a
// uuid the handler has on hand at compute/close time).
func periodEntityID(period time.Time) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte("depreciation_periods:"+period.Format(periodLayout)))
}

// parsePeriodParam parses the `:period` path param ("YYYY-MM").
func parsePeriodParam(c *gin.Context) (time.Time, bool) {
	period, err := time.Parse(periodLayout, c.Param("period"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period, expected YYYY-MM"})
		return time.Time{}, false
	}
	return period, true
}

// parsePeriodQuery parses the `period` query param ("YYYY-MM").
func parsePeriodQuery(c *gin.Context) (time.Time, bool) {
	period, err := time.Parse(periodLayout, c.Query("period"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period, expected YYYY-MM"})
		return time.Time{}, false
	}
	return period, true
}

// listPeriods handles GET /depreciation/periods.
func (h *Handler) listPeriods(c *gin.Context) {
	infos, err := h.svc.Periods(c.Request.Context())
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]gin.H, 0, len(infos))
	for _, pi := range infos {
		data = append(data, periodInfoToMap(pi))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// compute handles POST /depreciation/periods/:period/compute.
func (h *Handler) compute(c *gin.Context) {
	period, ok := parsePeriodParam(c)
	if !ok {
		return
	}
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	summary, err := h.svc.ComputePeriod(c.Request.Context(), period, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}
	out := gin.H{
		"period": period.Format(periodLayout), "status": "computed",
		"asset_count": summary.AssetCount, "total_amount": summary.TotalAmount,
		"skipped_count": summary.SkippedCount,
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "depreciation_periods", periodEntityID(period), nil, audit.Diff(nil, out))
	c.JSON(http.StatusOK, out)
}

// close handles POST /depreciation/periods/:period/close.
func (h *Handler) close(c *gin.Context) {
	period, ok := parsePeriodParam(c)
	if !ok {
		return
	}
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	if err := h.svc.ClosePeriod(c.Request.Context(), period, uid); err != nil {
		h.svcError(c, err)
		return
	}
	out := gin.H{"period": period.Format(periodLayout), "status": "closed"}
	audit.Record(c, h.aud, audit.ActionUpdate, "depreciation_periods", periodEntityID(period), nil, audit.Diff(nil, out))
	c.JSON(http.StatusOK, out)
}

// parseOptionalUUIDQuery parses an optional uuid query param; "" is nil/ok.
func parseOptionalUUIDQuery(c *gin.Context, name string) (*uuid.UUID, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return nil, false
	}
	return &id, true
}

// schedule handles GET /depreciation/schedule.
func (h *Handler) schedule(c *gin.Context) {
	period, ok := parsePeriodQuery(c)
	if !ok {
		return
	}
	basis, err := parseBasis(c.Query("basis"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	categoryID, ok := parseOptionalUUIDQuery(c, "category_id")
	if !ok {
		return
	}
	officeID, ok := parseOptionalUUIDQuery(c, "office_id")
	if !ok {
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	result, err := h.svc.Schedule(c.Request.Context(), period, basis, all, ids, c.Query("search"), categoryID, officeID)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, scheduleToMap(result))
}

// journal handles GET /depreciation/journal.
func (h *Handler) journal(c *gin.Context) {
	period, ok := parsePeriodQuery(c)
	if !ok {
		return
	}
	basis, err := parseBasis(c.Query("basis"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	result, err := h.svc.Journal(c.Request.Context(), period, basis, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, journalToMap(result))
}

// assetSchedule handles GET /assets/:id/depreciation.
func (h *Handler) assetSchedule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	result, err := h.svc.AssetSchedule(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, assetEntity)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if !common.InScope(all, ids, result.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}

	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	policies, err := h.fieldSvc.ForEntity(c.Request.Context(), roleID, assetEntity)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if pol, ok := policies["book_value"]; ok && !pol.CanView {
		c.JSON(http.StatusOK, maskedAssetScheduleMap())
		return
	}
	c.JSON(http.StatusOK, assetScheduleToMap(result))
}

// recordImpairment handles POST /assets/:id/impairment (PSAK 48 write-down).
// The asset's office is resolved and scope-checked BEFORE calling
// RecordImpairment (which has no scope params of its own — see service.go);
// unlike assetSchedule's read-only scope check, this guards a mutation, so
// the pre-fetch must happen ahead of, not after, the write.
func (h *Handler) recordImpairment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req ImpairmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, ok := parsePlainDecimal(req.RecoverableAmount); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recoverable_amount"})
		return
	}

	before, err := h.svc.GetAssetSummary(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if !common.InScope(all, ids, before.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}

	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}

	after, err := h.svc.RecordImpairment(c.Request.Context(), id, req.RecoverableAmount, req.Reason, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}

	beforeMoney := map[string]any{
		"book_value":               before.BookValue,
		"impairment_loss":          before.ImpairmentLoss,
		"accumulated_depreciation": before.AccumulatedDepreciation,
	}
	afterMoney := map[string]any{
		"book_value":               after.BookValue,
		"impairment_loss":          after.ImpairmentLoss,
		"accumulated_depreciation": after.AccumulatedDepreciation,
		"reason":                   req.Reason,
	}
	audit.Record(c, h.aud, audit.ActionUpdate, assetEntity, id, &after.OfficeID, audit.Diff(beforeMoney, afterMoney))

	c.JSON(http.StatusOK, impairmentResultToMap(after))
}
