package approval

import (
	"sort"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

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
