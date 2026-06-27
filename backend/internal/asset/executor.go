package asset

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/masterdata/common"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// AssetCreatePayload is the JSON stored in approval_requests.payload for the
// asset_create request type.
type AssetCreatePayload struct {
	Name         string  `json:"name"`
	CategoryID   string  `json:"category_id"`
	OfficeID     string  `json:"office_id"`
	RoomID       *string `json:"room_id"`
	AssetClass   string  `json:"asset_class"`
	PurchaseCost *string `json:"purchase_cost"`
	PurchaseDate *string `json:"purchase_date"` // RFC3339 date "2006-01-02"
	SerialNumber *string `json:"serial_number"`
}

// parsePurchaseDate parses an optional "2006-01-02" date string into a pgtype.Date.
// Returns a zero pgtype.Date (Valid=false) when s is nil.
func parsePurchaseDate(s *string) (pgtype.Date, error) {
	if s == nil {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// --- createExec ----------------------------------------------------------

type createExec struct{ s *Service }

func (e createExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	var p AssetCreatePayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return err
	}

	officeID, err := uuid.Parse(p.OfficeID)
	if err != nil {
		return ErrInvalidRef
	}
	categoryID, err := uuid.Parse(p.CategoryID)
	if err != nil {
		return ErrInvalidRef
	}

	// Derive asset-tag year from purchase date when present, else current year.
	year := int32(time.Now().Year())
	purchaseDate, err := parsePurchaseDate(p.PurchaseDate)
	if err == nil && purchaseDate.Valid {
		year = int32(purchaseDate.Time.Year())
	}

	tag, err := e.s.GenerateAssetTag(ctx, qtx, officeID, categoryID, year)
	if err != nil {
		return mapDBError(err)
	}

	// Parse optional room UUID.
	roomID, err := common.ParseUUIDPtr(p.RoomID)
	if err != nil {
		return ErrInvalidRef
	}

	requesterID := req.RequestedByID
	_, err = qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
		AssetTag:     tag,
		Name:         p.Name,
		CategoryID:   categoryID,
		OfficeID:     officeID,
		RoomID:       roomID,
		AssetClass:   sqlc.SharedAssetClass(p.AssetClass),
		Capitalized:  true,
		CreatedByID:  &requesterID,
		SerialNumber: p.SerialNumber,
		PurchaseCost: p.PurchaseCost,
		PurchaseDate: purchaseDate,
		// Unset optional fields — leave as zero values (nil / false / empty).
		Specifications: []byte("{}"),
	})
	return mapDBError(err)
}

// --- disposalExec --------------------------------------------------------

type disposalExec struct{ s *Service }

func (e disposalExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return ErrInvalidRef
	}
	cur, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		return mapDBError(err)
	}
	if !validTransition(cur.Status, sqlc.SharedAssetStatusDisposed) {
		return ErrInvalidState
	}
	_, err = qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{
		ID:     *req.TargetID,
		Status: sqlc.SharedAssetStatusDisposed,
	})
	return mapDBError(err)
}

// --- exclusionExec -------------------------------------------------------

type exclusionExec struct{ s *Service }

func (e exclusionExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return ErrInvalidRef
	}
	_, err := qtx.SetAssetValuationExclusion(ctx, sqlc.SetAssetValuationExclusionParams{
		ID:                       *req.TargetID,
		ExcludedFromValuation:    true,
		ValuationExclusionReason: req.Reason,
	})
	return mapDBError(err)
}

// --- Service accessor methods --------------------------------------------

// CreateExecutor returns an Executor that creates a new asset inside the
// approval commit transaction.
func (s *Service) CreateExecutor() approval.Executor { return createExec{s} }

// DisposalExecutor returns an Executor that marks an asset as disposed inside
// the approval commit transaction.
func (s *Service) DisposalExecutor() approval.Executor { return disposalExec{s} }

// ExclusionExecutor returns an Executor that excludes an asset from valuation
// inside the approval commit transaction.
func (s *Service) ExclusionExecutor() approval.Executor { return exclusionExec{s} }
