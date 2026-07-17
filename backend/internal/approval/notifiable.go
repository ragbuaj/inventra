package approval

import (
	"context"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// scopeModule is the data_scope_policies module the approval module resolves
// callers against. It must stay identical to the module string used when a real
// caller decides (handler.callerFromCtx), or the set of users we notify would
// diverge from the set actually allowed to act.
const scopeModule = "requests"

// decidePermission gates POST /requests/:id/approve and /reject.
const decidePermission = "request.decide"

// NotifiableApprovers returns the users currently eligible to decide the given
// step of req: the inverse of the (request, caller) -> bool predicate the rest
// of the module is built on.
//
// Eligibility is decided solely by eligibleToDecide, run once per candidate.
// The segregation-of-duty rules (maker exclusion, no repeat approver), the tier
// and the data scope are deliberately not restated here nor pushed into SQL: a
// second copy would drift from the predicate that guards the real decision, and
// the two would disagree about who may see that a request exists.
func (s *Service) NotifiableApprovers(ctx context.Context, req sqlc.ApprovalRequest, step sqlc.ApprovalRequestApproval) ([]uuid.UUID, error) {
	// Submit guarantees office_id is non-nil; without it no tier office can be
	// resolved, so nobody is eligible. Decide/Inbox treat nil the same way.
	if req.OfficeID == nil {
		return []uuid.UUID{}, nil
	}

	approvals, err := s.q.ListRequestApprovals(ctx, req.ID)
	if err != nil {
		return nil, mapDBError(err)
	}
	prior := priorApprovers(approvals, req.CurrentStep)

	anc, err := s.ancestorsFor(ctx, *req.OfficeID)
	if err != nil {
		return nil, err
	}
	tierOffice, tierOK := resolveTierOffice(anc, *req.OfficeID, step.RequiredLevel)

	candidates, err := s.q.ListUsersWithPermission(ctx, decidePermission)
	if err != nil {
		return nil, mapDBError(err)
	}

	out := make([]uuid.UUID, 0, len(candidates))
	for _, cand := range candidates {
		caller, err := s.callerFor(ctx, cand)
		if err != nil {
			return nil, err
		}
		if eligibleToDecide(caller, req, step, prior, tierOffice, tierOK) == nil {
			out = append(out, cand.ID)
		}
	}
	return out, nil
}

// priorApprovers collects the approvers who already decided an earlier step:
// the same derivation Decide and Inbox feed to eligibleToDecide.
func priorApprovers(approvals []sqlc.ApprovalRequestApproval, currentStep int32) []uuid.UUID {
	var prior []uuid.UUID
	for _, a := range approvals {
		if a.StepOrder < currentStep && a.ApproverID != nil {
			prior = append(prior, *a.ApproverID)
		}
	}
	return prior
}

// callerFor builds a candidate's Caller outside any Gin context, since the
// fan-out runs in a worker rather than an HTTP request. It mirrors
// masterdata/common.CallerOfficeScope (same precedent as
// importer.resolveMakerScope), including its "own" -> caller's own office
// translation: a bare ScopeService.Resolve leaves OfficeIDs empty for "own",
// which would silently notify a narrower set than the one allowed to decide.
func (s *Service) callerFor(ctx context.Context, u sqlc.ListUsersWithPermissionRow) (Caller, error) {
	sc, err := s.scope.Resolve(ctx, u.RoleID, u.OfficeID, scopeModule)
	if err != nil {
		return Caller{}, err
	}
	caller := Caller{UserID: u.ID, RoleID: u.RoleID}
	switch sc.Level {
	case sqlc.SharedScopeLevelGlobal:
		caller.AllScope = true
	case sqlc.SharedScopeLevelOwn:
		if u.OfficeID != nil {
			caller.OfficeIDs = []uuid.UUID{*u.OfficeID}
		} else {
			caller.OfficeIDs = []uuid.UUID{}
		}
	default: // office / office_subtree
		caller.OfficeIDs = sc.OfficeIDs
	}
	return caller, nil
}
