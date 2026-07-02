package disposal

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
)

// disposalExec writes the disposal row + flips asset status on final approval, in-tx.
type disposalExec struct{ s *Service }

func (e disposalExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return approval.ErrInvalidRef
	}
	var p DisposalPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return approval.ErrInvalidRef
	}
	cur, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
	}
	// Defense-in-depth: asset office must match the request office.
	if req.OfficeID == nil || cur.OfficeID != *req.OfficeID {
		return approval.ErrInvalidRef
	}
	if !asset.ValidTransition(cur.Status, sqlc.SharedAssetStatusDisposed) {
		return approval.ErrInvalidRef
	}
	// Guard: at most one live disposal per asset.
	if _, err := qtx.GetDisposalByAsset(ctx, *req.TargetID); err == nil {
		return approval.ErrConflict
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	date, derr := parseDate(p.DisposalDate)
	if derr != nil {
		return approval.ErrInvalidRef
	}
	reqID := req.ID
	approver := req.DecidedByID
	requester := req.RequestedByID
	if _, err := qtx.CreateDisposal(ctx, sqlc.CreateDisposalParams{
		AssetID:             *req.TargetID,
		Method:              sqlc.SharedDisposalMethod(p.Method),
		DisposalDate:        date,
		Proceeds:            p.Proceeds,
		BookValueAtDisposal: p.BookValue,
		BastNo:              p.BastNo,
		ApprovedByID:        approver,
		RequestID:           &reqID,
		CreatedByID:         &requester,
	}); err != nil {
		return err
	}
	_, err = qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: *req.TargetID, Status: sqlc.SharedAssetStatusDisposed})
	return err
}

// Executor returns the asset_disposal approval executor.
func (s *Service) Executor() approval.Executor { return disposalExec{s} }
