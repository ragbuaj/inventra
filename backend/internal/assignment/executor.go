package assignment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// assignmentExec performs the check-out on final approval of a peminjaman, inside
// the commit tx. Employee = the requester's linked employee; assigned_by = approver.
type assignmentExec struct{ s *Service }

func (e assignmentExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return approval.ErrInvalidRef
	}
	var p BorrowPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return approval.ErrInvalidRef
	}
	// Resolve the requester's linked employee (the borrower).
	u, err := qtx.GetUserByID(ctx, req.RequestedByID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
	}
	if u.EmployeeID == nil {
		return approval.ErrInvalidRef // requester has no employee → cannot borrow
	}
	asset, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
	}
	if asset.Status != sqlc.SharedAssetStatusAvailable {
		return approval.ErrConflict // asset no longer available at approval time
	}
	if req.DecidedByID == nil {
		return approval.ErrInvalidRef
	}
	var due pgtype.Date
	if p.DueDate != nil && *p.DueDate != "" {
		t, perr := time.Parse("2006-01-02", *p.DueDate)
		if perr != nil {
			return approval.ErrInvalidRef
		}
		due = pgtype.Date{Time: t, Valid: true}
	}
	coDate := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	_, err = checkoutTx(ctx, qtx, *req.TargetID, *u.EmployeeID, *req.DecidedByID, coDate, due, p.ConditionOut, p.Notes)
	return err
}

// Executor returns the assignment approval executor.
func (s *Service) Executor() approval.Executor { return assignmentExec{s} }
