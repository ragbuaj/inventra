// Package stockopname implements physical stock-take (stock opname): session
// lifecycle (open → counting → reconciling → closed), asset snapshotting per
// office, scanning/result recording, and KPI reporting. Split into dto /
// service / handler / routes (ADR-0008). The service holds business rules +
// data-scope enforcement (Gin-free); the handler maps HTTP ↔ service.
package stockopname

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
	"github.com/ragbuaj/inventra/internal/disposal"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/transfer"
)

// Sentinel errors (mapped to HTTP status by the handler).
var (
	ErrNotFound          = errors.New("stockopname: not found")
	ErrOutOfScope        = errors.New("stockopname: office out of scope")
	ErrInvalidState      = errors.New("stockopname: not in a state that allows this action")
	ErrInvalidRef        = errors.New("stockopname: invalid reference")
	ErrAlreadyFollowedUp = errors.New("stockopname: item already has a follow-up request")
	ErrNoItem            = errors.New("stockopname: asset not found in this session")
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

// Service holds data access + business rules for stock opname.
type Service struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
	disp *disposal.Service
	tr   *transfer.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, disp *disposal.Service, tr *transfer.Service) *Service {
	return &Service{q: q, pool: pool, disp: disp, tr: tr}
}

// CreateInput carries the parameters to open a new stock-opname session.
type CreateInput struct {
	OfficeID uuid.UUID
	Name     *string
	Period   time.Time
}

// CreateSession opens a session and snapshots every in-scope, non-disposed
// asset of the office, atomically.
func (s *Service) CreateSession(ctx context.Context, caller approval.Caller, in CreateInput) (sqlc.StockopnameStockOpnameSession, error) {
	if !common.InScope(caller.AllScope, caller.OfficeIDs, in.OfficeID) {
		return sqlc.StockopnameStockOpnameSession{}, ErrOutOfScope
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.StockopnameStockOpnameSession{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	sess, err := qtx.CreateOpnameSession(ctx, sqlc.CreateOpnameSessionParams{
		OfficeID:    in.OfficeID,
		Name:        in.Name,
		Period:      pgtype.Date{Time: in.Period, Valid: true},
		StartedByID: caller.UserID,
	})
	if err != nil {
		return sqlc.StockopnameStockOpnameSession{}, mapDBError(err)
	}
	if err := qtx.SnapshotSessionItems(ctx, sqlc.SnapshotSessionItemsParams{
		SessionID: sess.ID,
		OfficeID:  in.OfficeID,
	}); err != nil {
		return sqlc.StockopnameStockOpnameSession{}, mapDBError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.StockopnameStockOpnameSession{}, err
	}
	return sess, nil
}

// SessionKpis mirrors sqlc.SessionKpisRow for callers outside this package.
type SessionKpis = sqlc.SessionKpisRow

// GetSession returns one scoped session (with resolved office/user names) plus
// its item-result KPIs.
func (s *Service) GetSession(ctx context.Context, caller approval.Caller, id uuid.UUID) (sqlc.GetOpnameSessionRow, SessionKpis, error) {
	ids := caller.OfficeIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	sess, err := s.q.GetOpnameSession(ctx, sqlc.GetOpnameSessionParams{
		ID:        id,
		AllScope:  caller.AllScope,
		OfficeIds: ids,
	})
	if err != nil {
		return sqlc.GetOpnameSessionRow{}, SessionKpis{}, mapDBError(err)
	}
	kpi, err := s.q.SessionKpis(ctx, id)
	if err != nil {
		return sqlc.GetOpnameSessionRow{}, SessionKpis{}, mapDBError(err)
	}
	return sess, kpi, nil
}

// ListSessions returns a scoped, paginated page of sessions + total. status
// nil means no filter.
func (s *Service) ListSessions(ctx context.Context, caller approval.Caller, status *string, limit, offset int32) ([]sqlc.ListOpnameSessionsRow, int64, error) {
	ids := caller.OfficeIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	var st *sqlc.SharedOpnameSessionStatus
	if status != nil && *status != "" {
		v := sqlc.SharedOpnameSessionStatus(*status)
		st = &v
	}
	rows, err := s.q.ListOpnameSessions(ctx, sqlc.ListOpnameSessionsParams{
		AllScope:  caller.AllScope,
		OfficeIds: ids,
		Status:    st,
		Lim:       limit,
		Off:       offset,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountOpnameSessions(ctx, sqlc.CountOpnameSessionsParams{
		AllScope:  caller.AllScope,
		OfficeIds: ids,
		Status:    st,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// canTransition reports whether the session status graph allows from → to.
func canTransition(from, to sqlc.SharedOpnameSessionStatus) bool {
	switch from {
	case sqlc.SharedOpnameSessionStatusOpen:
		return to == sqlc.SharedOpnameSessionStatusCounting
	case sqlc.SharedOpnameSessionStatusCounting:
		return to == sqlc.SharedOpnameSessionStatusReconciling
	case sqlc.SharedOpnameSessionStatusReconciling:
		return to == sqlc.SharedOpnameSessionStatusClosed
	default:
		return false
	}
}

// Transition drives the session's state machine. GetSession enforces scope +
// existence first; close stamps closed_by_id/closed_at.
func (s *Service) Transition(ctx context.Context, caller approval.Caller, id uuid.UUID, to sqlc.SharedOpnameSessionStatus) (sqlc.StockopnameStockOpnameSession, error) {
	cur, _, err := s.GetSession(ctx, caller, id)
	if err != nil {
		return sqlc.StockopnameStockOpnameSession{}, err
	}
	from := cur.StockopnameStockOpnameSession.Status
	if !canTransition(from, to) {
		return sqlc.StockopnameStockOpnameSession{}, ErrInvalidState
	}

	var closedBy *uuid.UUID
	var closedAt pgtype.Timestamptz
	if to == sqlc.SharedOpnameSessionStatusClosed {
		closedBy = &caller.UserID
		closedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	out, err := s.q.SetSessionStatus(ctx, sqlc.SetSessionStatusParams{
		Status:     to,
		ClosedByID: closedBy,
		ClosedAt:   closedAt,
		ID:         id,
	})
	if err != nil {
		return sqlc.StockopnameStockOpnameSession{}, mapDBError(err)
	}
	return out, nil
}

// SetItemResult records a counted result for one item. Only allowed while the
// session is 'counting'; stamps counted_by/at.
func (s *Service) SetItemResult(ctx context.Context, caller approval.Caller, sessionID, itemID uuid.UUID, result sqlc.SharedOpnameItemResult, note *string) (sqlc.StockopnameStockOpnameItem, error) {
	sess, _, err := s.GetSession(ctx, caller, sessionID)
	if err != nil {
		return sqlc.StockopnameStockOpnameItem{}, err
	}
	if sess.StockopnameStockOpnameSession.Status != sqlc.SharedOpnameSessionStatusCounting {
		return sqlc.StockopnameStockOpnameItem{}, ErrInvalidState
	}

	row, err := s.q.SetOpnameItemResult(ctx, sqlc.SetOpnameItemResultParams{
		ID:          itemID,
		SessionID:   sessionID,
		Result:      result,
		Note:        note,
		CountedByID: &caller.UserID,
	})
	if err != nil {
		return sqlc.StockopnameStockOpnameItem{}, mapDBError(err)
	}
	return row, nil
}

// Scan resolves a scanned asset tag against an in-progress session: returns
// the matching item if it's already in the session, or — for an in-scope
// asset not yet in the snapshot — inserts and returns a new expected=false
// item. Only allowed while the session is 'counting'.
func (s *Service) Scan(ctx context.Context, caller approval.Caller, sessionID uuid.UUID, tag string) (sqlc.StockopnameStockOpnameItem, error) {
	sess, _, err := s.GetSession(ctx, caller, sessionID)
	if err != nil {
		return sqlc.StockopnameStockOpnameItem{}, err
	}
	if sess.StockopnameStockOpnameSession.Status != sqlc.SharedOpnameSessionStatusCounting {
		return sqlc.StockopnameStockOpnameItem{}, ErrInvalidState
	}

	item, err := s.q.GetOpnameItemByTag(ctx, sqlc.GetOpnameItemByTagParams{SessionID: sessionID, AssetTag: tag})
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.StockopnameStockOpnameItem{}, mapDBError(err)
	}

	asset, err := s.q.GetAssetByTag(ctx, tag)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.StockopnameStockOpnameItem{}, ErrNoItem
		}
		return sqlc.StockopnameStockOpnameItem{}, mapDBError(err)
	}
	if !common.InScope(caller.AllScope, caller.OfficeIDs, asset.OfficeID) {
		return sqlc.StockopnameStockOpnameItem{}, ErrOutOfScope
	}

	inserted, err := s.q.InsertUnexpectedItem(ctx, sqlc.InsertUnexpectedItemParams{SessionID: sessionID, AssetID: asset.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING: another scan/snapshot already inserted
			// this item — return the existing row rather than treating this
			// as a real not-found.
			existing, getErr := s.q.GetOpnameItemByTag(ctx, sqlc.GetOpnameItemByTagParams{SessionID: sessionID, AssetTag: tag})
			if getErr != nil {
				return sqlc.StockopnameStockOpnameItem{}, mapDBError(getErr)
			}
			return existing, nil
		}
		return sqlc.StockopnameStockOpnameItem{}, mapDBError(err)
	}
	return inserted, nil
}

// ListItems returns a scoped session's items (enriched with asset/office/room/
// floor/counted-by names), optionally filtered by result.
func (s *Service) ListItems(ctx context.Context, caller approval.Caller, sessionID uuid.UUID, result *string) ([]sqlc.ListOpnameItemsEnrichedRow, error) {
	if _, _, err := s.GetSession(ctx, caller, sessionID); err != nil {
		return nil, err
	}
	var res *sqlc.SharedOpnameItemResult
	if result != nil && *result != "" {
		v := sqlc.SharedOpnameItemResult(*result)
		res = &v
	}
	rows, err := s.q.ListOpnameItemsEnriched(ctx, sqlc.ListOpnameItemsEnrichedParams{SessionID: sessionID, Result: res})
	if err != nil {
		return nil, mapDBError(err)
	}
	return rows, nil
}
