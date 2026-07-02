// Package transfer implements inter-office asset transfer (mutasi): submit via the
// generic approval engine, then ship/receive with BAST + asset relocation. Split
// into dto / service / handler / routes (ADR-0008). The service holds business
// rules + data-scope enforcement (Gin-free); the handler maps HTTP ↔ service.
package transfer

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Sentinel errors (mapped to HTTP status by the handler).
var (
	ErrNotFound       = errors.New("transfer: not found")
	ErrInvalidState   = errors.New("transfer: not in a state that allows this action")
	ErrAssetInTransit = errors.New("transfer: asset already has an open transfer")
	ErrOutOfScope     = errors.New("transfer: office out of scope")
	ErrSameOffice     = errors.New("transfer: destination office must differ from origin")
	ErrInvalidRef     = errors.New("transfer: invalid reference")
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrInvalidRef
	}
	return err
}

// Service holds data access + business rules for transfers.
type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
	appr *approval.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service) *Service {
	return &Service{q: q, pool: pool, appr: appr}
}

// Input structs.
type SubmitInput struct {
	AssetID    uuid.UUID
	ToOfficeID uuid.UUID
	ToRoomID   *uuid.UUID
	Reason     *string
}
type ShipInput struct{ ShippedDate pgtype.Date }
type ReceiveInput struct {
	BastNo       *string
	ReceivedDate pgtype.Date
	ToRoomID     *uuid.UUID
}

// Submit validates the asset + destination and opens an approval request. No transfer
// row is created here — the asset_transfer executor creates it on final approval.
func (s *Service) Submit(ctx context.Context, caller approval.Caller, in SubmitInput) (sqlc.ApprovalRequest, error) {
	asset, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	// Scope: caller must have the asset's home (from) office in scope.
	if !common.InScope(caller.AllScope, caller.OfficeIDs, asset.OfficeID) {
		return sqlc.ApprovalRequest{}, ErrOutOfScope
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return sqlc.ApprovalRequest{}, ErrInvalidState
	}
	if in.ToOfficeID == asset.OfficeID {
		return sqlc.ApprovalRequest{}, ErrSameOffice
	}
	// Guard: at most one open transfer row + one pending transfer request per asset.
	if _, err := s.q.GetOpenTransferForAsset(ctx, in.AssetID); err == nil {
		return sqlc.ApprovalRequest{}, ErrAssetInTransit
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ApprovalRequest{}, err
	}
	pending, err := s.q.CountPendingTransferRequestsForAsset(ctx, &in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	if pending > 0 {
		return sqlc.ApprovalRequest{}, ErrAssetInTransit
	}

	payload, err := marshalPayload(asset.OfficeID, in.ToOfficeID, in.ToRoomID, in.Reason)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	// Value basis: purchase_cost (book value needs depreciation — deferred).
	amount := "0"
	if asset.PurchaseCost != nil {
		amount = *asset.PurchaseCost
	}
	entity := "asset"
	targetID := in.AssetID
	return s.appr.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetTransfer,
		Amount:       amount,
		OfficeID:     asset.OfficeID,
		TargetEntity: &entity,
		TargetID:     &targetID,
		Payload:      payload,
		Reason:       in.Reason,
		Maker:        caller.UserID,
	})
}

// Ship marks an approved transfer as in_transit. Caller must have from_office in scope.
func (s *Service) Ship(ctx context.Context, all bool, ids []uuid.UUID, id uuid.UUID, in ShipInput) (sqlc.TransferAssetTransfer, error) {
	cur, err := s.q.GetTransfer(ctx, sqlc.GetTransferParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, mapDBError(err)
	}
	if !common.InScope(all, ids, cur.FromOfficeID) {
		return cur, ErrOutOfScope
	}
	if cur.Status != sqlc.SharedTransferStatusApproved {
		return cur, ErrInvalidState
	}
	shipped := in.ShippedDate
	if !shipped.Valid {
		shipped = pgtype.Date{Time: time.Now(), Valid: true}
	}
	out, err := s.q.SetTransferShipped(ctx, sqlc.SetTransferShippedParams{ID: id, ShippedDate: shipped})
	if err != nil {
		return cur, mapDBError(err)
	}
	return out, nil
}

// Receive marks an in_transit transfer as received and relocates the asset, atomically.
// Returns (before, after) for audit diffing. BAST document creation is done by the handler.
func (s *Service) Receive(ctx context.Context, all bool, ids []uuid.UUID, receiver, id uuid.UUID, in ReceiveInput) (before, after sqlc.TransferAssetTransfer, err error) {
	before, err = s.q.GetTransfer(ctx, sqlc.GetTransferParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, before, mapDBError(err)
	}
	if !common.InScope(all, ids, before.ToOfficeID) {
		return before, before, ErrOutOfScope
	}
	if before.Status != sqlc.SharedTransferStatusInTransit {
		return before, before, ErrInvalidState
	}
	recvDate := in.ReceivedDate
	if !recvDate.Valid {
		recvDate = pgtype.Date{Time: time.Now(), Valid: true}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return before, before, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	after, err = qtx.SetTransferReceived(ctx, sqlc.SetTransferReceivedParams{
		ID:           id,
		ReceivedDate: recvDate,
		ReceivedByID: &receiver,
		BastNo:       in.BastNo,
		ToRoomID:     in.ToRoomID,
	})
	if err != nil {
		return before, before, mapDBError(err)
	}
	// Relocate the asset to the destination office/room.
	if _, err = qtx.SetAssetOffice(ctx, sqlc.SetAssetOfficeParams{
		ID:       before.AssetID,
		OfficeID: before.ToOfficeID,
		RoomID:   after.ToRoomID,
	}); err != nil {
		return before, before, mapDBError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return before, before, err
	}
	return before, after, nil
}

// Get returns one scoped transfer.
func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.TransferAssetTransfer, error) {
	t, err := s.q.GetTransfer(ctx, sqlc.GetTransferParams{ID: id, AllScope: all, OfficeIds: ids})
	return t, mapDBError(err)
}

// List returns a scoped, paginated page + total. Empty status = no filter.
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, status string, limit, offset int32) ([]sqlc.TransferAssetTransfer, int64, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	var st *sqlc.SharedTransferStatus
	if status != "" {
		v := sqlc.SharedTransferStatus(status)
		st = &v
	}
	rows, err := s.q.ListTransfers(ctx, sqlc.ListTransfersParams{AllScope: all, OfficeIds: ids, Status: st, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountTransfers(ctx, sqlc.CountTransfersParams{AllScope: all, OfficeIds: ids, Status: st})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// ListByAsset returns a scoped transfer history for one asset.
func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, all bool, ids []uuid.UUID) ([]sqlc.TransferAssetTransfer, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListTransfersByAsset(ctx, sqlc.ListTransfersByAssetParams{AssetID: assetID, AllScope: all, OfficeIds: ids})
	return rows, mapDBError(err)
}
