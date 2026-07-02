package transfer

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// transferExec creates the transfer row on final approval, inside the commit tx.
type transferExec struct{ s *Service }

func (e transferExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return approval.ErrInvalidRef
	}
	var p TransferPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return approval.ErrInvalidRef
	}
	fromOffice, err := uuid.Parse(p.FromOfficeID)
	if err != nil {
		return approval.ErrInvalidRef
	}
	toOffice, err := uuid.Parse(p.ToOfficeID)
	if err != nil {
		return approval.ErrInvalidRef
	}

	// Defense-in-depth: the payload's from-office must match the request office (set at
	// submit from the asset's home office), and the asset must still live there.
	if req.OfficeID == nil || fromOffice != *req.OfficeID {
		return approval.ErrInvalidRef
	}
	asset, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		return approval.ErrInvalidRef
	}
	if asset.OfficeID != fromOffice || toOffice == fromOffice {
		return approval.ErrInvalidRef
	}
	// Guard: refuse a second open transfer for the same asset.
	if _, err := qtx.GetOpenTransferForAsset(ctx, *req.TargetID); err == nil {
		return approval.ErrConflict
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var toRoom *uuid.UUID
	if p.ToRoomID != nil {
		r, perr := uuid.Parse(*p.ToRoomID)
		if perr != nil {
			return approval.ErrInvalidRef
		}
		toRoom = &r
	}
	reqID := req.ID
	approver := req.DecidedByID
	_, err = qtx.CreateTransfer(ctx, sqlc.CreateTransferParams{
		AssetID:       *req.TargetID,
		FromOfficeID:  fromOffice,
		ToOfficeID:    toOffice,
		ToRoomID:      toRoom,
		Reason:        p.Reason,
		RequestedByID: req.RequestedByID,
		ApprovedByID:  approver,
		RequestID:     &reqID,
	})
	return err
}

// Executor returns the asset_transfer approval executor.
func (s *Service) Executor() approval.Executor { return transferExec{s} }
