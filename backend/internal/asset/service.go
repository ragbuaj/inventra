package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
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
