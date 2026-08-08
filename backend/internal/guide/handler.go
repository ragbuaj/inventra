package guide

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// permManage is the write permission for guide content. Reading attachments
// needs only a session; this key governs authoring.
const permManage = "guide.manage"

// Handler binds the guide service to HTTP.
type Handler struct {
	svc  *Service
	perm *authz.PermissionService
	aud  *audit.Service
}

func NewHandler(svc *Service, perm *authz.PermissionService, aud *audit.Service) *Handler {
	return &Handler{svc: svc, perm: perm, aud: aud}
}

// svcError maps service sentinels to HTTP status. Status lives here, never in
// the service.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, ErrSlugExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidRef), errors.Is(err, ErrInvalidSteps):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidVideoURL):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, ErrTooManyAttachments):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUnsupportedType):
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
	case errors.Is(err, ErrTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// caller reports whether the request carries a session and whether that session
// may author guide content. Behind OptionalAuth both may be false; behind
// RequireAuth signedIn is always true.
func (h *Handler) caller(c *gin.Context) (signedIn, canManage bool) {
	signedIn = c.GetString(middleware.CtxUserID) != ""
	if !signedIn || h.perm == nil {
		return signedIn, false
	}
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return signedIn, false
	}
	ok, err := h.perm.Has(c.Request.Context(), roleID, permManage)
	if err != nil {
		// Conservative: a permission lookup we cannot complete is not authority.
		return signedIn, false
	}
	return signedIn, ok
}

func (h *Handler) actor(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString(middleware.CtxUserID))
	return id
}

// listModules handles GET /guide/modules for everyone: guests, signed-in
// readers, and authors.
//
// The response varies by Authorization, so it must never be cached by anything
// in front of us — a shared cache could otherwise hand a signed-in reader's
// payload (video ids, file links) to the next anonymous visitor.
func (h *Handler) listModules(c *gin.Context) {
	signedIn, canManage := h.caller(c)

	// Drafts are gated on authority, not on the parameter: asking for them
	// without guide.manage simply yields the published set.
	includeDraft := canManage && strings.EqualFold(c.Query("status"), "all")

	mods, err := h.svc.List(c.Request.Context(), includeDraft)
	if err != nil {
		h.svcError(c, err)
		return
	}

	data := make([]moduleResponse, 0, len(mods))
	for _, m := range mods {
		data = append(data, toModuleResponse(m, signedIn))
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Vary", "Authorization")
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

// getModule backs the author's editor; it always includes drafts and media.
func (h *Handler) getModule(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, toModuleResponse(m, true))
}

func (h *Handler) createModule(c *gin.Context) {
	var req moduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	in := req.toInput()
	in.Actor = h.actor(c)

	row, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	resp := moduleOnlyResponse(row)
	audit.Record(c, h.aud, audit.ActionCreate, "guide_modules", row.ID, nil, audit.Diff(nil, resp))
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) updateModule(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req moduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	before, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	in := req.toInput()
	in.Actor = h.actor(c)

	row, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		h.svcError(c, err)
		return
	}
	resp := moduleOnlyResponse(row)
	audit.Record(c, h.aud, audit.ActionUpdate, "guide_modules", id, nil,
		audit.Diff(moduleOnlyResponse(before.Module), resp))
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) deleteModule(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	before, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	if _, err := h.svc.Delete(c.Request.Context(), id, h.actor(c)); err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "guide_modules", id, nil,
		audit.Diff(moduleOnlyResponse(before.Module), nil))
	c.Status(http.StatusNoContent)
}

func (h *Handler) addVideo(c *gin.Context) {
	moduleID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req videoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	row, err := h.svc.AddVideo(c.Request.Context(), moduleID, VideoInput{
		URL:       req.URL,
		TitleID:   req.TitleID,
		TitleEn:   req.TitleEn,
		SortOrder: req.SortOrder,
		Actor:     h.actor(c),
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	resp := toAttachmentResponse(row, true)
	audit.Record(c, h.aud, audit.ActionCreate, "guide_attachments", row.ID, nil, audit.Diff(nil, resp))
	c.JSON(http.StatusCreated, resp)
}

// multipartOverheadAllowance is the headroom the whole request body gets over
// the per-file limit.
//
// The body carries the file PLUS the multipart boundaries, the part headers, and
// the three text fields, so capping it at exactly the file limit would reject a
// file that is precisely AT the limit — the very size the upload form advertises.
// The per-file check in the service is the rule; this cap is only a bound on how
// much we are willing to read. It stays far below the roughly 12.5 MB request
// body ceiling the production WAF enforces (see config.GuidePDFMaxBytes).
const multipartOverheadAllowance = 1 << 20 // 1 MiB

// limitAwareBody records that the body cap was hit.
//
// http.MaxBytesReader on its own is not enough to answer correctly. mime/multipart
// reads the body through it, and when the cap trips while PART HEADERS are still
// being parsed, the parser fails with "malformed MIME header: missing colon" and
// the *http.MaxBytesError never reaches the handler — an upload that is merely
// too large would then be reported as 422 malformed input. Recording the trip
// where it happens survives whatever the parser makes of the error afterwards.
type limitAwareBody struct {
	io.ReadCloser
	tripped bool
}

func (b *limitAwareBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		b.tripped = true
	}
	return n, err
}

// addDocument handles the PDF upload. The body is capped before parsing so an
// oversized request is rejected without being buffered whole.
func (h *Handler) addDocument(c *gin.Context) {
	moduleID, ok := parseID(c, "id")
	if !ok {
		return
	}

	body := &limitAwareBody{
		ReadCloser: http.MaxBytesReader(c.Writer, c.Request.Body,
			h.svc.MaxBytes()+multipartOverheadAllowance),
	}
	c.Request.Body = body

	var req documentRequest
	if err := c.ShouldBind(&req); err != nil {
		if body.tripped {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": ErrTooLarge.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if body.tripped {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": ErrTooLarge.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "missing file field"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}

	row, err := h.svc.AddDocument(c.Request.Context(), moduleID, DocumentInput{
		Filename:  fileHeader.Filename,
		Data:      data,
		TitleID:   req.TitleID,
		TitleEn:   req.TitleEn,
		SortOrder: req.SortOrder,
		Actor:     h.actor(c),
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	resp := toAttachmentResponse(row, true)
	audit.Record(c, h.aud, audit.ActionCreate, "guide_attachments", row.ID, nil, audit.Diff(nil, resp))
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) patchAttachment(c *gin.Context) {
	id, ok := parseID(c, "aid")
	if !ok {
		return
	}
	var req attachmentPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	before, err := h.svc.GetAttachment(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	row, err := h.svc.UpdateAttachment(c.Request.Context(), id, req.TitleID, req.TitleEn, req.SortOrder)
	if err != nil {
		h.svcError(c, err)
		return
	}
	resp := toAttachmentResponse(row, true)
	audit.Record(c, h.aud, audit.ActionUpdate, "guide_attachments", id, nil,
		audit.Diff(toAttachmentResponse(before, true), resp))
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) deleteAttachment(c *gin.Context) {
	id, ok := parseID(c, "aid")
	if !ok {
		return
	}
	before, err := h.svc.GetAttachment(c.Request.Context(), id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	if _, err := h.svc.DeleteAttachment(c.Request.Context(), id); err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionDelete, "guide_attachments", id, nil,
		audit.Diff(toAttachmentResponse(before, true), nil))
	c.Status(http.StatusNoContent)
}

// downloadAttachment streams the stored PDF. It sits behind RequireAuth, and a
// session is *almost* the whole access rule: media on a PUBLISHED module is
// readable by any signed-in user, but media on a draft follows its module and
// stays with the authors.
//
// That second half matters because withdrawing a module has to withdraw its
// files too. Every reader who loaded the page while it was published holds the
// attachment ids, so if the rule were "any session", pulling a module back to
// draft would remove the text and leave the PDF downloadable forever — which is
// exactly the containment step the spec relies on for a document published by
// mistake. Neither the id nor the object key is a capability.
func (h *Handler) downloadAttachment(c *gin.Context) {
	id, ok := parseID(c, "aid")
	if !ok {
		return
	}
	_, canManage := h.caller(c)
	att, err := h.svc.GetAttachmentForRead(c.Request.Context(), id, canManage)
	if err != nil {
		h.svcError(c, err)
		return
	}
	rc, info, err := h.svc.OpenAttachment(c.Request.Context(), att)
	if err != nil {
		h.svcError(c, err)
		return
	}
	defer rc.Close()

	name := "panduan.pdf"
	if att.OriginalFilename != nil && *att.OriginalFilename != "" {
		name = *att.OriginalFilename
	}
	// Content-Type comes from us, never from the uploader. nosniff + sandbox stop
	// the browser from executing whatever the bytes turn out to be.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", contentDisposition(name))
	c.DataFromReader(http.StatusOK, info.Size, "application/pdf", rc, nil)
}

func parseID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, false
	}
	return id, true
}

// contentDisposition builds a safe RFC 6266 inline value: CR/LF are stripped and
// mime.FormatMediaType handles quoting, so an uploader-supplied filename cannot
// inject raw header bytes. Mirrors the asset module's helper.
func contentDisposition(filename string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, filename)
	v := mime.FormatMediaType("inline", map[string]string{"filename": clean})
	if v == "" {
		return `inline; filename="panduan.pdf"`
	}
	return v
}
