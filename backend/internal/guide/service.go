// Package guide implements the data-driven Panduan Penggunaan content: modules
// (title, icon, ordering, draft/published, numbered steps) and their media
// attachments (YouTube links + PDF objects in MinIO).
//
// See docs/superpowers/specs/2026-08-08-guide-media-cms-design.md. Two properties
// shape this package and should not be "simplified" away:
//
//   - Guide content is GLOBAL. There is no office scoping anywhere here, and no
//     data_scope_policies row for it — deliberately, per the spec.
//   - Module text is public; ATTACHMENTS ARE NOT. This package never decides who
//     may read what: the handler serializes attachment refs only for callers with
//     a session, and the file endpoint sits behind RequireAuth.
//
// Split per ADR-0008: service (business rules, Gin-free) / dto / handler / routes,
// with attachment.go carrying the upload + storage half of the service.
package guide

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

// maxSteps bounds one module's numbered list. Well above the nine seeded modules
// (longest has four) and low enough that a runaway payload cannot bloat a row.
const maxSteps = 50

// MaxModules is the ceiling on one guide listing.
//
// Not paginated on purpose: the guide is read as a single document, and slicing
// it into pages would be worse for the reader than a bound that no real guide
// will reach — twenty-two times today's nine modules. Together with the ten
// attachments a module may carry, it caps the worst-case response at 200 modules
// and 2000 attachment rows.
const MaxModules = 200

var (
	ErrNotFound           = errors.New("guide: not found")
	ErrSlugExists         = errors.New("guide: a module with this slug already exists")
	ErrInvalidRef         = errors.New("guide: invalid reference")
	ErrInvalidSteps       = errors.New("guide: invalid steps")
	ErrInvalidVideoURL    = errors.New("guide: not a valid YouTube link")
	ErrUnsupportedType    = errors.New("guide: only PDF documents are accepted")
	ErrTooLarge           = errors.New("guide: file exceeds the size limit")
	ErrTooManyAttachments = errors.New("guide: attachment limit reached for this module")
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrSlugExists
		case "23503":
			return ErrInvalidRef
		case "23514":
			return ErrInvalidSteps
		}
	}
	return err
}

// Step is one entry of a module's numbered list. Indonesian is required;
// English is optional and falls back to Indonesian at render time — the
// fallback lives in the client so switching locale needs no refetch.
type Step struct {
	TextID string  `json:"text_id"`
	TextEn *string `json:"text_en,omitempty"`
}

// ModuleWithAttachments is what both the reader and the admin list return: a
// module plus every attachment belonging to it, already ordered.
type ModuleWithAttachments struct {
	Module      sqlc.GuideGuideModule
	Attachments []sqlc.GuideGuideAttachment
}

// ModuleInput carries the writable fields of a module. Create and update take
// the same shape: an update always replaces the whole module, including its
// step list, because that is how the editor works.
//
// Status is the one exception — it is read by Update only. Create ignores it and
// stores a draft; see the comment there.
type ModuleInput struct {
	Slug      string
	Icon      string
	SortOrder int32
	Status    sqlc.SharedGuideStatus
	TitleID   string
	TitleEn   *string
	BodyID    *string
	BodyEn    *string
	Steps     []Step
	Actor     uuid.UUID
}

// Service holds the guide module's business rules.
type Service struct {
	q        *sqlc.Queries
	pool     *pgxpool.Pool
	store    storage.Storage
	maxBytes int64
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, store storage.Storage, maxBytes int64) *Service {
	return &Service{q: q, pool: pool, store: store, maxBytes: maxBytes}
}

// MaxBytes is the PDF upload cap, used by the handler to bound the request body
// before parsing it and by the API to tell the UI what the limit is.
func (s *Service) MaxBytes() int64 { return s.maxBytes }

// validateSteps checks the step list and returns its canonical JSON encoding.
// Storing what we re-encoded (rather than what arrived) keeps unknown keys out
// of the column, so the jsonb shape stays exactly what the reader expects.
func validateSteps(steps []Step) ([]byte, error) {
	if len(steps) > maxSteps {
		return nil, ErrInvalidSteps
	}
	out := make([]Step, 0, len(steps))
	for _, st := range steps {
		if strings.TrimSpace(st.TextID) == "" {
			return nil, ErrInvalidSteps
		}
		st.TextID = strings.TrimSpace(st.TextID)
		if st.TextEn != nil {
			if trimmed := strings.TrimSpace(*st.TextEn); trimmed == "" {
				st.TextEn = nil
			} else {
				st.TextEn = &trimmed
			}
		}
		out = append(out, st)
	}
	return json.Marshal(out)
}

// List returns modules with their attachments. includeDraft is the caller's
// resolved authority, never a raw request parameter: the handler only sets it
// for callers holding guide.manage.
//
// The result is capped at MaxModules. This is the only data endpoint in the API
// that answers without a session, and its response cannot be cached, so the cost
// of an anonymous request must not grow with however many modules an author
// creates. Hitting the cap is logged rather than passed over in silence — a
// truncated list that nobody is told about reads exactly like a complete one.
func (s *Service) List(ctx context.Context, includeDraft bool) ([]ModuleWithAttachments, error) {
	mods, err := s.q.ListGuideModules(ctx, sqlc.ListGuideModulesParams{
		IncludeDraft: includeDraft,
		MaxRows:      MaxModules,
	})
	if err != nil {
		return nil, mapDBError(err)
	}
	if len(mods) >= MaxModules {
		slog.WarnContext(ctx, "guide: module list hit its ceiling and was truncated",
			"limit", MaxModules, "include_draft", includeDraft)
	}
	if len(mods) == 0 {
		return []ModuleWithAttachments{}, nil
	}

	ids := make([]uuid.UUID, 0, len(mods))
	for _, m := range mods {
		ids = append(ids, m.ID)
	}
	// One query for every module on the page, not one per module.
	atts, err := s.q.ListGuideAttachmentsByModules(ctx, ids)
	if err != nil {
		return nil, mapDBError(err)
	}
	byModule := make(map[uuid.UUID][]sqlc.GuideGuideAttachment, len(mods))
	for _, a := range atts {
		byModule[a.ModuleID] = append(byModule[a.ModuleID], a)
	}

	out := make([]ModuleWithAttachments, 0, len(mods))
	for _, m := range mods {
		out = append(out, ModuleWithAttachments{Module: m, Attachments: byModule[m.ID]})
	}
	return out, nil
}

// Get returns one module with its attachments, draft or not. Callers without
// guide.manage never reach this: it backs the admin editor only.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (ModuleWithAttachments, error) {
	m, err := s.q.GetGuideModule(ctx, id)
	if err != nil {
		return ModuleWithAttachments{}, mapDBError(err)
	}
	atts, err := s.q.ListGuideAttachmentsByModules(ctx, []uuid.UUID{id})
	if err != nil {
		return ModuleWithAttachments{}, mapDBError(err)
	}
	return ModuleWithAttachments{Module: m, Attachments: atts}, nil
}

// Create always produces a DRAFT, whatever in.Status holds.
//
// A module is published by editing it, never by creating it (spec business rule
// 5): the guide page is readable without a session, so half-written content must
// not be one request away from being public. The HTTP layer already withholds
// the field — moduleCreateRequest has no status — and this line is what keeps
// the rule true for any other caller of the service.
func (s *Service) Create(ctx context.Context, in ModuleInput) (sqlc.GuideGuideModule, error) {
	steps, err := validateSteps(in.Steps)
	if err != nil {
		return sqlc.GuideGuideModule{}, err
	}
	actor := in.Actor
	row, err := s.q.CreateGuideModule(ctx, sqlc.CreateGuideModuleParams{
		Slug:      in.Slug,
		Icon:      in.Icon,
		SortOrder: in.SortOrder,
		Status:    sqlc.SharedGuideStatusDraft,
		TitleID:   in.TitleID,
		TitleEn:   in.TitleEn,
		BodyID:    in.BodyID,
		BodyEn:    in.BodyEn,
		Steps:     steps,
		Actor:     &actor,
	})
	return row, mapDBError(err)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in ModuleInput) (sqlc.GuideGuideModule, error) {
	steps, err := validateSteps(in.Steps)
	if err != nil {
		return sqlc.GuideGuideModule{}, err
	}
	actor := in.Actor
	row, err := s.q.UpdateGuideModule(ctx, sqlc.UpdateGuideModuleParams{
		ID:        id,
		Slug:      in.Slug,
		Icon:      in.Icon,
		SortOrder: in.SortOrder,
		Status:    in.Status,
		TitleID:   in.TitleID,
		TitleEn:   in.TitleEn,
		BodyID:    in.BodyID,
		BodyEn:    in.BodyEn,
		Steps:     steps,
		Actor:     &actor,
	})
	return row, mapDBError(err)
}

// Delete soft-deletes the module and its attachments in one transaction.
//
// The stored objects are deliberately LEFT IN MinIO: with their rows gone the
// file endpoint answers 404, so they are unreachable, and keeping the bytes
// means a mistaken delete is still recoverable from the row's object_key.
// Deleting a single attachment does remove its object — that is the explicit
// "get rid of this file" action.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, actor uuid.UUID) (sqlc.GuideGuideModule, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.GuideGuideModule{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once Commit succeeded

	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteGuideModuleAttachments(ctx, id); err != nil {
		return sqlc.GuideGuideModule{}, mapDBError(err)
	}
	row, err := qtx.SoftDeleteGuideModule(ctx, sqlc.SoftDeleteGuideModuleParams{ID: id, Actor: &actor})
	if err != nil {
		return sqlc.GuideGuideModule{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.GuideGuideModule{}, err
	}
	return row, nil
}
