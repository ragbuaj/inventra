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

// MaxBatchQuantity caps how many identical units one asset_create request may
// create (spec 2026-07-23 section 9). The approval submit-time cross-check in
// internal/approval enforces the same ceiling with its own constant — the two
// MUST stay in sync (approval cannot import asset without an import cycle).
const MaxBatchQuantity = 500

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
	// Legacy-parity fields (spec 2026-07-23).
	FloorID          *string `json:"floor_id"`
	PICEmployeeID    *string `json:"pic_employee_id"`
	Capacity         *string `json:"capacity"`
	LeaseDate        *string `json:"lease_date"`        // "2006-01-02"
	InstallationDate *string `json:"installation_date"` // "2006-01-02"
	WarrantyStart    *string `json:"warranty_start"`    // "2006-01-02"
	// Batch registration: number of identical asset units to create in one
	// request (spec 2026-07-23 section 9). Absent/0 means a single unit; each
	// unit takes its own sequential tag. Approval amount = purchase_cost * quantity.
	Quantity int `json:"quantity"`
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

	// Legacy-parity fields.
	floorID, err := common.ParseUUIDPtr(p.FloorID)
	if err != nil {
		return ErrInvalidRef
	}
	picID, err := common.ParseUUIDPtr(p.PICEmployeeID)
	if err != nil {
		return ErrInvalidRef
	}
	leaseDate, derr := parsePurchaseDate(p.LeaseDate)
	if derr != nil {
		return fmt.Errorf("invalid lease_date: %w", derr)
	}
	installationDate, derr := parsePurchaseDate(p.InstallationDate)
	if derr != nil {
		return fmt.Errorf("invalid installation_date: %w", derr)
	}
	warrantyStart, derr := parsePurchaseDate(p.WarrantyStart)
	if derr != nil {
		return fmt.Errorf("invalid warranty_start: %w", derr)
	}

	// Validate + normalize floor/room against the asset's office (forces floor to
	// the room's own floor when a room is chosen).
	floorID, err = e.s.resolveLocation(ctx, qtx, officeID, floorID, roomID)
	if err != nil {
		return err
	}

	requesterID := req.RequestedByID

	// Batch registration: create `quantity` identical units in one approval
	// commit (spec 2026-07-23 section 9). Absent/0 quantity means a single unit.
	// Each unit draws its own sequential tag from the per-office advisory-locked
	// counter, so a batch of N yields N consecutive tag_seq values.
	quantity := p.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	// Defense-in-depth: the submit-time cross-check (approval.validateAssetCreateAmount)
	// already caps quantity, but the executor re-reads the raw payload, so bound it
	// here too — a huge N would hold the per-office tag advisory lock for the whole
	// commit and starve every other asset create in that office.
	if quantity > MaxBatchQuantity {
		return ErrInvalidRef
	}

	// A serial number identifies one physical unit, so it can never be shared
	// across a batch. The form clears it, but a hand-crafted payload could still
	// carry one — drop it server-side for quantity > 1.
	serialNumber := p.SerialNumber
	if quantity > 1 {
		serialNumber = nil
	}

	for i := 0; i < quantity; i++ {
		tag, tagSeq, terr := e.s.GenerateAssetTag(ctx, qtx, officeID, categoryID, year)
		if terr != nil {
			return mapDBError(terr)
		}

		created, cerr := qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
			AssetTag:         tag,
			TagSeq:           &tagSeq,
			Name:             p.Name,
			CategoryID:       categoryID,
			OfficeID:         officeID,
			RoomID:           roomID,
			FloorID:          floorID,
			AssetClass:       sqlc.SharedAssetClass(p.AssetClass),
			Capitalized:      true,
			CreatedByID:      &requesterID,
			SerialNumber:     serialNumber,
			PurchaseCost:     p.PurchaseCost,
			PurchaseDate:     purchaseDate,
			BrandID:          brandID,
			ModelID:          modelID,
			UnitID:           unitID,
			VendorID:         vendorID,
			PoNumber:         p.PONumber,
			FundingSource:    p.FundingSource,
			WarrantyExpiry:   warrantyExpiry,
			WarrantyStart:    warrantyStart,
			Capacity:         p.Capacity,
			LeaseDate:        leaseDate,
			InstallationDate: installationDate,
			PicEmployeeID:    picID,
			Notes:            p.Notes,
			// Unset optional fields — leave as zero values (nil / false / empty).
			Specifications: []byte("{}"),
		})
		if cerr != nil {
			return mapDBError(cerr)
		}

		// Record initial location + PIC history (Fase 3 legacy-parity).
		if herr := qtx.InsertAssetLocationHistory(ctx, sqlc.InsertAssetLocationHistoryParams{
			AssetID:   created.ID,
			OfficeID:  officeID,
			FloorID:   floorID,
			RoomID:    roomID,
			Source:    sqlc.SharedLocationChangeSourceRegistration,
			MovedByID: &requesterID,
		}); herr != nil {
			return mapDBError(herr)
		}
		if picID != nil {
			if herr := qtx.InsertAssetPICHistory(ctx, sqlc.InsertAssetPICHistoryParams{
				AssetID:       created.ID,
				PicEmployeeID: *picID,
				AssignedByID:  &requesterID,
			}); herr != nil {
				return mapDBError(herr)
			}
		}
	}
	return nil
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
	//
	// The "create batch exactly once" invariant is enforced across THREE
	// cooperating pieces — do not weaken any one without re-checking the others:
	//   1. importer.Worker.executePhase's FindActiveImportRequest de-dup (avoids
	//      a second asset_import request after a Submit-commit/crash),
	//   2. approval.Service.Decide's GetRequestForUpdate row-lock (serializes
	//      concurrent approvers on the same request),
	//   3. this awaiting_approval status guard (a stale/duplicate approval that
	//      wins the race finds the job already completed and no-ops).
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

	// domainRows only carries rows that PASSED validation (ListValidImportRows
	// above) — `len(domainRows) - created` is only the count that failed
	// during this Execute (e.g. a DB constraint), not the rows that already
	// failed validation. job.FailedRows (set by the validate phase — see
	// importer/worker.go's validatePhase/SetJobValidated) must be preserved
	// and added to, not overwritten, or a batch's original validation
	// failures silently vanish from the completed job's failed_rows (and from
	// the UI's "N gagal" tile) once execution succeeds for all valid rows.
	execFailed := len(domainRows) - created
	_, err = qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
		ID:          jobID,
		Status:      sqlc.SharedImportStatusCompleted,
		SuccessRows: int32(created),
		FailedRows:  job.FailedRows + int32(execFailed),
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
