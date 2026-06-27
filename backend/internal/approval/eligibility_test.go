package approval

import (
	"testing"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestEligibility(t *testing.T) {
	maker := uuid.New()
	office := uuid.New()
	approver := uuid.New()
	prior := uuid.New()
	req := sqlc.ApprovalRequest{RequestedByID: maker, OfficeID: &office}
	step := sqlc.ApprovalRequestApproval{RequiredLevel: "office"}

	// maker cannot self-approve
	if err := eligibleToDecide(Caller{UserID: maker, AllScope: true}, req, step, nil, office, true); err != ErrSelfApproval {
		t.Errorf("maker self-approve: want ErrSelfApproval, got %v", err)
	}
	// prior approver cannot approve again
	if err := eligibleToDecide(Caller{UserID: prior, AllScope: true}, req, step, []uuid.UUID{prior}, office, true); err != ErrSelfApproval {
		t.Errorf("prior approver: want ErrSelfApproval, got %v", err)
	}
	// tier unsatisfiable
	if err := eligibleToDecide(Caller{UserID: approver, AllScope: true}, req, step, nil, uuid.Nil, false); err != ErrNotEligible {
		t.Errorf("tier missing: want ErrNotEligible, got %v", err)
	}
	// out of scope (does not cover tier office)
	other := uuid.New()
	if err := eligibleToDecide(Caller{UserID: approver, OfficeIDs: []uuid.UUID{other}}, req, step, nil, office, true); err != ErrNotEligible {
		t.Errorf("out of scope: want ErrNotEligible, got %v", err)
	}
	// happy path: global scope, distinct identity
	if err := eligibleToDecide(Caller{UserID: approver, AllScope: true}, req, step, nil, office, true); err != nil {
		t.Errorf("happy: want nil, got %v", err)
	}
}
