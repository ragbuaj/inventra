package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

var (
	ErrNotFound     = errors.New("asset not found")
	ErrInvalidState = errors.New("invalid status transition")
	ErrConflict     = errors.New("conflict")
	ErrInvalidRef   = errors.New("invalid reference")
	ErrRoomRequired = errors.New("tangible asset requires a room")
)

// Service holds the data-access layer for the asset module.
type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool) *Service { return &Service{q: q, pool: pool} }

// allowedTransitions defines the valid status transitions for assets.
// Only transitions present in this map are permitted; everything else is rejected.
var allowedTransitions = map[sqlc.SharedAssetStatus]map[sqlc.SharedAssetStatus]bool{
	"available":         {"assigned": true, "under_maintenance": true, "lost": true, "disposed": true},
	"assigned":          {"available": true, "lost": true, "disposed": true},
	"under_maintenance": {"available": true, "disposed": true},
}

// validTransition reports whether transitioning an asset from status `from` to
// status `to` is permitted by the state machine.
func validTransition(from, to sqlc.SharedAssetStatus) bool {
	return allowedTransitions[from][to]
}

// formatAssetTag formats an asset tag as <officeCode>-<categoryCode>-<year>-<seq:%05d>.
// Example: JKT01-ELK-2026-00001
func formatAssetTag(officeCode, categoryCode string, year int, seq int64) string {
	return fmt.Sprintf("%s-%s-%d-%05d", officeCode, categoryCode, year, seq)
}

// GenerateAssetTag bumps the per-(office, category, year) counter inside the
// caller's transaction and returns the formatted asset tag. The caller must
// pass a tx-bound *sqlc.Queries so the counter bump is atomic with the INSERT.
func (s *Service) GenerateAssetTag(ctx context.Context, qtx *sqlc.Queries, officeID, categoryID uuid.UUID, year int32) (string, error) {
	officeCode, err := qtx.GetOfficeCode(ctx, officeID)
	if err != nil {
		return "", ErrInvalidRef
	}
	categoryCode, err := qtx.GetCategoryCode(ctx, categoryID)
	if err != nil || categoryCode == nil {
		return "", ErrInvalidRef
	}
	seq, err := qtx.BumpAssetTagCounter(ctx, sqlc.BumpAssetTagCounterParams{
		OfficeID: officeID, CategoryID: categoryID, Year: year,
	})
	if err != nil {
		return "", err
	}
	return formatAssetTag(officeCode, *categoryCode, int(year), int64(seq)), nil
}

// mapDBError translates pgx/Postgres errors into package sentinel errors.
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
			return ErrConflict
		case "23503":
			return ErrInvalidRef
		case "23514":
			return ErrRoomRequired
		}
	}
	return err
}

// ListInput holds the parameters for a scoped asset list query.
type ListInput struct {
	Search       *string
	CategoryID   *uuid.UUID
	OfficeFilter *uuid.UUID
	Status       *sqlc.SharedAssetStatus
	AssetClass   *sqlc.SharedAssetClass
	Limit, Offset int32
	AllScope     bool
	OfficeIDs    []uuid.UUID
}

// List returns a page of assets matching the given filters, scoped to the
// caller's office set, along with the total unfiltered count.
func (s *Service) List(ctx context.Context, in ListInput) ([]sqlc.AssetAsset, int64, error) {
	rows, err := s.q.ListAssets(ctx, sqlc.ListAssetsParams{
		AllScope:     in.AllScope,
		OfficeIds:    in.OfficeIDs,
		Search:       in.Search,
		CategoryID:   in.CategoryID,
		OfficeFilter: in.OfficeFilter,
		Status:       in.Status,
		AssetClass:   in.AssetClass,
		Off:          in.Offset,
		Lim:          in.Limit,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountAssets(ctx, sqlc.CountAssetsParams{
		AllScope:     in.AllScope,
		OfficeIds:    in.OfficeIDs,
		Search:       in.Search,
		CategoryID:   in.CategoryID,
		OfficeFilter: in.OfficeFilter,
		Status:       in.Status,
		AssetClass:   in.AssetClass,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// Get returns a single asset by ID, or ErrNotFound if it does not exist or is
// soft-deleted.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (sqlc.AssetAsset, error) {
	a, err := s.q.GetAsset(ctx, id)
	return a, mapDBError(err)
}
