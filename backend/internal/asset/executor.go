package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/importer"
	"github.com/ragbuaj/inventra/internal/masterdata/common"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// AssetCreatePayload is the JSON stored in approval_requests.payload for the
// asset_create request type.
type AssetCreatePayload struct {
	Name           string  `json:"name"`
	CategoryID     string  `json:"category_id"`
	OfficeID       string  `json:"office_id"`
	RoomID         *string `json:"room_id"`
	AssetClass     string  `json:"asset_class"`
	PurchaseCost   *string `json:"purchase_cost"`
	PurchaseDate   *string `json:"purchase_date"` // "2006-01-02"
	SerialNumber   *string `json:"serial_number"`
	BrandID        *string `json:"brand_id"`
	ModelID        *string `json:"model_id"`
	UnitID         *string `json:"unit_id"`
	VendorID       *string `json:"vendor_id"`
	PONumber       *string `json:"po_number"`
	FundingSource  *string `json:"funding_source"`
	WarrantyExpiry *string `json:"warranty_expiry"` // "2006-01-02"
	Notes          *string `json:"notes"`
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

	// Defense-in-depth: the office embedded in the payload must match the
	// office recorded on the approval request row. A mismatch means the
	// payload was tampered with after the scope check in the submit handler.
	if req.OfficeID == nil || officeID != *req.OfficeID {
		return ErrInvalidRef
	}

	categoryID, err := uuid.Parse(p.CategoryID)
	if err != nil {
		return ErrInvalidRef
	}

	// Derive asset-tag year from purchase date when present, else current year.
	year := int32(time.Now().Year())
	purchaseDate, derr := parsePurchaseDate(p.PurchaseDate)
	if derr != nil {
		return fmt.Errorf("invalid purchase_date: %w", derr)
	}
	if purchaseDate.Valid {
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

	brandID, err := common.ParseUUIDPtr(p.BrandID)
	if err != nil {
		return ErrInvalidRef
	}
	modelID, err := common.ParseUUIDPtr(p.ModelID)
	if err != nil {
		return ErrInvalidRef
	}
	unitID, err := common.ParseUUIDPtr(p.UnitID)
	if err != nil {
		return ErrInvalidRef
	}
	vendorID, err := common.ParseUUIDPtr(p.VendorID)
	if err != nil {
		return ErrInvalidRef
	}
	warrantyExpiry, derr := parsePurchaseDate(p.WarrantyExpiry)
	if derr != nil {
		return fmt.Errorf("invalid warranty_expiry: %w", derr)
	}

	requesterID := req.RequestedByID
	_, err = qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
		AssetTag:       tag,
		Name:           p.Name,
		CategoryID:     categoryID,
		OfficeID:       officeID,
		RoomID:         roomID,
		AssetClass:     sqlc.SharedAssetClass(p.AssetClass),
		Capitalized:    true,
		CreatedByID:    &requesterID,
		SerialNumber:   p.SerialNumber,
		PurchaseCost:   p.PurchaseCost,
		PurchaseDate:   purchaseDate,
		BrandID:        brandID,
		ModelID:        modelID,
		UnitID:         unitID,
		VendorID:       vendorID,
		PoNumber:       p.PONumber,
		FundingSource:  p.FundingSource,
		WarrantyExpiry: warrantyExpiry,
		Notes:          p.Notes,
		// Unset optional fields — leave as zero values (nil / false / empty).
		Specifications: []byte("{}"),
	})
	return mapDBError(err)
}

// --- exclusionExec -------------------------------------------------------

type exclusionExec struct{ s *Service }

func (e exclusionExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return ErrInvalidRef
	}

	// Defense-in-depth: load the asset to verify its office matches the request
	// office before applying the valuation exclusion.
	cur, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		return mapDBError(err)
	}
	if req.OfficeID == nil || cur.OfficeID != *req.OfficeID {
		return ErrInvalidRef
	}

	_, err = qtx.SetAssetValuationExclusion(ctx, sqlc.SetAssetValuationExclusionParams{
		ID:                       *req.TargetID,
		ExcludedFromValuation:    true,
		ValuationExclusionReason: req.Reason,
	})
	return mapDBError(err)
}

// --- assetImportExec -----------------------------------------------------

// AssetImportPayload is the JSON stored in approval_requests.payload for the
// asset_import request type. The worker builds it from the validated import job
// (see importer/worker.go buildAssetPayload); this executor consumes it when
// the batch's maker-checker approval is granted.
type AssetImportPayload struct {
	JobID      string `json:"job_id"`
	Filename   string `json:"filename"`
	TotalRows  int    `json:"total_rows"`
	TotalValue string `json:"total_value"`
	OfficeID   string `json:"office_id"`
}

type assetImportExec struct{ s *Service }

// Execute creates every valid row of an approved import batch inside the
// approval-commit transaction. It is idempotent against stale/duplicate
// approvals: the import job must still be in awaiting_approval, so a job already
// executed (completed) by a prior approval is a no-op rather than a
// double-create. This pairs with the worker's crash-window guard that ensures a
// single approval request per batch.
func (e assetImportExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	var p AssetImportPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return err
	}

	officeID, err := uuid.Parse(p.OfficeID)
	if err != nil {
		return ErrInvalidRef
	}
	// Defense-in-depth: the payload office must match the office on the
	// approval request row (mirrors createExec).
	if req.OfficeID == nil || officeID != *req.OfficeID {
		return ErrInvalidRef
	}

	jobID, err := uuid.Parse(p.JobID)
	if err != nil {
		return ErrInvalidRef
	}
	job, err := qtx.GetImportJob(ctx, jobID)
	if err != nil {
		return mapDBError(err)
	}

	// Critical guard: only an import job still awaiting approval may be
	// executed. A stale duplicate approval landing on an already-completed job
	// must not re-create its assets — treat it as a no-op.
	if job.Status != sqlc.SharedImportStatusAwaitingApproval {
		return nil
	}

	rows, err := qtx.ListValidImportRows(ctx, jobID)
	if err != nil {
		return err
	}
	domainRows := make([]importer.Row, 0, len(rows))
	for _, r := range rows {
		var data map[string]string
		if len(r.Data) > 0 {
			if uErr := json.Unmarshal(r.Data, &data); uErr != nil {
				return uErr
			}
		}
		domainRows = append(domainRows, importer.Row{ID: r.ID, RowNo: int(r.RowNo), Data: data})
	}

	maker := job.CreatedByID
	created, err := (assetImporter{e.s}).createRows(ctx, qtx, &maker, domainRows)
	if err != nil {
		return err
	}

	_, err = qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
		ID:          jobID,
		Status:      sqlc.SharedImportStatusCompleted,
		SuccessRows: int32(created),
		FailedRows:  int32(len(domainRows) - created),
		ErrorKey:    nil,
	})
	return mapDBError(err)
}

// --- Service accessor methods --------------------------------------------

// CreateExecutor returns an Executor that creates a new asset inside the
// approval commit transaction.
func (s *Service) CreateExecutor() approval.Executor { return createExec{s} }

// ExclusionExecutor returns an Executor that excludes an asset from valuation
// inside the approval commit transaction.
func (s *Service) ExclusionExecutor() approval.Executor { return exclusionExec{s} }

// ImportExecutor returns an Executor that creates a validated import batch's
// assets inside the approval commit transaction. Wiring
// (RegisterExecutor(SharedRequestTypeAssetImport, ...)) is done by the router.
func (s *Service) ImportExecutor() approval.Executor { return assetImportExec{s} }
