package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

var (
	ErrNotFound     = errors.New("asset not found")
	ErrInvalidState = errors.New("invalid status transition")
	ErrConflict     = errors.New("conflict")
	ErrInvalidRef   = errors.New("invalid reference")
	ErrRoomRequired = errors.New("tangible asset requires a floor or room")
)

// Service holds the data-access layer for the asset module.
type Service struct {
	q        *sqlc.Queries
	pool     *pgxpool.Pool
	store    storage.Storage
	maxBytes int64
	logoPath string
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, store storage.Storage, maxBytes int64, logoPath string) *Service {
	return &Service{q: q, pool: pool, store: store, maxBytes: maxBytes, logoPath: logoPath}
}

// MaxBytes returns the configured upload size cap, shared with other modules
// (e.g. maintenance's damage-report photo) that need the same request-body limit.
func (s *Service) MaxBytes() int64 { return s.maxBytes }

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

// ValidTransition reports whether an asset may move from status `from` to `to`.
// Exported for the disposal module's executor, which shares the transition matrix.
func ValidTransition(from, to sqlc.SharedAssetStatus) bool { return validTransition(from, to) }

// formatAssetTag formats an asset tag as <officeCode><categoryCode><year><seq:%05d>
// (no separators). Example: JKT01ELK202600001. The 5-digit sequence is per-office.
func formatAssetTag(officeCode, categoryCode string, year int, seq int64) string {
	return fmt.Sprintf("%s%s%d%05d", officeCode, categoryCode, year, seq)
}

// NextTagSeq allocates the next per-office tag sequence as MAX(tag_seq)+1, under a
// per-office advisory lock held until the caller's transaction commits/rolls back,
// so two concurrent creations in the same office never pick the same number.
// Soft-deleted rows keep their number (counted in MAX); a hard-deleted top row
// frees its number for reuse. The caller must pass a tx-bound *sqlc.Queries.
func (s *Service) NextTagSeq(ctx context.Context, qtx *sqlc.Queries, officeID uuid.UUID) (int32, error) {
	if err := qtx.AcquireOfficeTagLock(ctx, officeID.String()); err != nil {
		return 0, err
	}
	maxSeq, err := qtx.GetMaxTagSeqForOffice(ctx, officeID)
	if err != nil {
		return 0, err
	}
	return maxSeq + 1, nil
}

// GenerateAssetTag allocates the next per-office sequence and formats the auto
// asset tag. It returns the sequence too so the caller stores it in
// assets.tag_seq. The category must have a code (used in the tag string).
func (s *Service) GenerateAssetTag(ctx context.Context, qtx *sqlc.Queries, officeID, categoryID uuid.UUID, year int32) (string, int32, error) {
	officeCode, err := qtx.GetOfficeCode(ctx, officeID)
	if err != nil {
		return "", 0, ErrInvalidRef
	}
	categoryCode, err := qtx.GetCategoryCode(ctx, categoryID)
	if err != nil || categoryCode == nil {
		return "", 0, ErrInvalidRef
	}
	seq, err := s.NextTagSeq(ctx, qtx, officeID)
	if err != nil {
		return "", 0, err
	}
	return formatAssetTag(officeCode, *categoryCode, int(year), int64(seq)), seq, nil
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
		case "22P02":
			return ErrInvalidRef
		}
	}
	return err
}

// ListInput holds the parameters for a scoped asset list query.
type ListInput struct {
	Search        *string
	CategoryID    *uuid.UUID
	OfficeFilter  *uuid.UUID
	Status        *sqlc.SharedAssetStatus
	AssetClass    *sqlc.SharedAssetClass
	Limit, Offset int32
	AllScope      bool
	OfficeIDs     []uuid.UUID
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

// UpdateInput holds the updatable attributes for a direct asset update.
// Excluded: purchase_cost, asset_class, status (handled by dedicated operations).
type UpdateInput struct {
	Name           string
	CategoryID     uuid.UUID
	BrandID        *uuid.UUID
	ModelID        *uuid.UUID
	RoomID         *uuid.UUID
	FloorID        *uuid.UUID
	UnitID         *uuid.UUID
	SerialNumber   *string
	PurchaseDate   pgtype.Date
	VendorID       *uuid.UUID
	PONumber       *string
	FundingSource  *string
	WarrantyExpiry pgtype.Date
	// Legacy-parity fields (spec 2026-07-23).
	WarrantyStart    pgtype.Date
	Capacity         *string
	LeaseDate        pgtype.Date
	InstallationDate pgtype.Date
	PICEmployeeID    *uuid.UUID
	Specifications   []byte
	Notes            *string
}

// Update fetches the current asset row (for audit before/after diff), applies
// the given field changes inside a transaction, and — when the location
// (floor/room) or PIC changed — records the matching history rows. `actor` is
// recorded as who moved/assigned. Returns ErrNotFound if the asset does not
// exist or is soft-deleted.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput, actor uuid.UUID) (before, after sqlc.AssetAsset, err error) {
	before, err = s.q.GetAsset(ctx, id)
	if err != nil {
		return before, before, mapDBError(err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return before, before, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	after, err = qtx.UpdateAsset(ctx, sqlc.UpdateAssetParams{
		ID:               id,
		Name:             in.Name,
		CategoryID:       in.CategoryID,
		BrandID:          in.BrandID,
		ModelID:          in.ModelID,
		RoomID:           in.RoomID,
		FloorID:          in.FloorID,
		UnitID:           in.UnitID,
		SerialNumber:     in.SerialNumber,
		PurchaseDate:     in.PurchaseDate,
		VendorID:         in.VendorID,
		PoNumber:         in.PONumber,
		FundingSource:    in.FundingSource,
		WarrantyExpiry:   in.WarrantyExpiry,
		WarrantyStart:    in.WarrantyStart,
		Capacity:         in.Capacity,
		LeaseDate:        in.LeaseDate,
		InstallationDate: in.InstallationDate,
		PicEmployeeID:    in.PICEmployeeID,
		Specifications:   in.Specifications,
		Notes:            in.Notes,
	})
	if err != nil {
		return before, before, mapDBError(err)
	}

	// Record a location-history row when floor/room changed (office is not
	// editable via this path, so office_id is stable).
	if !sameUUIDPtr(before.FloorID, after.FloorID) || !sameUUIDPtr(before.RoomID, after.RoomID) {
		if err = qtx.InsertAssetLocationHistory(ctx, sqlc.InsertAssetLocationHistoryParams{
			AssetID:   id,
			OfficeID:  after.OfficeID,
			FloorID:   after.FloorID,
			RoomID:    after.RoomID,
			Source:    sqlc.SharedLocationChangeSourceEdit,
			MovedByID: &actor,
		}); err != nil {
			return before, before, mapDBError(err)
		}
	}

	// Record PIC history when the PIC changed: close the active row, open a new
	// one when a PIC is set (a cleared PIC just closes the active row).
	if !sameUUIDPtr(before.PicEmployeeID, after.PicEmployeeID) {
		if err = qtx.CloseActivePIC(ctx, id); err != nil {
			return before, before, mapDBError(err)
		}
		if after.PicEmployeeID != nil {
			if err = qtx.InsertAssetPICHistory(ctx, sqlc.InsertAssetPICHistoryParams{
				AssetID:       id,
				PicEmployeeID: *after.PicEmployeeID,
				AssignedByID:  &actor,
			}); err != nil {
				return before, before, mapDBError(err)
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return before, before, err
	}
	return before, after, nil
}

// sameUUIDPtr reports whether two optional UUIDs are equal (both nil, or same value).
func sameUUIDPtr(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// ListLocationHistory returns an asset's location-change history, newest first.
func (s *Service) ListLocationHistory(ctx context.Context, assetID uuid.UUID) ([]sqlc.ListAssetLocationHistoryRow, error) {
	rows, err := s.q.ListAssetLocationHistory(ctx, assetID)
	return rows, mapDBError(err)
}

// ListPICHistory returns an asset's PIC (person-in-charge) history, newest first.
func (s *Service) ListPICHistory(ctx context.Context, assetID uuid.UUID) ([]sqlc.ListAssetPICHistoryRow, error) {
	rows, err := s.q.ListAssetPICHistory(ctx, assetID)
	return rows, mapDBError(err)
}
