package approval

import (
	"errors"
	"sort"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
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
)

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
