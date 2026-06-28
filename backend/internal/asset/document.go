package asset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

// DocumentInput holds the metadata for creating an asset document.
type DocumentInput struct {
	AssetID          uuid.UUID
	DocType          sqlc.SharedAssetDocumentType
	DocNo            *string
	DocDate          pgtype.Date
	Counterparty     *string
	RelatedRequestID *uuid.UUID
	CreatedBy        uuid.UUID
}

// DocumentUpdateInput holds the editable metadata for an asset document.
type DocumentUpdateInput struct {
	DocType          sqlc.SharedAssetDocumentType
	DocNo            *string
	DocDate          pgtype.Date
	Counterparty     *string
	RelatedRequestID *uuid.UUID
}

// DocumentFileInput carries an uploaded document file.
type DocumentFileInput struct {
	ContentType string
	Data        []byte
}

// CreateDocument inserts a document metadata row (no file yet).
func (s *Service) CreateDocument(ctx context.Context, in DocumentInput) (sqlc.AssetAssetDocument, error) {
	cb := in.CreatedBy
	row, err := s.q.CreateAssetDocument(ctx, sqlc.CreateAssetDocumentParams{
		AssetID:          in.AssetID,
		DocType:          in.DocType,
		DocNo:            in.DocNo,
		DocDate:          in.DocDate,
		Counterparty:     in.Counterparty,
		RelatedRequestID: in.RelatedRequestID,
		CreatedByID:      &cb,
	})
	return row, mapDBError(err)
}

// ListDocuments returns all non-deleted documents for an asset (newest first).
func (s *Service) ListDocuments(ctx context.Context, assetID uuid.UUID) ([]sqlc.AssetAssetDocument, error) {
	rows, err := s.q.ListAssetDocuments(ctx, assetID)
	return rows, mapDBError(err)
}

// GetDocument returns a single document by ID, or ErrNotFound.
func (s *Service) GetDocument(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetDocument, error) {
	row, err := s.q.GetAssetDocument(ctx, id)
	return row, mapDBError(err)
}

// UpdateDocument applies metadata edits and returns before/after for audit diffing.
func (s *Service) UpdateDocument(ctx context.Context, id uuid.UUID, in DocumentUpdateInput) (before, after sqlc.AssetAssetDocument, err error) {
	before, err = s.q.GetAssetDocument(ctx, id)
	if err != nil {
		return before, before, mapDBError(err)
	}
	after, err = s.q.UpdateAssetDocument(ctx, sqlc.UpdateAssetDocumentParams{
		ID:               id,
		DocType:          in.DocType,
		DocNo:            in.DocNo,
		DocDate:          in.DocDate,
		Counterparty:     in.Counterparty,
		RelatedRequestID: in.RelatedRequestID,
	})
	return before, after, mapDBError(err)
}

// DeleteDocument soft-deletes a document and best-effort removes its stored file.
func (s *Service) DeleteDocument(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetDocument, error) {
	doc, err := s.q.GetAssetDocument(ctx, id)
	if err != nil {
		return doc, mapDBError(err)
	}
	n, err := s.q.SoftDeleteAssetDocument(ctx, id)
	if err != nil {
		return doc, mapDBError(err)
	}
	if n == 0 {
		return doc, ErrNotFound
	}
	if doc.ObjectKey != nil {
		_ = s.store.Remove(ctx, *doc.ObjectKey)
	}
	return doc, nil
}

// AttachFile validates and stores the file, updates object_key, and best-effort removes
// any previously stored object. Validation fires before any storage/DB call.
func (s *Service) AttachFile(ctx context.Context, doc sqlc.AssetAssetDocument, in DocumentFileInput) (sqlc.AssetAssetDocument, error) {
	var zero sqlc.AssetAssetDocument
	if !allowedMIME(in.ContentType) {
		return zero, ErrUnsupportedType
	}
	if int64(len(in.Data)) > s.maxBytes {
		return zero, ErrTooLarge
	}

	newKey := fmt.Sprintf("assets/%s/documents/%s.%s", doc.AssetID, doc.ID, extFor(in.ContentType))
	if err := s.store.Put(ctx, newKey, bytes.NewReader(in.Data), int64(len(in.Data)), in.ContentType); err != nil {
		return zero, err
	}

	row, err := s.q.SetAssetDocumentObjectKey(ctx, sqlc.SetAssetDocumentObjectKeyParams{
		ID:        doc.ID,
		ObjectKey: &newKey,
	})
	if err != nil {
		_ = s.store.Remove(ctx, newKey) // rollback the just-uploaded object
		return zero, mapDBError(err)
	}

	// Remove the previous object only when the key actually changed (same ext => same key).
	if doc.ObjectKey != nil && *doc.ObjectKey != newKey {
		_ = s.store.Remove(ctx, *doc.ObjectKey)
	}
	return row, nil
}

// OpenDocumentFile returns a reader for the document's file, or ErrNotFound when the
// document has no file or the object is missing.
func (s *Service) OpenDocumentFile(ctx context.Context, doc sqlc.AssetAssetDocument) (io.ReadCloser, storage.ObjectInfo, error) {
	if doc.ObjectKey == nil {
		return nil, storage.ObjectInfo{}, ErrNotFound
	}
	rc, info, err := s.store.Get(ctx, *doc.ObjectKey)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return nil, storage.ObjectInfo{}, ErrNotFound
	}
	return rc, info, err
}
