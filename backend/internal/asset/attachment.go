package asset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

var (
	ErrUnsupportedType = errors.New("unsupported attachment type")
	ErrTooLarge        = errors.New("attachment exceeds size limit")
)

var mimeExt = map[string]string{
	"image/jpeg":      "jpg",
	"image/png":       "png",
	"image/webp":      "webp",
	"application/pdf": "pdf",
}

func allowedMIME(m string) bool { _, ok := mimeExt[m]; return ok }
func extFor(m string) string    { return mimeExt[m] }

func kindFor(m string) sqlc.SharedAttachmentKind {
	if strings.HasPrefix(m, "image/") {
		return sqlc.SharedAttachmentKindPhoto
	}
	return sqlc.SharedAttachmentKindDocument
}

// UploadInput holds the parameters for uploading an asset attachment.
type UploadInput struct {
	AssetID     uuid.UUID
	Filename    string
	ContentType string
	Data        []byte
	CreatedBy   uuid.UUID
	// Normalize, when true, re-encodes an image upload to a size-bounded JPEG
	// (compressing storage + stripping EXIF) before it is stored. Used for
	// mobile field photos (damage reports); leave false to store the original
	// bytes unchanged (e.g. web attachments that may be lossless scans/PDFs).
	Normalize bool
}

// UploadAttachment validates the MIME type and size, stores the object (and a
// thumbnail for images), then records the attachment row. On DB failure the
// uploaded objects are removed as a best-effort rollback.
func (s *Service) UploadAttachment(ctx context.Context, in UploadInput) (sqlc.AssetAssetAttachment, error) {
	var zero sqlc.AssetAssetAttachment

	// Validation MUST happen before any DB/storage call so callers with q=nil
	// can rely on these sentinel errors being returned immediately.
	if !allowedMIME(in.ContentType) {
		return zero, ErrUnsupportedType
	}
	if int64(len(in.Data)) > s.maxBytes {
		return zero, ErrTooLarge
	}

	kind := kindFor(in.ContentType)

	// Optionally compress an image upload before storing: downscale to bound
	// storage and re-encode to JPEG (which strips EXIF/GPS), preserving quality.
	// Only images are touched; PDFs/other types are stored as-is.
	if in.Normalize && kind == sqlc.SharedAttachmentKindPhoto {
		norm, err := normalizeImage(in.Data)
		if err != nil {
			// Image MIME declared but data is not decodable.
			return zero, ErrUnsupportedType
		}
		in.Data = norm
		in.ContentType = "image/jpeg"
	}

	id := uuid.New()
	objectKey := fmt.Sprintf("assets/%s/%s.%s", in.AssetID, id, extFor(in.ContentType))

	// For images, generate and store a thumbnail first.
	var thumbKey *string
	if kind == sqlc.SharedAttachmentKindPhoto {
		thumb, err := makeThumbnail(in.Data)
		if err != nil {
			// Image MIME declared but data is not decodable.
			return zero, ErrUnsupportedType
		}
		tk := fmt.Sprintf("assets/%s/%s_thumb.jpg", in.AssetID, id)
		if err := s.store.Put(ctx, tk, bytes.NewReader(thumb), int64(len(thumb)), "image/jpeg"); err != nil {
			return zero, err
		}
		thumbKey = &tk
	}

	// Store the original object.
	if err := s.store.Put(ctx, objectKey, bytes.NewReader(in.Data), int64(len(in.Data)), in.ContentType); err != nil {
		if thumbKey != nil {
			_ = s.store.Remove(ctx, *thumbKey)
		}
		return zero, err
	}

	// Persist the attachment record.
	createdBy := in.CreatedBy
	row, err := s.q.CreateAttachment(ctx, sqlc.CreateAttachmentParams{
		AssetID:          in.AssetID,
		Kind:             kind,
		ObjectKey:        objectKey,
		ThumbnailKey:     thumbKey,
		OriginalFilename: in.Filename,
		SizeBytes:        int64(len(in.Data)),
		MimeType:         in.ContentType,
		CreatedByID:      &createdBy,
	})
	if err != nil {
		// Best-effort rollback: remove uploaded objects so storage stays clean.
		_ = s.store.Remove(ctx, objectKey)
		if thumbKey != nil {
			_ = s.store.Remove(ctx, *thumbKey)
		}
		return zero, mapDBError(err)
	}
	return row, nil
}

// ListAttachments returns all non-deleted attachments for the given asset.
func (s *Service) ListAttachments(ctx context.Context, assetID uuid.UUID) ([]sqlc.AssetAssetAttachment, error) {
	rows, err := s.q.ListAttachments(ctx, assetID)
	return rows, mapDBError(err)
}

// GetAttachment returns a single attachment by ID, or ErrNotFound.
func (s *Service) GetAttachment(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetAttachment, error) {
	row, err := s.q.GetAttachment(ctx, id)
	return row, mapDBError(err)
}

// OpenAttachment returns a read-closer for the attachment object. When thumb is
// true and the attachment has no thumbnail, ErrNotFound is returned.
// storage.ErrObjectNotFound is mapped to ErrNotFound.
func (s *Service) OpenAttachment(ctx context.Context, att sqlc.AssetAssetAttachment, thumb bool) (io.ReadCloser, storage.ObjectInfo, error) {
	key := att.ObjectKey
	if thumb {
		if att.ThumbnailKey == nil {
			return nil, storage.ObjectInfo{}, ErrNotFound
		}
		key = *att.ThumbnailKey
	}
	rc, info, err := s.store.Get(ctx, key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return nil, storage.ObjectInfo{}, ErrNotFound
	}
	return rc, info, err
}

// DeleteAttachment soft-deletes an attachment record and best-effort removes its
// stored objects. Returns ErrNotFound if the attachment does not exist.
func (s *Service) DeleteAttachment(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetAttachment, error) {
	att, err := s.q.GetAttachment(ctx, id)
	if err != nil {
		return att, mapDBError(err)
	}
	n, err := s.q.SoftDeleteAttachment(ctx, id)
	if err != nil {
		return att, mapDBError(err)
	}
	if n == 0 {
		return att, ErrNotFound
	}
	// Best-effort object removal.
	_ = s.store.Remove(ctx, att.ObjectKey)
	if att.ThumbnailKey != nil {
		_ = s.store.Remove(ctx, *att.ThumbnailKey)
	}
	return att, nil
}
