package asset

import (
	"errors"
	"io"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// resolveDoc loads the :docId document and verifies it belongs to assetID.
func (h *Handler) resolveDoc(c *gin.Context, assetID uuid.UUID) (sqlc.AssetAssetDocument, bool) {
	docID, err := uuid.Parse(c.Param("docId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return sqlc.AssetAssetDocument{}, false
	}
	doc, err := h.svc.GetDocument(c.Request.Context(), docID)
	if err != nil {
		h.handleErr(c, err)
		return sqlc.AssetAssetDocument{}, false
	}
	if doc.AssetID != assetID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return sqlc.AssetAssetDocument{}, false
	}
	return doc, true
}

// docDownloadName derives a safe download filename: doc_no (or doc_type) + the stored extension.
func docDownloadName(d sqlc.AssetAssetDocument) string {
	base := string(d.DocType)
	if d.DocNo != nil && *d.DocNo != "" {
		base = *d.DocNo
	}
	ext := ""
	if d.ObjectKey != nil {
		ext = path.Ext(*d.ObjectKey)
	}
	return base + ext
}

// createDocument handles POST /assets/:id/documents.
func (h *Handler) createDocument(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	var req DocumentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, _ := uuid.Parse(c.GetString(middleware.CtxUserID))
	in, err := req.toInput(assetID, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := h.svc.CreateDocument(c.Request.Context(), in)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionCreate, "asset_documents", row.ID, &oid,
		audit.Diff(nil, documentToMap(row)))
	c.JSON(http.StatusCreated, documentToMap(row))
}

// listDocuments handles GET /assets/:id/documents.
func (h *Handler) listDocuments(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListDocuments(c.Request.Context(), assetID)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, documentToMap(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

// getDocument handles GET /assets/:id/documents/:docId.
func (h *Handler) getDocument(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, documentToMap(doc))
}

// updateDocument handles PUT /assets/:id/documents/:docId.
func (h *Handler) updateDocument(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	var req DocumentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := req.toUpdateInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, after, err := h.svc.UpdateDocument(c.Request.Context(), doc.ID, in)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionUpdate, "asset_documents", after.ID, &oid,
		audit.Diff(documentToMap(before), documentToMap(after)))
	c.JSON(http.StatusOK, documentToMap(after))
}

// deleteDocument handles DELETE /assets/:id/documents/:docId.
func (h *Handler) deleteDocument(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	if _, err := h.svc.DeleteDocument(c.Request.Context(), doc.ID); err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionDelete, "asset_documents", doc.ID, &oid,
		audit.Diff(documentToMap(doc), nil))
	c.Status(http.StatusNoContent)
}

// uploadDocumentFile handles PUT /assets/:id/documents/:docId/file (multipart, field "file").
func (h *Handler) uploadDocumentFile(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.maxBytes+1)
	fileHeader, err := c.FormFile("file")
	if err != nil {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	before := documentToMap(doc)
	row, err := h.svc.AttachFile(c.Request.Context(), doc, DocumentFileInput{ContentType: contentType, Data: data})
	if err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionUpdate, "asset_documents", row.ID, &oid,
		audit.Diff(before, documentToMap(row)))
	c.JSON(http.StatusOK, documentToMap(row))
}

// downloadDocumentFile handles GET /assets/:id/documents/:docId/file.
func (h *Handler) downloadDocumentFile(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	rc, info, err := h.svc.OpenDocumentFile(c.Request.Context(), doc)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	defer rc.Close()
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Content-Disposition", contentDisposition(docDownloadName(doc)))
	c.DataFromReader(http.StatusOK, info.Size, info.ContentType, rc, nil)
}
