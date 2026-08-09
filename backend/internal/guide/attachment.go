package guide

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

// maxAttachmentsPerModule caps one module's media list. A guide section that
// needs more than ten videos and documents is really several sections.
const maxAttachmentsPerModule = 10

// objectPrefix keeps guide PDFs distinguishable from asset attachments inside
// the shared bucket, so a storage audit can tell them apart.
const objectPrefix = "guide/"

// isPDF verifies the upload from its OWN BYTES. Filename, extension, and the
// Content-Type header all come from the uploader and are therefore worthless as
// evidence — a .pdf whose body is HTML is the oldest trick in this category.
func isPDF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("%PDF-"))
}

// VideoInput adds a YouTube link. The raw URL is validated and reduced to its
// video id; the URL itself is never stored and never rendered.
type VideoInput struct {
	URL       string
	TitleID   string
	TitleEn   *string
	SortOrder int32
	Actor     uuid.UUID
}

// DocumentInput adds a PDF. Filename is display metadata only — the object key
// is generated here, so nothing the uploader typed can shape a storage path.
type DocumentInput struct {
	Filename  string
	Data      []byte
	TitleID   string
	TitleEn   *string
	SortOrder int32
	Actor     uuid.UUID
}

// requireModule loads a live module. The FK alone would not do: it still points
// at soft-deleted modules, and attaching media to a deleted module should read
// as "not found", not as success.
func (s *Service) requireModule(ctx context.Context, moduleID uuid.UUID) error {
	if _, err := s.q.GetGuideModule(ctx, moduleID); err != nil {
		return mapDBError(err)
	}
	return nil
}

// AddVideo validates the link, enforces the per-module cap, and records the row.
func (s *Service) AddVideo(ctx context.Context, moduleID uuid.UUID, in VideoInput) (sqlc.GuideGuideAttachment, error) {
	var zero sqlc.GuideGuideAttachment

	videoID, err := ParseYouTubeID(in.URL)
	if err != nil {
		return zero, err
	}
	if strings.TrimSpace(in.TitleID) == "" {
		return zero, ErrInvalidRef
	}
	if err := s.requireModule(ctx, moduleID); err != nil {
		return zero, err
	}

	actor := in.Actor
	return s.insertCapped(ctx, sqlc.CreateGuideAttachmentParams{
		ModuleID:  moduleID,
		Kind:      sqlc.SharedGuideAttachmentKindVideo,
		TitleID:   strings.TrimSpace(in.TitleID),
		TitleEn:   trimPtr(in.TitleEn),
		SortOrder: in.SortOrder,
		YoutubeID: &videoID,
		Actor:     &actor,
	})
}

// AddDocument stores the PDF then records it.
//
// Order matters: the object is written first so a committed row can never point
// at a missing file. If the insert then fails — including when it trips the cap —
// the object is removed again, so neither half survives alone.
func (s *Service) AddDocument(ctx context.Context, moduleID uuid.UUID, in DocumentInput) (sqlc.GuideGuideAttachment, error) {
	var zero sqlc.GuideGuideAttachment

	// Validation before any storage or DB call, so callers get these sentinels
	// without side effects.
	if !isPDF(in.Data) {
		return zero, ErrUnsupportedType
	}
	if int64(len(in.Data)) > s.maxBytes {
		return zero, ErrTooLarge
	}
	if strings.TrimSpace(in.TitleID) == "" {
		return zero, ErrInvalidRef
	}
	if err := s.requireModule(ctx, moduleID); err != nil {
		return zero, err
	}

	key := objectPrefix + uuid.NewString() + ".pdf"
	size := int64(len(in.Data))
	if err := s.store.Put(ctx, key, bytes.NewReader(in.Data), size, "application/pdf"); err != nil {
		return zero, err
	}

	actor := in.Actor
	mime := "application/pdf"
	filename := in.Filename
	row, err := s.insertCapped(ctx, sqlc.CreateGuideAttachmentParams{
		ModuleID:         moduleID,
		Kind:             sqlc.SharedGuideAttachmentKindDocument,
		TitleID:          strings.TrimSpace(in.TitleID),
		TitleEn:          trimPtr(in.TitleEn),
		SortOrder:        in.SortOrder,
		ObjectKey:        &key,
		OriginalFilename: &filename,
		MimeType:         &mime,
		SizeBytes:        &size,
		Actor:            &actor,
	})
	if err != nil {
		s.removeObject(ctx, key, "rollback after failed attachment insert")
		return zero, err
	}
	return row, nil
}

// insertCapped locks the module row, then counts and inserts — all in ONE
// transaction.
//
// The lock is the part that matters, and one transaction alone is NOT enough to
// replace it: under READ COMMITTED a `SELECT count(*)` takes no locks, so
// concurrent uploads all read a number below the cap and all insert. Twenty
// concurrent requests against a cap of ten produced seventeen rows before this
// line existed. The module row is the serialization point because the cap is
// per module; holding it for the length of one small insert costs nothing, and
// the only other writer of that row is an edit of the module itself.
func (s *Service) insertCapped(ctx context.Context, arg sqlc.CreateGuideAttachmentParams) (sqlc.GuideGuideAttachment, error) {
	var zero sqlc.GuideGuideAttachment

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once Commit succeeded

	qtx := s.q.WithTx(tx)
	// Also re-checks the module inside the transaction: requireModule ran before
	// the object was stored, and the module can be deleted between the two.
	if _, err := qtx.LockGuideModuleForUpdate(ctx, arg.ModuleID); err != nil {
		return zero, mapDBError(err)
	}
	n, err := qtx.CountGuideAttachments(ctx, arg.ModuleID)
	if err != nil {
		return zero, mapDBError(err)
	}
	if n >= maxAttachmentsPerModule {
		return zero, ErrTooManyAttachments
	}
	row, err := qtx.CreateGuideAttachment(ctx, arg)
	if err != nil {
		return zero, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return row, nil
}

func (s *Service) GetAttachment(ctx context.Context, id uuid.UUID) (sqlc.GuideGuideAttachment, error) {
	row, err := s.q.GetGuideAttachment(ctx, id)
	return row, mapDBError(err)
}

// GetAttachmentForRead loads an attachment for the file endpoint, where the
// parent module's status is part of the access rule: a draft is an authoring
// workspace, so its media belongs to whoever may author, not to every session.
//
// includeDraft is the caller's RESOLVED authority — the same discipline as
// List: the handler sets it from guide.manage, never from a request parameter.
// The filtering happens inside the query rather than on its result, so a caller
// without that authority cannot tell a draft's attachment from one that was
// never there.
func (s *Service) GetAttachmentForRead(ctx context.Context, id uuid.UUID, includeDraft bool) (sqlc.GuideGuideAttachment, error) {
	row, err := s.q.GetGuideAttachmentForRead(ctx, sqlc.GetGuideAttachmentForReadParams{
		ID:           id,
		IncludeDraft: includeDraft,
	})
	return row, mapDBError(err)
}

// UpdateAttachment changes the title and ordering only. Neither the file nor the
// link can be swapped in place: replacing one means deleting the attachment and
// adding it again, so no row ever ends up pointing at an object it did not
// create.
func (s *Service) UpdateAttachment(ctx context.Context, id uuid.UUID, titleID string, titleEn *string, sortOrder int32) (sqlc.GuideGuideAttachment, error) {
	if strings.TrimSpace(titleID) == "" {
		return sqlc.GuideGuideAttachment{}, ErrInvalidRef
	}
	row, err := s.q.UpdateGuideAttachment(ctx, sqlc.UpdateGuideAttachmentParams{
		ID:        id,
		TitleID:   strings.TrimSpace(titleID),
		TitleEn:   trimPtr(titleEn),
		SortOrder: sortOrder,
	})
	return row, mapDBError(err)
}

// DeleteAttachment soft-deletes the row, then removes the stored object. The row
// goes first: with it gone the file is already unreachable, so a failed object
// removal leaves an orphan blob (logged, harmless) rather than a live row whose
// file has vanished.
func (s *Service) DeleteAttachment(ctx context.Context, id uuid.UUID) (sqlc.GuideGuideAttachment, error) {
	row, err := s.q.SoftDeleteGuideAttachment(ctx, id)
	if err != nil {
		return sqlc.GuideGuideAttachment{}, mapDBError(err)
	}
	if row.ObjectKey != nil {
		s.removeObject(ctx, *row.ObjectKey, "attachment deleted")
	}
	return row, nil
}

// OpenAttachment streams the stored PDF. Authorization is the handler's job —
// this returns bytes to whoever asks.
func (s *Service) OpenAttachment(ctx context.Context, att sqlc.GuideGuideAttachment) (io.ReadCloser, storage.ObjectInfo, error) {
	if att.ObjectKey == nil {
		return nil, storage.ObjectInfo{}, ErrNotFound
	}
	rc, info, err := s.store.Get(ctx, *att.ObjectKey)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, storage.ObjectInfo{}, ErrNotFound
		}
		return nil, storage.ObjectInfo{}, err
	}
	return rc, info, nil
}

// removeObject deletes a stored object, logging the key on failure. An orphan
// blob nobody can name is far worse than one recorded in the log.
func (s *Service) removeObject(ctx context.Context, key, reason string) {
	if err := s.store.Remove(ctx, key); err != nil {
		slog.Error("guide: failed to remove stored object",
			"key", key, "reason", reason, "error", err)
	}
}

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	t := strings.TrimSpace(*p)
	if t == "" {
		return nil
	}
	return &t
}
