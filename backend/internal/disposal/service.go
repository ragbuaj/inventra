// Package disposal implements asset disposal: submit via the generic approval engine,
// then record the disposal + flip asset status on approval, with a BAST document.
// Split into dto / service / handler / routes (ADR-0008), mirroring internal/transfer.
package disposal

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

var (
	ErrNotFound        = errors.New("disposal: not found")
	ErrInvalidState    = errors.New("disposal: not in a state that allows this action")
	ErrAlreadyDisposed = errors.New("disposal: asset cannot be disposed from its current status")
	ErrDisposalExists  = errors.New("disposal: asset already has a disposal or pending disposal request")
	ErrOutOfScope      = errors.New("disposal: office out of scope")
	ErrInvalidRef      = errors.New("disposal: invalid reference")
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

type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
	appr *approval.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service) *Service {
	return &Service{q: q, pool: pool, appr: appr}
}

type SubmitInput struct {
	AssetID      uuid.UUID
	Method       string
	DisposalDate string
	Proceeds     *string
	BookValue    *string
	BastNo       *string
	Reason       *string
}

// Submit validates the asset and opens an approval request. No disposal row is created
// here — the asset_disposal executor creates it on final approval.
func (s *Service) Submit(ctx context.Context, caller approval.Caller, in SubmitInput) (sqlc.ApprovalRequest, error) {
	a, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	if !common.InScope(caller.AllScope, caller.OfficeIDs, a.OfficeID) {
		return sqlc.ApprovalRequest{}, ErrOutOfScope
	}
	if !asset.ValidTransition(a.Status, sqlc.SharedAssetStatusDisposed) {
		return sqlc.ApprovalRequest{}, ErrAlreadyDisposed
	}
	if _, err := s.q.GetDisposalByAsset(ctx, in.AssetID); err == nil {
		return sqlc.ApprovalRequest{}, ErrDisposalExists
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ApprovalRequest{}, err
	}
	pending, err := s.q.CountPendingDisposalRequestsForAsset(ctx, &in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	if pending > 0 {
		return sqlc.ApprovalRequest{}, ErrDisposalExists
	}

	payload, err := marshalPayload(in)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	amount := "0"
	if a.PurchaseCost != nil {
		amount = *a.PurchaseCost
	}
	entity := "asset"
	targetID := in.AssetID
	return s.appr.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetDisposal,
		Amount:       amount,
		OfficeID:     a.OfficeID,
		TargetEntity: &entity,
		TargetID:     &targetID,
		Payload:      payload,
		Reason:       in.Reason,
		Maker:        caller.UserID,
	})
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.DisposalDisposal, error) {
	d, err := s.q.GetDisposal(ctx, sqlc.GetDisposalParams{ID: id, AllScope: all, OfficeIds: ids})
	return d, mapDBError(err)
}

func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, limit, offset int32) ([]sqlc.DisposalDisposal, int64, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListDisposals(ctx, sqlc.ListDisposalsParams{AllScope: all, OfficeIds: ids, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountDisposals(ctx, sqlc.CountDisposalsParams{AllScope: all, OfficeIds: ids})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, all bool, ids []uuid.UUID) ([]sqlc.DisposalDisposal, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListDisposalsByAsset(ctx, sqlc.ListDisposalsByAssetParams{AssetID: assetID, AllScope: all, OfficeIds: ids})
	return rows, mapDBError(err)
}
