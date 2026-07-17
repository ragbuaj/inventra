//go:build integration

// End-to-end tests for the auto-resolve of stale approval_pending notifications:
// once a step's turn has passed -- the chain advanced, or the request was
// rejected, approved at the final step, or cancelled -- that step's "waiting for
// you" is soft-deleted, because clicking it would take the approver to a request
// that is no longer theirs to decide.
//
// The notifications are born through the real pipeline (relay + consumer) rather
// than inserted by hand wherever possible, so what is being cleared is the same
// row a real approver would have seen.
package approval_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// dedupKeysAllStates lists a user's notification dedup keys including the
// soft-deleted ones, with a flag per row. The feed helper hides deleted rows by
// design, so proving a row was cleared (rather than never written) needs this.
func dedupKeysAllStates(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) map[string]bool {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT dedup_key, deleted_at IS NOT NULL
		 FROM notification.notifications
		 WHERE user_id = $1 AND dedup_key IS NOT NULL`, userID)
	require.NoError(t, err)
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var key string
		var deleted bool
		require.NoError(t, rows.Scan(&key, &deleted))
		out[key] = deleted
	}
	require.NoError(t, rows.Err())
	return out
}

// insertNotification writes a notification row directly. Used only for the keys
// the real pipeline cannot produce in a test -- a step number beyond the three
// tiers a real chain has.
func insertNotification(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, dedupKey string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO notification.notifications (user_id, type, params, entity_type, entity_id, dedup_key)
		 VALUES ($1, 'approval_pending', '{}'::jsonb, 'requests', $2, $3)`,
		userID, uuid.New(), dedupKey)
	require.NoError(t, err)
}

// submitThreeStep submits a request whose amount spans the full
// office -> wilayah -> pusat chain, so there is a step to advance past.
func (f outboxFixture) submitThreeStep(t *testing.T, maker uuid.UUID) sqlc.ApprovalRequest {
	t.Helper()
	return f.submit(t, maker, "Laptop 150M", "150000000")
}

// Branch 1 of 4 -- chain advance. The approver who never got round to acting on
// step 1 must not be left holding it: their turn passed when the chain moved on.
func TestApproval_Stale_ChainAdvance_clears_passed_step(t *testing.T) {
	f := newOutboxFixture(t, "SCA")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.adv.maker@test.local")
	decider := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "stale.adv.decider@test.local")
	fresh := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "stale.adv.fresh@test.local")

	req := f.submitThreeStep(t, maker)
	f.drainPipeline(t)

	step1 := "request:" + req.ID.String() + ":step:1"
	require.Equal(t, []string{step1}, f.pendingKeys(t, fresh),
		"precondition: the approver holds step 1 before the chain moves")

	caller := buildCaller(decider, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	require.Equal(t, int32(2), out.CurrentStep)

	// Cleared before the consumer for step 2 ever runs: the clear rides the
	// business transaction, not the fan-out.
	assert.NotContains(t, f.pendingKeys(t, fresh), step1,
		"a passed step must leave the feed of an approver who never acted on it")
	assert.True(t, dedupKeysAllStates(t, f.pool, fresh)[step1],
		"the row must be soft-deleted, not hard-deleted or merely absent")
	assert.True(t, dedupKeysAllStates(t, f.pool, decider)[step1],
		"the step is cleared for every recipient, not just the one who acted")
}

// THE TRAP. dedup_key is 'request:<id>:step:<n>', so a LIKE 'request:<id>:step:1%'
// would also sweep step 10, 11, 12... Clearing step 1 must use an exact match.
//
// A real chain has three tiers, so step 10 cannot be reached by submitting; the
// step-10 row is seeded directly. What is under test is the clear, and it cannot
// tell a seeded key from a fanned-out one -- both are just dedup_key text.
func TestApproval_Stale_ChainAdvance_clearing_step1_spares_step10(t *testing.T) {
	f := newOutboxFixture(t, "STR")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.trap.maker@test.local")
	decider := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "stale.trap.decider@test.local")
	fresh := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "stale.trap.fresh@test.local")

	req := f.submitThreeStep(t, maker)
	f.drainPipeline(t)

	step1 := "request:" + req.ID.String() + ":step:1"
	step10 := "request:" + req.ID.String() + ":step:10"
	step11 := "request:" + req.ID.String() + ":step:11"
	insertNotification(t, f.pool, fresh, step10)
	insertNotification(t, f.pool, fresh, step11)

	require.Equal(t, []string{step1}, pendingKeysNamed(t, f, fresh, step1),
		"precondition: step 1 is really there to be cleared")

	// Advance past step 1 only.
	caller := buildCaller(decider, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	_, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)

	states := dedupKeysAllStates(t, f.pool, fresh)
	assert.True(t, states[step1], "step 1 is the step that passed: it must be cleared")
	assert.False(t, states[step10],
		"step 10 must survive: 'step:1' is a prefix of 'step:10', so a LIKE clear would wrongly sweep it")
	assert.False(t, states[step11], "step 11 must survive for the same reason")

	// The surviving rows are still in the live feed, not just undeleted.
	live := f.pendingKeys(t, fresh)
	assert.Contains(t, live, step10)
	assert.Contains(t, live, step11)
	assert.NotContains(t, live, step1)
}

// pendingKeysNamed narrows a user's live pending keys to the one asked about, so
// a precondition can assert on a single key without depending on how many other
// notifications the fixture happens to have produced.
func pendingKeysNamed(t *testing.T, f outboxFixture, userID uuid.UUID, key string) []string {
	t.Helper()
	var out []string
	for _, k := range f.pendingKeys(t, userID) {
		if k == key {
			out = append(out, k)
		}
	}
	return out
}

// Branch 2 of 4 -- rejected. Terminal: every step goes, and the maker's
// approval_decided must survive, since telling the maker is the whole point.
func TestApproval_Stale_Reject_clears_all_steps_but_spares_decided(t *testing.T) {
	f := newOutboxFixture(t, "SRJ")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.rej.maker@test.local")
	decider := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "stale.rej.decider@test.local")
	fresh := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "stale.rej.fresh@test.local")

	req := f.submitThreeStep(t, maker)
	f.drainPipeline(t)

	step1 := "request:" + req.ID.String() + ":step:1"
	// A later step's notification, as if the chain had already been walking.
	step2 := "request:" + req.ID.String() + ":step:2"
	insertNotification(t, f.pool, fresh, step2)
	require.NotEmpty(t, f.pendingKeys(t, fresh))

	caller := buildCaller(decider, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller, false, nil)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedRequestStatusRejected, out.Status)

	states := dedupKeysAllStates(t, f.pool, fresh)
	assert.True(t, states[step1], "rejection is terminal: step 1 must go")
	assert.True(t, states[step2], "rejection is terminal: every later step must go too")
	assert.Empty(t, f.pendingKeys(t, fresh), "no step of a rejected request may sit in a feed")
	assert.Empty(t, f.pendingKeys(t, decider))

	// The decided notification is produced by the same transaction's outbox row.
	f.drainPipeline(t)
	decidedKey := "request:" + req.ID.String() + ":decided"
	assert.False(t, dedupKeysAllStates(t, f.pool, maker)[decidedKey],
		"the maker's 'your request was rejected' must survive the clear")
	makerFeed := f.feed(t, maker)
	require.Len(t, makerFeed, 1)
	assert.Equal(t, sqlc.SharedNotificationTypeApprovalDecided, makerFeed[0].Type)
}

// Branch 3 of 4 -- approved at the final step. Terminal in the same way, and the
// maker's approval_decided must likewise survive.
func TestApproval_Stale_ApproveFinal_clears_all_steps_but_spares_decided(t *testing.T) {
	f := newOutboxFixture(t, "SAF")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.fin.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.fin.appr@test.local")
	other := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.fin.other@test.local")

	// 5M is a single-step chain, so this one decision IS the final step.
	req := f.submit(t, maker, "Router 5M", "5000000")
	f.drainPipeline(t)

	step1 := "request:" + req.ID.String() + ":step:1"
	require.Equal(t, []string{step1}, f.pendingKeys(t, other),
		"precondition: a second eligible approver is holding step 1")

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedRequestStatusApproved, out.Status)

	assert.True(t, dedupKeysAllStates(t, f.pool, other)[step1],
		"an approved request is decided: the other approver's turn is over")
	assert.Empty(t, f.pendingKeys(t, other))
	assert.Empty(t, f.pendingKeys(t, approver))

	f.drainPipeline(t)
	decidedKey := "request:" + req.ID.String() + ":decided"
	assert.False(t, dedupKeysAllStates(t, f.pool, maker)[decidedKey],
		"the maker's 'your request was approved' must survive the clear")
	makerFeed := f.feed(t, maker)
	require.Len(t, makerFeed, 1)
	assert.Equal(t, sqlc.SharedNotificationTypeApprovalDecided, makerFeed[0].Type)
}

// Branch 4 of 4 -- cancelled by the maker. The approvers were already told; the
// request is now un-decidable, so the ask must be withdrawn.
func TestApproval_Stale_Cancel_clears_all_steps(t *testing.T) {
	f := newOutboxFixture(t, "SCN")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.can.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.can.appr@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")
	f.drainPipeline(t)

	step1 := "request:" + req.ID.String() + ":step:1"
	require.Equal(t, []string{step1}, f.pendingKeys(t, approver),
		"precondition: the approver was told before the maker changed their mind")

	_, err := f.svc.Cancel(ctx, req.ID, maker)
	require.NoError(t, err)

	assert.True(t, dedupKeysAllStates(t, f.pool, approver)[step1],
		"a cancelled request must not keep asking for a decision")
	assert.Empty(t, f.pendingKeys(t, approver))
}

// A cancel that fails must not clear anything: the request is still pending and
// its approvers must keep their ask. This is what the shared transaction buys.
func TestApproval_Stale_Cancel_by_non_maker_clears_nothing(t *testing.T) {
	f := newOutboxFixture(t, "SCF")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.cnf.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.cnf.appr@test.local")
	stranger := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.cnf.stranger@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")
	f.drainPipeline(t)
	step1 := "request:" + req.ID.String() + ":step:1"
	require.Equal(t, []string{step1}, f.pendingKeys(t, approver))

	// Not the maker: CancelRequest matches no row.
	_, err := f.svc.Cancel(ctx, req.ID, stranger)
	require.Error(t, err)

	assert.Equal(t, []string{step1}, f.pendingKeys(t, approver),
		"a rejected cancel must leave the pending ask standing")
}

// The audit log is a separate, append-only record and the clear must not touch
// it: the reasoning for soft-deleting is UX, and nothing evidentiary is lost
// because the audit trail keeps every chain step (FR-6.6). audit.audit_logs has
// no deleted_at at all, so this also pins that the clear stays scoped to the
// notifications table rather than growing into a general-purpose sweep.
func TestApproval_Stale_Clear_leaves_audit_log_untouched(t *testing.T) {
	f := newOutboxFixture(t, "SAU")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.aud.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.aud.appr@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")
	f.drainPipeline(t)

	// Stand in for the audit row the handler records on submit.
	var auditID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO audit.audit_logs (actor_id, action, entity_type, entity_id, changes)
		 VALUES ($1, 'create', 'requests', $2, '{"step": 1}'::jsonb) RETURNING id`,
		maker, req.ID).Scan(&auditID))

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	_, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)

	// The notification went...
	assert.Empty(t, f.pendingKeys(t, approver), "control: the clear really ran")

	// ...and the audit row is still there, byte for byte.
	var changes []byte
	var action, entityType string
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT action, entity_type, changes FROM audit.audit_logs WHERE id = $1`,
		auditID).Scan(&action, &entityType, &changes))
	assert.Equal(t, "create", action)
	assert.Equal(t, "requests", entityType)
	assert.JSONEq(t, `{"step": 1}`, string(changes),
		"the audit trail records every chain step and must outlive the feed")
}

// The interaction with the Task 9 guard, which is what makes the clear stick:
// uq_notif_dedup is partial on deleted_at IS NULL, so a soft-deleted row does
// NOT block reinsertion. Only ApproversForStep returning ErrStepPassed stops a
// redelivered event from resurrecting what was just cleared.
func TestApproval_Stale_RedeliveredEvent_does_not_resurrect_cleared_notification(t *testing.T) {
	f := newOutboxFixture(t, "SRR")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.res.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.res.appr@test.local")
	other := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "stale.res.other@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")
	f.drainPipeline(t)
	step1 := "request:" + req.ID.String() + ":step:1"
	require.Equal(t, []string{step1}, f.pendingKeys(t, other))

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	_, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	require.Empty(t, f.pendingKeys(t, other), "precondition: the step was cleared")

	// Redeliver the original request_submitted event, exactly as a crash between
	// the DB commit and the XACK would.
	republishOutbox(t, f.pool)
	f.drainPipeline(t)

	assert.Empty(t, f.pendingKeys(t, other),
		"a redelivered stale event must not resurrect a cleared notification")
	assert.Empty(t, f.pendingKeys(t, approver))
	assert.True(t, dedupKeysAllStates(t, f.pool, other)[step1],
		"the cleared row must still be the soft-deleted one, not a fresh insert")
}
