//go:build integration

package approval_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// These tests run against a real Postgres because the two things most likely to
// get the inverse wrong -- data-scope resolution and the office ancestor CTE --
// are exactly what a fake would fake away.

// notifiableFixture wires a Service plus the seeded office tree and roles.
type notifiableFixture struct {
	outboxFixture
	q *sqlc.Queries
}

func newNotifiableFixture(t *testing.T, categoryCode string) notifiableFixture {
	t.Helper()
	f := newOutboxFixture(t, categoryCode)
	return notifiableFixture{outboxFixture: f, q: sqlc.New(f.pool)}
}

// seedUserAtOffice inserts an active identity.users row placed at officeID.
// Placement matters here: data scope is resolved from the user's own office.
func seedUserAtOffice(t *testing.T, pool *pgxpool.Pool, roleID, officeID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		email, email, roleID, officeID).Scan(&id))
	return id
}

// seedSiblingOffice adds a second Cabang under Wilayah, so "out of scope" can be
// a real sibling branch rather than an invented uuid.
func seedSiblingOffice(t *testing.T, pool *pgxpool.Pool, tr tieredOfficeTree) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, 'Cabang Beta', 'CBB') RETURNING id`,
		tr.WilayahID, tr.CabangTypeID).Scan(&id))
	return id
}

// currentStepOf returns the request's live row plus the approval row for the
// step it is currently waiting on -- the pair NotifiableApprovers takes.
func currentStepOf(t *testing.T, q *sqlc.Queries, requestID uuid.UUID) (sqlc.ApprovalRequest, sqlc.ApprovalRequestApproval) {
	t.Helper()
	ctx := context.Background()
	req, err := q.GetRequest(ctx, requestID)
	require.NoError(t, err)
	approvals, err := q.ListRequestApprovals(ctx, requestID)
	require.NoError(t, err)
	for _, a := range approvals {
		if a.StepOrder == req.CurrentStep {
			return req, a
		}
	}
	t.Fatalf("no approval row for current step %d", req.CurrentStep)
	return sqlc.ApprovalRequest{}, sqlc.ApprovalRequestApproval{}
}

func notifiable(t *testing.T, f notifiableFixture, requestID uuid.UUID) []uuid.UUID {
	t.Helper()
	req, step := currentStepOf(t, f.q, requestID)
	got, err := f.svc.NotifiableApprovers(context.Background(), req, step)
	require.NoError(t, err)
	return got
}

// TestNotifiable_MakerExcluded is the SoD guarantee: a maker is never told to
// approve their own request. The control is what makes it real -- the very same
// user, with the very same role and office, IS returned for a request they did
// not submit. So the exclusion is maker-ness, not a silently empty list.
func TestNotifiable_MakerExcluded(t *testing.T) {
	f := newNotifiableFixture(t, "NMA")

	// Kepala Unit holds request.decide, so the maker is a genuine candidate
	// that only the SoD rule removes.
	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.maker@test.local")
	other := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.other@test.local")

	// 5M stays inside the single-step (office tier) chain.
	own := f.submit(t, maker, "Router 5M", "5000000")
	foreign := f.submit(t, other, "Switch 5M", "5000000")

	ownGot := notifiable(t, f, own.ID)
	assert.NotContains(t, ownGot, maker, "maker must never be notified about their own request")
	assert.Contains(t, ownGot, other, "an eligible non-maker approver must be notified")

	// Control: identical user, identical scope, different maker -> now eligible.
	foreignGot := notifiable(t, f, foreign.ID)
	assert.Contains(t, foreignGot, maker,
		"maker is only excluded from their OWN request; otherwise they are a real candidate")
	assert.NotContains(t, foreignGot, other, "and the roles reverse for the other request")
}

// TestNotifiable_PriorApproverExcluded is the no-repeat-approver SoD rule.
// Both users are Kepala Kanwil at Wilayah, so their scope covers both the
// step-1 (Cabang) and step-2 (Wilayah) tier offices. That is deliberate: it
// makes scope identical between the two, so the only thing separating them at
// step 2 is that one of them already decided step 1.
func TestNotifiable_PriorApproverExcluded(t *testing.T) {
	f := newNotifiableFixture(t, "NPA")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.prior.maker@test.local")
	decided := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "notif.prior.decided@test.local")
	fresh := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "notif.prior.fresh@test.local")

	// 150M spans the three-step chain: office -> wilayah -> pusat.
	req := f.submit(t, maker, "Laptop 150M", "150000000")

	// Both are eligible at step 1 (office tier at Cabang, inside the Wilayah subtree).
	step1 := notifiable(t, f, req.ID)
	require.Contains(t, step1, decided)
	require.Contains(t, step1, fresh)

	// One of them decides step 1; the chain advances to the wilayah step.
	caller := buildCaller(decided, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	require.Equal(t, int32(2), out.CurrentStep)

	step2 := notifiable(t, f, req.ID)
	assert.NotContains(t, step2, decided,
		"an approver who already decided an earlier step must not be asked again")
	assert.Contains(t, step2, fresh,
		"control: same role, same office, same scope -- only the prior decision differs")
	assert.NotContains(t, step2, maker, "maker stays excluded at every step")
}

// TestNotifiable_OutOfScopeExcluded covers the data-scope filter. Cabang Beta's
// head has request.decide and is not the maker, so scope is the only thing that
// can exclude him -- and the control proves it does not exclude him from his own
// branch's request.
func TestNotifiable_OutOfScopeExcluded(t *testing.T) {
	f := newNotifiableFixture(t, "NOS")
	betaID := seedSiblingOffice(t, f.pool, f.tree)

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.scope.maker@test.local")
	alphaHead := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.scope.alpha@test.local")
	betaHead := seedUserAtOffice(t, f.pool, f.officeRoleID, betaID, "notif.scope.beta@test.local")

	alphaReq := f.submit(t, maker, "Router Alpha 5M", "5000000")

	got := notifiable(t, f, alphaReq.ID)
	assert.NotContains(t, got, betaHead,
		"a candidate outside the tier office scope must not be notified")
	assert.Contains(t, got, alphaHead, "the in-scope approver still is")

	// Control: the same betaHead IS notified for a request raised in his own
	// branch, so the exclusion above is scope and nothing else.
	betaReq, err := f.svc.Submit(context.Background(), approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "5000000",
		OfficeID: betaID,
		Payload:  []byte(`{}`),
		Maker:    maker,
	})
	require.NoError(t, err)

	betaGot := notifiable(t, f, betaReq.ID)
	assert.Contains(t, betaGot, betaHead, "control: in his own branch he is a valid approver")
	assert.NotContains(t, betaGot, alphaHead, "and Alpha's head is now the out-of-scope one")
}

// TestNotifiable_WithoutPermissionExcluded pins the permission gate: Staf is not
// granted request.decide, so no scope or SoD accident can put them on the list.
func TestNotifiable_WithoutPermissionExcluded(t *testing.T) {
	f := newNotifiableFixture(t, "NPX")
	stafRole := lookupRole(t, f.pool, "Staf")

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.perm.maker@test.local")
	// Same office as the eligible approver: only the missing permission differs.
	staf := seedUserAtOffice(t, f.pool, stafRole, f.tree.CabangID, "notif.perm.staf@test.local")
	head := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.perm.head@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")

	got := notifiable(t, f, req.ID)
	assert.NotContains(t, got, staf, "a user without request.decide is never notifiable")
	assert.Contains(t, got, head, "control: the list is not simply empty")
}

// TestNotifiable_InactiveAndDeletedExcluded checks the candidate query's own
// filters. Each assertion is bracketed by the same user being returned first,
// so a regression that empties the list cannot pass.
func TestNotifiable_InactiveAndDeletedExcluded(t *testing.T) {
	f := newNotifiableFixture(t, "NID")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.state.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.state.appr@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")

	require.Contains(t, notifiable(t, f, req.ID), approver, "baseline: active approver is notifiable")

	_, err := f.pool.Exec(ctx, `UPDATE identity.users SET status = 'inactive' WHERE id = $1`, approver)
	require.NoError(t, err)
	assert.NotContains(t, notifiable(t, f, req.ID), approver, "an inactive user must never be notified")

	_, err = f.pool.Exec(ctx, `UPDATE identity.users SET status = 'active' WHERE id = $1`, approver)
	require.NoError(t, err)
	require.Contains(t, notifiable(t, f, req.ID), approver, "reactivating restores eligibility")

	_, err = f.pool.Exec(ctx, `UPDATE identity.users SET deleted_at = now() WHERE id = $1`, approver)
	require.NoError(t, err)
	assert.NotContains(t, notifiable(t, f, req.ID), approver, "a soft-deleted user must never be notified")
}

// TestNotifiable_GlobalScopeIncluded covers the AllScope branch of callerFor:
// a Superadmin has no office placement at all, and global scope must still put
// them in range of a Cabang-tier step.
func TestNotifiable_GlobalScopeIncluded(t *testing.T) {
	f := newNotifiableFixture(t, "NGS")

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "notif.global.maker@test.local")
	// Deliberately unplaced (office_id NULL) -- global scope must not depend on placement.
	super := seedUser(t, f.pool, f.pusatRoleID, "notif.global.super@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")

	assert.Contains(t, notifiable(t, f, req.ID), super,
		"a global-scope approver is in range of every office")
}

// TestNotifiable_TierUnsatisfiable_ReturnsEmpty: when the chain demands a tier
// the office ancestry cannot supply, nobody is eligible -- and notifying nobody
// is the correct, safe answer rather than falling back to everyone.
func TestNotifiable_TierUnsatisfiable_ReturnsEmpty(t *testing.T) {
	f := newNotifiableFixture(t, "NTU")
	ctx := context.Background()

	// An orphan office with no pusat/wilayah ancestor: a wilayah-tier step
	// cannot resolve a tier office from it.
	var orphanID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Cabang Yatim', 'CBY') RETURNING id`,
		f.tree.CabangTypeID).Scan(&orphanID))

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, orphanID, "notif.tier.maker@test.local")
	seedUserAtOffice(t, f.pool, f.pusatRoleID, orphanID, "notif.tier.super@test.local")

	// 150M needs office -> wilayah -> pusat; the orphan has no such ancestors.
	req, err := f.svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "150000000",
		OfficeID: orphanID,
		Payload:  []byte(`{}`),
		Maker:    maker,
	})
	require.NoError(t, err)

	// Step 1 is office tier and resolves to the origin office, so it has approvers.
	require.NotEmpty(t, notifiable(t, f, req.ID), "the office-tier step resolves normally")

	// Force the request onto the wilayah step, which cannot resolve a tier office.
	_, err = f.pool.Exec(ctx, `UPDATE approval.requests SET current_step = 2 WHERE id = $1`, req.ID)
	require.NoError(t, err)

	assert.Empty(t, notifiable(t, f, req.ID),
		"an unsatisfiable tier notifies nobody rather than everybody")
}
