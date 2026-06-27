package asset

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// contentDisposition returns a safe RFC 6266 inline Content-Disposition header value.
// It strips CR/LF control characters and lets mime.FormatMediaType handle quote/non-ASCII
// encoding so that user-controlled filenames cannot inject raw header bytes.
func contentDisposition(filename string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, filename)
	v := mime.FormatMediaType("inline", map[string]string{"filename": clean})
	if v == "" {
		return `inline; filename="download"`
	}
	return v
}

// resolveAssetInScope loads the asset for :id and enforces the caller's "assets" office scope.
// Returns the assetID, officeID and true if access is allowed; otherwise writes the error
// response and returns ok=false.
func (h *Handler) resolveAssetInScope(c *gin.Context) (assetID uuid.UUID, officeID uuid.UUID, ok bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if !common.InScope(all, ids, a.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	return a.ID, a.OfficeID, true
}

// handleErr maps asset/attachment sentinel errors to HTTP status codes.
func (h *Handler) handleErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnsupportedType):
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
	case errors.Is(err, ErrTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		common.WriteError(c, err)
	}
}

// uploadAttachment handles POST /assets/:id/attachments.
// It caps the request body, reads the multipart file, detects the MIME type,
// uploads via the service, records an audit entry, and returns 201 with the attachment map.
func (h *Handler) uploadAttachment(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}

	// Cap the request body to maxBytes+1 so we detect oversize without buffering unbounded data.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.maxBytes+1)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		// MaxBytesReader fires during multipart parsing when the body exceeds the limit.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field"})
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
		// MaxBytesReader causes ReadAll to fail when the limit is exceeded.
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	uid, _ := uuid.Parse(c.GetString(middleware.CtxUserID))
	row, err := h.svc.UploadAttachment(c.Request.Context(), UploadInput{
		AssetID:     assetID,
		Filename:    fileHeader.Filename,
		ContentType: contentType,
		Data:        data,
		CreatedBy:   uid,
	})
	if err != nil {
		h.handleErr(c, err)
		return
	}

	oid := officeID
	audit.Record(c, h.aud, audit.ActionCreate, "asset_attachments", row.ID, &oid,
		audit.Diff(nil, attachmentToMap(row)))
	c.JSON(http.StatusCreated, attachmentToMap(row))
}

// listAttachments handles GET /assets/:id/attachments.
func (h *Handler) listAttachments(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListAttachments(c.Request.Context(), assetID)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, attachmentToMap(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

// streamAttachment is the shared implementation for download and thumbnail endpoints.
// When thumb is true it streams the thumbnail; otherwise it streams the original file.
func (h *Handler) streamAttachment(c *gin.Context, thumb bool) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	aid, err := uuid.Parse(c.Param("aid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid attachment id"})
		return
	}
	att, err := h.svc.GetAttachment(c.Request.Context(), aid)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	// Verify the attachment belongs to the asset resolved from :id.
	if att.AssetID != assetID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	rc, info, err := h.svc.OpenAttachment(c.Request.Context(), att, thumb)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	defer rc.Close()

	ct := info.ContentType
	if thumb {
		ct = "image/jpeg"
	} else if att.MimeType != "" {
		ct = att.MimeType
	}
	// Anti-sniffing + sandbox: never let the browser execute served bytes.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Content-Disposition", contentDisposition(att.OriginalFilename))
	c.DataFromReader(http.StatusOK, info.Size, ct, rc, nil)
}

// downloadAttachment handles GET /assets/:id/attachments/:aid/content.
func (h *Handler) downloadAttachment(c *gin.Context) { h.streamAttachment(c, false) }

// downloadThumbnail handles GET /assets/:id/attachments/:aid/thumbnail.
func (h *Handler) downloadThumbnail(c *gin.Context) { h.streamAttachment(c, true) }

// deleteAttachment handles DELETE /assets/:id/attachments/:aid.
func (h *Handler) deleteAttachment(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	aid, err := uuid.Parse(c.Param("aid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid attachment id"})
		return
	}
	att, err := h.svc.GetAttachment(c.Request.Context(), aid)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	if att.AssetID != assetID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if _, err := h.svc.DeleteAttachment(c.Request.Context(), aid); err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionDelete, "asset_attachments", aid, &oid,
		audit.Diff(attachmentToMap(att), nil))
	c.Status(http.StatusNoContent)
}
