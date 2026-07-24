package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
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
	params := sqlc.CreateTransferParams{
		AssetID:       *req.TargetID,
		FromOfficeID:  fromOffice,
		ToOfficeID:    toOffice,
		ToRoomID:      toRoom,
		Reason:        p.Reason,
		RequestedByID: req.RequestedByID,
		ApprovedByID:  approver,
		RequestID:     &reqID,
	}
	if p.ConditionSent != nil {
		cond := sqlc.SharedTransferCondition(*p.ConditionSent)
		params.ConditionSent = &cond
	}
	if p.TransferDate != nil {
		td, perr := time.Parse("2006-01-02", *p.TransferDate)
		if perr != nil {
			return approval.ErrInvalidRef
		}
		params.TransferDate = pgtype.Date{Time: td, Valid: true}
	}
	created, err := qtx.CreateTransfer(ctx, params)
	if err != nil {
		return err
	}
	// Notify the origin office the transfer is approved and ready to ship. Enqueued
	// in the same commit tx as the transfer row, so it shares its fate. asset (loaded
	// above) supplies the tag/name the notification renders.
	return e.s.enqueueTransferEvent(ctx, qtx, EventTransferApproved, TransferEvent{
		TransferID:   created.ID,
		AssetID:      created.AssetID,
		AssetTag:     asset.AssetTag,
		AssetName:    asset.Name,
		FromOfficeID: created.FromOfficeID,
		ToOfficeID:   created.ToOfficeID,
	})
}

// Executor returns the asset_transfer approval executor.
func (s *Service) Executor() approval.Executor { return transferExec{s} }
