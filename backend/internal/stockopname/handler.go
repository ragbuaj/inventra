package stockopname

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/disposal"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/transfer"
)

const scopeModule = "stockopname"

// Handler maps HTTP <-> the stock-opname service.
type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
	aud    *audit.Service
}

func NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler {
	return &Handler{svc: svc, scoped: common.ScopedDeps{Q: q, Scope: scope}, aud: aud}
}

// svcError maps this package's sentinels — plus the disposal/transfer
// sentinels that GenerateFollowup can surface (it reuses their own Submit) —
// to HTTP status codes.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNoItem):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrOutOfScope), errors.Is(err, disposal.ErrOutOfScope), errors.Is(err, transfer.ErrOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidState), errors.Is(err, ErrAlreadyFollowedUp),
		errors.Is(err, disposal.ErrAlreadyDisposed), errors.Is(err, disposal.ErrDisposalExists),
		errors.Is(err, transfer.ErrInvalidState), errors.Is(err, transfer.ErrAssetInTransit), errors.Is(err, transfer.ErrSameOffice):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidRef), errors.Is(err, disposal.ErrInvalidRef), errors.Is(err, transfer.ErrInvalidRef), errors.Is(err, approval.ErrNoThreshold):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, disposal.ErrNotFound), errors.Is(err, transfer.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// caller builds an approval.Caller from the Gin context, mirroring
// internal/transfer/handler.go's caller() (CtxUserID/CtxRoleID are string
// context keys read via c.GetString, then parsed as UUIDs).
func (h *Handler) caller(c *gin.Context) (approval.Caller, error) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return approval.Caller{}, err
	}
	rid, _ := uuid.Parse(c.GetString(middleware.CtxRoleID))
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		return approval.Caller{}, err
	}
	return approval.Caller{UserID: uid, RoleID: rid, AllScope: all, OfficeIDs: ids}, nil
}

// clampInt parses raw and clamps it into [min, max], falling back to def.
func clampInt(raw string, def, min, max int32) int32 {
	return common.ClampInt(raw, def, min, max)
}

// sessionResponse fetches and serializes a session by id (used after mutations
// to return the enriched view: office/started-by/closed-by names + KPIs).
func (h *Handler) sessionResponse(c *gin.Context, caller approval.Caller, id uuid.UUID) (map[string]any, error) {
	sess, kpi, err := h.svc.GetSession(c.Request.Context(), caller, id)
	if err != nil {
		return nil, err
	}
	return toSessionResponse(sess.StockopnameStockOpnameSession, sess.OfficeName, sess.StartedByName, sess.ClosedByName, &kpi), nil
}

func (h *Handler) create(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	officeID, err := uuid.Parse(req.OfficeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid office_id"})
		return
	}
	period, err := parsePeriod(req.Period)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period"})
		return
	}
	sess, err := h.svc.CreateSession(c.Request.Context(), caller, CreateInput{
		OfficeID: officeID,
		Name:     req.Name,
		Period:   period,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "stock_opname_sessions", sess.ID, &sess.OfficeID, audit.Diff(nil, map[string]any{"office_id": sess.OfficeID.String(), "period": req.Period}))
	out, err := h.sessionResponse(c, caller, sess.ID)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	out, err := h.sessionResponse(c, caller, id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) list(c *gin.Context) {
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	var status *string
	if s := c.Query("status"); s != "" {
		status = &s
	}
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.ListSessions(c.Request.Context(), caller, status, limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, toSessionResponse(r.StockopnameStockOpnameSession, r.OfficeName, nil, nil, nil))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) listItems(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	var result *string
	if r := c.Query("result"); r != "" {
		result = &r
	}
	rows, err := h.svc.ListItems(c.Request.Context(), caller, id, result)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, toItemResponse(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data), "limit": len(data), "offset": 0})
}

func (h *Handler) transition(c *gin.Context, to sqlc.SharedOpnameSessionStatus, action audit.Action) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	sess, err := h.svc.Transition(c.Request.Context(), caller, id, to)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, action, "stock_opname_sessions", sess.ID, &sess.OfficeID, audit.Diff(nil, map[string]any{"status": string(sess.Status)}))
	out, err := h.sessionResponse(c, caller, sess.ID)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) start(c *gin.Context) {
	h.transition(c, sqlc.SharedOpnameSessionStatusCounting, audit.ActionUpdate)
}

func (h *Handler) reconcile(c *gin.Context) {
	h.transition(c, sqlc.SharedOpnameSessionStatusReconciling, audit.ActionUpdate)
}

func (h *Handler) close(c *gin.Context) {
	h.transition(c, sqlc.SharedOpnameSessionStatusClosed, audit.ActionUpdate)
}

func (h *Handler) scan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	item, err := h.svc.Scan(c.Request.Context(), caller, id, req.AssetTag)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "stock_opname_items", item.ID, nil, audit.Diff(nil, map[string]any{"asset_tag": req.AssetTag, "session_id": id.String()}))
	c.JSON(http.StatusOK, map[string]any{
		"id":         item.ID.String(),
		"session_id": item.SessionID.String(),
		"asset_id":   item.AssetID.String(),
		"expected":   item.Expected,
		"result":     string(item.Result),
	})
}

func (h *Handler) setResult(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}
	var req SetResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	item, err := h.svc.SetItemResult(c.Request.Context(), caller, sessionID, itemID, sqlc.SharedOpnameItemResult(req.Result), req.Note)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "stock_opname_items", item.ID, nil, audit.Diff(nil, map[string]any{"result": string(item.Result)}))
	c.JSON(http.StatusOK, map[string]any{
		"id":         item.ID.String(),
		"session_id": item.SessionID.String(),
		"asset_id":   item.AssetID.String(),
		"expected":   item.Expected,
		"result":     string(item.Result),
		"note":       item.Note,
		"counted_at": common.TsStr(item.CountedAt),
	})
}

func (h *Handler) followup(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid itemId"})
		return
	}
	var req FollowupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	in := FollowupInput{Reason: req.Reason}
	if req.ToOfficeID != nil {
		v, perr := uuid.Parse(*req.ToOfficeID)
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to_office_id"})
			return
		}
		in.ToOfficeID = &v
	}
	if req.ToRoomID != nil {
		v, perr := uuid.Parse(*req.ToRoomID)
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to_room_id"})
			return
		}
		in.ToRoomID = &v
	}
	reqID, reqType, err := h.svc.GenerateFollowup(c.Request.Context(), caller, sessionID, itemID, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "stock_opname_items", itemID, nil, audit.Diff(nil, map[string]any{"followup_request_id": reqID.String(), "type": reqType}))
	c.JSON(http.StatusOK, gin.H{"request_id": reqID.String(), "type": reqType})
}

func (h *Handler) report(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	caller, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	data, err := h.svc.ReportData(c.Request.Context(), caller, id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	format := c.DefaultQuery("format", "pdf")
	switch format {
	case "xlsx":
		bytes, err := RenderXLSX(data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "berita-acara-stock-opname-"+id.String()+".xlsx"))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", bytes)
	default:
		bytes, err := RenderPDF(data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "berita-acara-stock-opname-"+id.String()+".pdf"))
		c.Data(http.StatusOK, "application/pdf", bytes)
	}
}
