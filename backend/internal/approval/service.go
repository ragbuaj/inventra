package approval

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Sentinel errors — reused by handler and other tasks.
var (
	ErrSelfApproval = errors.New("approval: maker or prior approver cannot approve")
	ErrNotEligible  = errors.New("approval: caller is not eligible for this step")
	ErrNoThreshold  = errors.New("approval: no threshold configured for this amount")
	ErrInvalidState = errors.New("approval: request is not in a state that allows this action")
	ErrNotFound     = errors.New("approval: record not found")
	ErrForbidden    = errors.New("approval: caller lacks permission")
	ErrConflict     = errors.New("approval: duplicate record")
	ErrInvalidRef   = errors.New("approval: invalid reference")
)

// mapDBError translates pgx/Postgres errors into package sentinel errors.
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
			return ErrConflict
		case "23503":
			return ErrInvalidRef
		}
	}
	return err
}

// Service holds the data-access and business-logic layer for the approval module.
type Service struct {
	q     *sqlc.Queries
	pool  *pgxpool.Pool
	scope *authz.ScopeService
	rdb   *redis.Client
	exec  registry
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, scope *authz.ScopeService, rdb *redis.Client) *Service {
	return &Service{q: q, pool: pool, scope: scope, rdb: rdb, exec: registry{}}
}

// RegisterExecutor registers a side-effect executor for the given request type.
func (s *Service) RegisterExecutor(t sqlc.SharedRequestType, e Executor) { s.exec[t] = e }

// SubmitInput carries the data needed to open a new approval request.
type SubmitInput struct {
	Type         sqlc.SharedRequestType
	Amount       string
	OfficeID     uuid.UUID
	TargetEntity *string
	TargetID     *uuid.UUID
	Payload      []byte
	Reason       *string
	Maker        uuid.UUID
}

// Submit resolves the approval chain for the given amount, creates the request
// and its per-step approval rows atomically inside a transaction.
func (s *Service) Submit(ctx context.Context, in SubmitInput) (sqlc.ApprovalRequest, error) {
	// Every request must be tied to a real office so it is always visible in Inbox.
	if in.OfficeID == uuid.Nil {
		return sqlc.ApprovalRequest{}, ErrInvalidState
	}

	steps, err := s.q.MatchThresholdSteps(ctx, sqlc.MatchThresholdStepsParams{
		RequestType: in.Type,
		Amount:      in.Amount,
	})
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	chain := buildChain(steps)
	if len(chain) == 0 {
		return sqlc.ApprovalRequest{}, ErrNoThreshold
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := s.q.WithTx(tx)

	req, err := qtx.CreateRequest(ctx, sqlc.CreateRequestParams{
		Type:          in.Type,
		OfficeID:      &in.OfficeID,
		Amount:        &in.Amount,
		TargetEntity:  in.TargetEntity,
		TargetID:      in.TargetID,
		Payload:       in.Payload,
		Reason:        in.Reason,
		RequestedByID: in.Maker,
	})
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}

	for _, st := range chain {
		if _, err := qtx.CreateRequestApproval(ctx, sqlc.CreateRequestApprovalParams{
			RequestID:     req.ID,
			StepOrder:     st.Order,
			RequiredLevel: st.Level,
		}); err != nil {
			return sqlc.ApprovalRequest{}, mapDBError(err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	return req, nil
}

// Caller carries the resolved identity and scope of the acting user.
type Caller struct {
	UserID    uuid.UUID
	RoleID    uuid.UUID
	AllScope  bool
	OfficeIDs []uuid.UUID
}

// eligibleToDecide returns nil when the caller may act on the given approval step,
// or a sentinel error when a segregation-of-duty or scope rule is violated.
func eligibleToDecide(
	caller Caller,
	req sqlc.ApprovalRequest,
	_ sqlc.ApprovalRequestApproval,
	priorApprovers []uuid.UUID,
	tierOffice uuid.UUID,
	tierOK bool,
) error {
	// SoD: maker cannot approve their own request.
	if caller.UserID == req.RequestedByID {
		return ErrSelfApproval
	}
	// SoD: no repeat approver across steps.
	for _, p := range priorApprovers {
		if p == caller.UserID {
			return ErrSelfApproval
		}
	}
	// Tier must be satisfiable.
	if !tierOK {
		return ErrNotEligible
	}
	// Caller's data scope must cover the tier office.
	if !common.InScope(caller.AllScope, caller.OfficeIDs, tierOffice) {
		return ErrNotEligible
	}
	return nil
}

type chainStep struct {
	Order int32
	Level sqlc.SharedApproverLevel
}

func buildChain(steps []sqlc.ApprovalApprovalThreshold) []chainStep {
	out := make([]chainStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, chainStep{Order: s.StepOrder, Level: s.RequiredLevel})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// resolveTierOffice returns the ancestor office satisfying the required approver level.
// office/office_subtree => the origin office itself; wilayah/pusat => nearest ancestor with that tier.
func resolveTierOffice(anc []sqlc.GetOfficeAncestorsRow, originID uuid.UUID, level sqlc.SharedApproverLevel) (uuid.UUID, bool) {
	switch level {
	case sqlc.SharedApproverLevelOffice, sqlc.SharedApproverLevelOfficeSubtree:
		return originID, true
	default:
		for _, a := range anc {
			if a.Tier != nil && *a.Tier == level {
				return a.ID, true
			}
		}
		return uuid.Nil, false
	}
}

// ancestorsFor returns the ancestor chain for the given office by querying GetOfficeAncestors directly.
func (s *Service) ancestorsFor(ctx context.Context, officeID uuid.UUID) ([]sqlc.GetOfficeAncestorsRow, error) {
	return s.q.GetOfficeAncestors(ctx, officeID)
}

// Decide records an approve or reject decision on the current step of a pending request.
// On rejection it finalises the request immediately. On the final approval it runs the
// registered executor inside the same transaction so the side-effect is atomic.
func (s *Service) Decide(ctx context.Context, requestID uuid.UUID, caller Caller, approve bool, note *string) (sqlc.ApprovalRequest, error) {
	req, err := s.q.GetRequest(ctx, requestID)
	if err != nil {
		return req, mapDBError(err)
	}
	if req.Status != sqlc.SharedRequestStatusPending {
		return req, ErrInvalidState
	}

	approvals, err := s.q.ListRequestApprovals(ctx, requestID)
	if err != nil {
		return req, mapDBError(err)
	}

	var step sqlc.ApprovalRequestApproval
	var prior []uuid.UUID
	found := false
	for _, a := range approvals {
		if a.StepOrder < req.CurrentStep && a.ApproverID != nil {
			prior = append(prior, *a.ApproverID)
		}
		if a.StepOrder == req.CurrentStep {
			step = a
			found = true
		}
	}
	if !found {
		return req, ErrInvalidState
	}

	var tierOffice uuid.UUID
	tierOK := false
	if req.OfficeID != nil {
		anc, err := s.ancestorsFor(ctx, *req.OfficeID)
		if err != nil {
			return req, err
		}
		tierOffice, tierOK = resolveTierOffice(anc, *req.OfficeID, step.RequiredLevel)
	}
	if err := eligibleToDecide(caller, req, step, prior, tierOffice, tierOK); err != nil {
		return req, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return req, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	// Re-load with a row lock so two concurrent approvers cannot both pass this point.
	// We abort if the request moved to a different step or is no longer pending since
	// our pre-tx eligibility check — the eligibility itself remains valid because the
	// chain configuration did not change.
	locked, err := qtx.GetRequestForUpdate(ctx, requestID)
	if err != nil {
		return req, mapDBError(err)
	}
	if locked.Status != sqlc.SharedRequestStatusPending || locked.CurrentStep != req.CurrentStep {
		return req, ErrInvalidState
	}

	if !approve {
		if _, err := qtx.DecideRequestApproval(ctx, sqlc.DecideRequestApprovalParams{
			RequestID:  requestID,
			StepOrder:  step.StepOrder,
			ApproverID: &caller.UserID,
			Decision:   sqlc.SharedRequestStatusRejected,
			Note:       note,
		}); err != nil {
			return req, mapDBError(err)
		}
		out, err := qtx.SetRequestDecision(ctx, sqlc.SetRequestDecisionParams{
			ID:           requestID,
			Status:       sqlc.SharedRequestStatusRejected,
			DecidedByID:  &caller.UserID,
			DecisionNote: note,
		})
		if err != nil {
			return req, mapDBError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return req, err
		}
		return out, nil
	}

	// approve path
	if _, err := qtx.DecideRequestApproval(ctx, sqlc.DecideRequestApprovalParams{
		RequestID:  requestID,
		StepOrder:  step.StepOrder,
		ApproverID: &caller.UserID,
		Decision:   sqlc.SharedRequestStatusApproved,
		Note:       note,
	}); err != nil {
		return req, mapDBError(err)
	}

	// isLast: compare against the highest step_order in the chain.
	// approvals is ORDER BY step_order and non-empty at this point, so the last element is safe.
	maxStep := approvals[len(approvals)-1].StepOrder
	isLast := step.StepOrder == maxStep
	if !isLast {
		out, err := qtx.AdvanceRequestStep(ctx, requestID)
		if err != nil {
			return req, mapDBError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return req, err
		}
		return out, nil
	}

	// final step: mark approved and run executor in the same tx
	out, err := qtx.SetRequestDecision(ctx, sqlc.SetRequestDecisionParams{
		ID:           requestID,
		Status:       sqlc.SharedRequestStatusApproved,
		DecidedByID:  &caller.UserID,
		DecisionNote: note,
	})
	if err != nil {
		return req, mapDBError(err)
	}
	exec, ok := s.exec[req.Type]
	if !ok {
		return req, ErrInvalidState
	}
	if err := exec.Execute(ctx, qtx, out); err != nil {
		return req, err
	}
	if err := tx.Commit(ctx); err != nil {
		return req, err
	}
	return out, nil
}

// Cancel cancels a pending request on behalf of its maker.
// Returns ErrNotFound if the request is not pending or does not belong to maker.
func (s *Service) Cancel(ctx context.Context, requestID, maker uuid.UUID) (sqlc.ApprovalRequest, error) {
	out, err := s.q.CancelRequest(ctx, sqlc.CancelRequestParams{ID: requestID, RequestedByID: maker})
	if err != nil {
		return out, mapDBError(err)
	}
	return out, nil
}

// Inbox returns all pending requests for which the caller is currently eligible to decide.
func (s *Service) Inbox(ctx context.Context, caller Caller) ([]sqlc.ApprovalRequest, error) {
	candidates, err := s.q.ListInboxCandidates(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	out := make([]sqlc.ApprovalRequest, 0)
	for _, req := range candidates {
		approvals, err := s.q.ListRequestApprovals(ctx, req.ID)
		if err != nil {
			return nil, mapDBError(err)
		}
		var step sqlc.ApprovalRequestApproval
		var prior []uuid.UUID
		found := false
		for _, a := range approvals {
			if a.StepOrder < req.CurrentStep && a.ApproverID != nil {
				prior = append(prior, *a.ApproverID)
			}
			if a.StepOrder == req.CurrentStep {
				step = a
				found = true
			}
		}
		// Submit guarantees office_id is non-nil; the nil check here is purely defensive.
		if !found || req.OfficeID == nil {
			continue
		}
		anc, err := s.ancestorsFor(ctx, *req.OfficeID)
		if err != nil {
			return nil, err
		}
		to, ok := resolveTierOffice(anc, *req.OfficeID, step.RequiredLevel)
		if eligibleToDecide(caller, req, step, prior, to, ok) == nil {
			out = append(out, req)
		}
	}
	return out, nil
}
