package disposal

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

const scopeModule = "disposals"

// Handler maps HTTP ↔ the disposal service, orchestrating BAST document creation.
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
	case errors.Is(err, ErrAlreadyDisposed), errors.Is(err, ErrDisposalExists), errors.Is(err, ErrInvalidRef):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// caller builds an approval.Caller from the Gin context, mirroring
// internal/transfer/handler.go's caller() (CtxUserID/CtxRoleID are string
// context keys read via c.GetString, then parsed as UUIDs).
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
	assetID, perr := uuid.Parse(req.AssetID)
	if perr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset_id"})
		return
	}
	out, err := h.svc.Submit(c.Request.Context(), caller, SubmitInput{
		AssetID: assetID, Method: req.Method, DisposalDate: req.DisposalDate,
		Proceeds: req.Proceeds, BookValue: req.BookValue, BastNo: req.BastNo, Reason: req.Reason,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionCreate, "disposals", out.ID, out.OfficeID,
		audit.Diff(nil, map[string]any{"request_id": out.ID.String(), "type": "asset_disposal", "asset_id": req.AssetID}))
	c.JSON(http.StatusCreated, gin.H{"request_id": out.ID.String(), "status": string(out.Status)})
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
	d, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(d))
}

func (h *Handler) list(c *gin.Context) {
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	rows, total, err := h.svc.List(c.Request.Context(), all, ids, limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		data = append(data, toResponse(d))
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
	for _, d := range rows {
		data = append(data, toResponse(d))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// attachDocument creates the BAST-disposal document metadata row (and, best-effort,
// stores an uploaded file + persists bast_no on the disposal).
func (h *Handler) attachDocument(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body DocumentRequest
	_ = c.ShouldBind(&body)
	_, all, ids, err := h.caller(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	d, err := h.svc.Get(c.Request.Context(), id, all, ids)
	if err != nil {
		h.svcError(c, err)
		return
	}
	docDate, derr := parseDate(derefStr(body.DocDate))
	if derr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doc_date"})
		return
	}
	disposalID := d.ID
	doc, err := h.assetSvc.CreateDocument(c.Request.Context(), asset.DocumentInput{
		AssetID:           d.AssetID,
		DocType:           sqlc.SharedAssetDocumentTypeBastDisposal,
		DocNo:             body.DocNo,
		DocDate:           docDate,
		Counterparty:      body.Counterparty,
		RelatedRequestID:  d.RequestID,
		RelatedDisposalID: &disposalID,
		CreatedBy:         derefUUID(d.CreatedByID),
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	// Best-effort file (a failure does not fail the metadata creation).
	if fh, ferr := c.FormFile("file"); ferr == nil && fh != nil {
		if f, oerr := fh.Open(); oerr == nil {
			defer f.Close()
			if data, rerr := io.ReadAll(f); rerr == nil {
				_, _ = h.assetSvc.AttachFile(c.Request.Context(), doc, asset.DocumentFileInput{
					ContentType: fh.Header.Get("Content-Type"), Data: data,
				})
			}
		}
	}
	// Persist bast_no on the disposal if provided.
	if body.BastNo != nil {
		_, _ = h.svc.setBastNo(c.Request.Context(), id, *body.BastNo)
	}
	c.JSON(http.StatusOK, gin.H{"document_id": doc.ID.String(), "disposal_id": id.String()})
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefUUID(p *uuid.UUID) uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return *p
}
