package identity

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// avatarErr maps the avatar service's sentinel errors onto HTTP status codes.
func avatarErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnsupportedType):
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "hanya file JPG atau PNG yang didukung"})
	case errors.Is(err, ErrTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "ukuran file melebihi batas"})
	case errors.Is(err, ErrNoAvatar):
		c.JSON(http.StatusNotFound, gin.H{"error": "no avatar set"})
	case errors.Is(err, ErrAvatarUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "avatar storage unavailable"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// uploadAvatar stores the caller's profile photo (multipart field "file").
func (h *Handler) uploadAvatar(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	// Cap the whole multipart body one byte above the limit so an oversize
	// upload is refused while streaming rather than buffered in full.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.avatarMaxBytes+1)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "ukuran file melebihi batas"})
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

	profile, err := h.svc.UploadAvatar(c.Request.Context(), userID, data, contentType)
	if err != nil {
		avatarErr(c, err)
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", userID, officeIDFromView(profile.OfficeID), gin.H{"event": "avatar_updated"})
	c.JSON(http.StatusOK, profile)
}

// removeAvatar clears the caller's profile photo.
func (h *Handler) removeAvatar(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	profile, err := h.svc.RemoveAvatar(c.Request.Context(), userID)
	if err != nil {
		avatarErr(c, err)
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", userID, officeIDFromView(profile.OfficeID), gin.H{"event": "avatar_removed"})
	c.JSON(http.StatusOK, profile)
}

// getAvatar streams the caller's stored profile photo. The endpoint is
// authenticated, so the image cannot be used as a bare <img src> — the frontend
// fetches it as a blob (same as asset attachment thumbnails).
func (h *Handler) getAvatar(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	rc, info, err := h.svc.GetAvatar(c.Request.Context(), userID)
	if err != nil {
		avatarErr(c, err)
		return
	}
	defer rc.Close()
	ct := info.ContentType
	if ct == "" {
		ct = "image/jpeg"
	}
	// Same hardening as attachment downloads: never let the browser sniff a
	// different type, and never let the response act as an active document.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Cache-Control", "private, no-store")
	c.DataFromReader(http.StatusOK, info.Size, ct, rc, nil)
}
