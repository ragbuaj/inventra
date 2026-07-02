package transfer

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

const scopeModule = "transfers"

// Handler maps HTTP ↔ the transfer service, orchestrating BAST document creation on receive.
type Handler struct {
	svc      *Service
	assetSvc *asset.Service
	scoped   common.ScopedDeps
	aud      *audit.Service
}

func NewHandler(svc *Service, assetSvc *asset.Service, scope *authz.ScopeService, q *sqlc.Queries, aud *audit.Service) *Handler {
	return &Handler{svc: svc, assetSvc: assetSvc, scoped: common.ScopedDeps{Q: q, Scope: scope}, aud: aud}
}

func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidState):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAssetInTransit), errors.Is(err, ErrSameOffice), errors.Is(err, ErrInvalidRef):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

func parseDate(s *string) (pgtype.Date, error) {
	if s == nil || *s == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// caller builds an approval.Caller from the Gin context, mirroring
// internal/approval/handler.go's callerFromCtx (CtxUserID/CtxRoleID are
// string context keys read via c.GetString, then parsed as UUIDs).
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

func (h *Handler) submit(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, _, _, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	in := SubmitInput{
		AssetID:    uuid.MustParse(req.AssetID),
		ToOfficeID: uuid.MustParse(req.ToOfficeID),
		Reason:     req.Reason,
	}
	if req.ToRoomID != nil {
		r := uuid.MustParse(*req.ToRoomID)
		in.ToRoomID = &r
	}
	out, err := h.svc.Submit(c.Request.Context(), caller, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "transfers", out.ID, out.OfficeID, audit.Diff(nil, map[string]any{"request_id": out.ID.String(), "type": "asset_transfer", "asset_id": req.AssetID}))
	c.JSON(http.StatusCreated, gin.H{"request_id": out.ID.String(), "status": string(out.Status)})
}

func (h *Handler) ship(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body ShipRequest
	_ = c.ShouldBindJSON(&body) // body optional
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	shipped, derr := parseDate(body.ShippedDate)
	if derr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid shipped_date"})
		return
	}
	out, err := h.svc.Ship(c.Request.Context(), all, ids, id, ShipInput{ShippedDate: shipped})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "transfers", out.ID, &out.FromOfficeID, audit.Diff(map[string]any{"status": "approved"}, map[string]any{"status": "in_transit"}))
	c.JSON(http.StatusOK, toResponse(out))
}

func (h *Handler) receive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body ReceiveRequest
	_ = c.ShouldBind(&body) // multipart or JSON; file read separately
	caller, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	recvDate, derr := parseDate(body.ReceivedDate)
	if derr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid received_date"})
		return
	}
	in := ReceiveInput{BastNo: body.BastNo, ReceivedDate: recvDate}
	if body.ToRoomID != nil {
		r := uuid.MustParse(*body.ToRoomID)
		in.ToRoomID = &r
	}
	before, after, err := h.svc.Receive(c.Request.Context(), all, ids, caller.UserID, id, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "transfers", after.ID, &after.ToOfficeID, audit.Diff(toResponse(before), toResponse(after)))

	// BAST document (best-effort): metadata row + optional MinIO file. Failures here do
	// not roll back the physical receive (asset already relocated + bast_no recorded).
	h.recordBAST(c, after)
	c.JSON(http.StatusOK, toResponse(after))
}

// recordBAST creates an asset_documents(bast_transfer) row and, if a file part is present,
// stores it in MinIO via the asset document service.
func (h *Handler) recordBAST(c *gin.Context, t sqlc.TransferAssetTransfer) {
	uid := t.ReceivedByID
	doc, err := h.assetSvc.CreateDocument(c.Request.Context(), asset.DocumentInput{
		AssetID:          t.AssetID,
		DocType:          sqlc.SharedAssetDocumentTypeBastTransfer,
		DocNo:            t.BastNo,
		DocDate:          t.ReceivedDate,
		RelatedRequestID: t.RequestID,
		CreatedBy:        deref(uid),
	})
	if err != nil {
		return // soft-fail; the transfer already succeeded
	}
	fh, ferr := c.FormFile("file")
	if ferr != nil || fh == nil {
		return // no file uploaded
	}
	f, oerr := fh.Open()
	if oerr != nil {
		return
	}
	defer f.Close()
	data, rerr := io.ReadAll(f)
	if rerr != nil {
		return
	}
	_, _ = h.assetSvc.AttachFile(c.Request.Context(), doc, asset.DocumentFileInput{
		ContentType: fh.Header.Get("Content-Type"),
		Data:        data,
	})
}

func deref(u *uuid.UUID) uuid.UUID {
	if u == nil {
		return uuid.Nil
	}
	return *u
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
	t, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(t))
}

func (h *Handler) list(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	status := c.Query("status")
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.List(c.Request.Context(), all, ids, status, limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		data = append(data, toResponse(t))
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
	for _, t := range rows {
		data = append(data, toResponse(t))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
