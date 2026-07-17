// Package assignment implements asset check-out/check-in (penugasan) and the
// Staf borrow (peminjaman) path via the generic approval engine. Split into
// dto / service / handler / routes (ADR-0008); the service holds business rules
// + data-scope enforcement (Gin-free), scoped by the asset's office.
package assignment

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

var (
	ErrNotFound          = errors.New("assignment: not found")
	ErrOutOfScope        = errors.New("assignment: office out of scope")
	ErrAssetNotAvailable = errors.New("assignment: asset is not available for check-out")
	ErrAlreadyAssigned   = errors.New("assignment: asset already has an active assignment")
	ErrNotActive         = errors.New("assignment: assignment is not active")
	ErrInvalidRef        = errors.New("assignment: invalid reference")
	ErrNoEmployee        = errors.New("assignment: requester has no linked employee")
)

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
			return ErrAlreadyAssigned
		case "23503":
			return ErrInvalidRef
		}
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

type CheckoutInput struct {
	AssetID      uuid.UUID
	EmployeeID   uuid.UUID
	CheckoutDate string // "2006-01-02"
	DueDate      *string
	ConditionOut *string
	Notes        *string
}

type CheckinInput struct {
	CheckinDate      *string
	ConditionIn      *string
	NeedsMaintenance bool
}

type BorrowInput struct {
	AssetID      uuid.UUID
	DueDate      *string
	ConditionOut *string
	Notes        *string
}

func parseDateArg(s *string, def time.Time) (pgtype.Date, error) {
	if s == nil || *s == "" {
		return pgtype.Date{Time: def, Valid: true}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func parseTs(s *string, def time.Time) (pgtype.Timestamptz, error) {
	if s == nil || *s == "" {
		return pgtype.Timestamptz{Time: def, Valid: true}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, nil
}

// checkoutTx performs the shared check-out mutation: insert the assignment row +
// flip the asset to 'assigned', atomically. Caller must have already validated
// scope + availability. assignedBy is the acting user (Manager) or approver.
func checkoutTx(ctx context.Context, qtx *sqlc.Queries, assetID, employeeID, assignedBy uuid.UUID,
	checkoutDate pgtype.Timestamptz, dueDate pgtype.Date, conditionOut, notes *string) (sqlc.AssignmentAssignment, error) {
	a, err := qtx.CheckoutAssignment(ctx, sqlc.CheckoutAssignmentParams{
		AssetID:      assetID,
		EmployeeID:   employeeID,
		AssignedByID: assignedBy,
		CheckoutDate: checkoutDate,
		DueDate:      dueDate,
		ConditionOut: conditionOut,
		Notes:        notes,
	})
	if err != nil {
		return a, mapDBError(err)
	}
	if _, err := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: assetID, Status: sqlc.SharedAssetStatusAssigned}); err != nil {
		return a, mapDBError(err)
	}
	return a, nil
}

// Checkout assigns an available asset to an employee (Manager direct action).
func (s *Service) Checkout(ctx context.Context, all bool, ids []uuid.UUID, assignedBy uuid.UUID, in CheckoutInput) (sqlc.AssignmentAssignment, error) {
	asset, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.AssignmentAssignment{}, mapDBError(err)
	}
	if !common.InScope(all, ids, asset.OfficeID) {
		return sqlc.AssignmentAssignment{}, ErrOutOfScope
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return sqlc.AssignmentAssignment{}, ErrAssetNotAvailable
	}
	coDate, err := parseTs(&in.CheckoutDate, time.Now())
	if err != nil {
		return sqlc.AssignmentAssignment{}, ErrInvalidRef
	}
	dueDate, err := parseDateArg(in.DueDate, time.Time{})
	if err != nil {
		return sqlc.AssignmentAssignment{}, ErrInvalidRef
	}
	if in.DueDate == nil || *in.DueDate == "" {
		dueDate = pgtype.Date{} // NULL
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.AssignmentAssignment{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	a, err := checkoutTx(ctx, qtx, in.AssetID, in.EmployeeID, assignedBy, coDate, dueDate, in.ConditionOut, in.Notes)
	if err != nil {
		return a, err
	}
	if err := tx.Commit(ctx); err != nil {
		return a, err
	}
	return a, nil
}

// Checkin returns an active assignment; the asset goes back to available, or to
// under_maintenance when NeedsMaintenance is set. Returns (before, after).
// checkedInBy is the acting user: it decides whether the check-in notification
// is worth emitting (see enqueueCheckin).
func (s *Service) Checkin(ctx context.Context, all bool, ids []uuid.UUID, id uuid.UUID, checkedInBy uuid.UUID, in CheckinInput) (before, after sqlc.AssignmentAssignment, err error) {
	before, err = s.q.GetAssignmentScoped(ctx, sqlc.GetAssignmentScopedParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, before, mapDBError(err)
	}
	if before.Status != sqlc.SharedAssignmentStatusActive {
		return before, before, ErrNotActive
	}
	ciDate, err := parseTs(in.CheckinDate, time.Now())
	if err != nil {
		return before, before, ErrInvalidRef
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return before, before, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	after, err = qtx.CheckinAssignment(ctx, sqlc.CheckinAssignmentParams{ID: id, CheckinDate: ciDate, ConditionIn: in.ConditionIn})
	if err != nil {
		return before, before, mapDBError(err)
	}
	newStatus := sqlc.SharedAssetStatusAvailable
	if in.NeedsMaintenance {
		newStatus = sqlc.SharedAssetStatusUnderMaintenance
	}
	// SetAssetStatus returns the asset row, so the event's i18n params (tag +
	// name) come from the mutation already in flight -- no extra read.
	asset, err := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: before.AssetID, Status: newStatus})
	if err != nil {
		return before, before, mapDBError(err)
	}
	// Transactional outbox: the event lands in the same tx as the business
	// change, so a rollback leaves no orphan event and a commit cannot lose one.
	if err = s.enqueueCheckin(ctx, qtx, after, asset, checkedInBy); err != nil {
		return before, before, err
	}
	if err = tx.Commit(ctx); err != nil {
		return before, before, err
	}
	return before, after, nil
}

// SubmitBorrow opens an assignment-type approval request for a Staf borrow. The
// asset must be available; the approval routes to the asset office's approvers.
func (s *Service) SubmitBorrow(ctx context.Context, caller approval.Caller, in BorrowInput) (sqlc.ApprovalRequest, error) {
	asset, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	// Scope: caller must have the asset's office in scope (mirrors transfer.Submit) —
	// otherwise a Staf could open a borrow request against an asset outside their
	// data scope by supplying an arbitrary asset UUID.
	if !common.InScope(caller.AllScope, caller.OfficeIDs, asset.OfficeID) {
		return sqlc.ApprovalRequest{}, ErrOutOfScope
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return sqlc.ApprovalRequest{}, ErrAssetNotAvailable
	}
	payload, err := marshalBorrowPayload(in)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	entity := "asset"
	targetID := in.AssetID
	return s.appr.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssignment,
		Amount:       "0",
		OfficeID:     asset.OfficeID,
		TargetEntity: &entity,
		TargetID:     &targetID,
		Payload:      payload,
		Reason:       in.Notes,
		Maker:        caller.UserID,
	})
}

// Available lists available assets within the caller's data scope for the
// assignments module: global (Superadmin) → all; office_subtree (Manager) → the
// subtree; own (Staf) → the caller's own office. Backs both the Manager check-out
// picker and the Staf borrow picker — CallerOfficeScope resolves each role to the
// right set (an 'own' Staf resolves to their own office, exactly the set they may
// borrow from).
func (s *Service) Available(ctx context.Context, all bool, ids []uuid.UUID) ([]sqlc.AssetAsset, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	st := sqlc.SharedAssetStatusAvailable
	rows, err := s.q.ListAssets(ctx, sqlc.ListAssetsParams{
		AllScope:  all,
		OfficeIds: ids,
		Status:    &st,
		Lim:       100,
		Off:       0,
	})
	return rows, mapDBError(err)
}

// Get returns one scoped, enriched assignment.
func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.GetAssignmentEnrichedRow, error) {
	r, err := s.q.GetAssignmentEnriched(ctx, sqlc.GetAssignmentEnrichedParams{ID: id, AllScope: all, OfficeIds: ids})
	return r, mapDBError(err)
}

// List returns a scoped, paginated, enriched page + total.
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, status string, employeeID *uuid.UUID, search string, limit, offset int32) ([]sqlc.ListAssignmentsEnrichedRow, int64, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	var st *sqlc.SharedAssignmentStatus
	if status != "" {
		v := sqlc.SharedAssignmentStatus(status)
		st = &v
	}
	var sr *string
	if search != "" {
		sr = &search
	}
	rows, err := s.q.ListAssignmentsEnriched(ctx, sqlc.ListAssignmentsEnrichedParams{
		AllScope: all, OfficeIds: ids, Status: st, EmployeeID: employeeID, Search: sr, Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountAssignments(ctx, sqlc.CountAssignmentsParams{
		AllScope: all, OfficeIds: ids, Status: st, EmployeeID: employeeID, Search: sr,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// Mine returns the caller's own assignments (optionally filtered by status),
// enriched, newest activity first. AllScope is forced true and OfficeIds empty
// here — safe ONLY because employeeID is resolved server-side from the caller's
// JWT-derived user record (never client-supplied), so EmployeeID still narrows
// every row to that one employee regardless of the scope bypass. This mirrors
// the /assignments/available precedent: a dedicated, permission-gated
// (request.create) endpoint rather than widening the general list's data scope
// or granting Staf assignment.view, which would let a Staf enumerate every
// coworker's assignments in the office via the plain employee_id query filter.
func (s *Service) Mine(ctx context.Context, employeeID uuid.UUID, status string) ([]sqlc.ListAssignmentsEnrichedRow, error) {
	var st *sqlc.SharedAssignmentStatus
	if status != "" {
		v := sqlc.SharedAssignmentStatus(status)
		st = &v
	}
	rows, err := s.q.ListAssignmentsEnriched(ctx, sqlc.ListAssignmentsEnrichedParams{
		AllScope: true, OfficeIds: []uuid.UUID{}, Status: st, EmployeeID: &employeeID, Search: nil, Lim: 100, Off: 0,
	})
	return rows, mapDBError(err)
}

// ListByAsset returns a scoped, enriched assignment history for one asset.
func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, all bool, ids []uuid.UUID) ([]sqlc.ListAssignmentsByAssetEnrichedRow, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListAssignmentsByAssetEnriched(ctx, sqlc.ListAssignmentsByAssetEnrichedParams{AssetID: assetID, AllScope: all, OfficeIds: ids})
	return rows, mapDBError(err)
}
