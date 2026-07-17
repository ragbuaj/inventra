//go:build integration

// End-to-end tests for the approval_pending notification: a real Submit or
// Decide through approval.Service, the real relay, the real consumer resolving
// recipients through the real eligibility rules, and the rows that land in the
// approvers' feeds. Nothing is mocked, so the wire contract between producer
// and consumer -- and the promise that only eligible approvers are told -- is
// actually exercised rather than asserted twice from both sides.
//
// This lives in package approval_test -- an external test package -- so it may
// import notification even though notification imports approval. Production
// code in approval must never import notification; the enqueue is a generated
// sqlc call on qtx, so it does not need to.
package approval_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/notification"
)

// drainPipeline runs the relay then the consumer once each, moving every
// pending outbox row all the way into the notifications table. Both expose an
// exported Tick, so the test is deterministic instead of racing the poll loops.
// The consumer resolves recipients through the real approval.Service.
func (f outboxFixture) drainPipeline(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(f.pool)
	relay := notification.NewRelay(q, f.pool, f.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)
	// The resolver is the real approval.Service: eligibility is decided by the
	// same predicate that guards a real decision, not a stub of it.
	_, err = notification.NewConsumer(q, f.rdb, f.svc, "pending-e2e", time.Second, time.Millisecond).Tick(ctx)
	require.NoError(t, err)
}

// feed reads a user's notifications straight from the table.
func (f outboxFixture) feed(t *testing.T, userID uuid.UUID) []sqlc.NotificationNotification {
	t.Helper()
	rows, err := sqlc.New(f.pool).ListNotifications(context.Background(), sqlc.ListNotificationsParams{
		UserID: userID, Lim: 100, Off: 0,
	})
	require.NoError(t, err)
	return rows
}

// pendingKeys lists a user's approval_pending dedup keys.
func (f outboxFixture) pendingKeys(t *testing.T, userID uuid.UUID) []string {
	t.Helper()
	var out []string
	for _, r := range f.feed(t, userID) {
		if r.Type == sqlc.SharedNotificationTypeApprovalPending && r.DedupKey != nil {
			out = append(out, *r.DedupKey)
		}
	}
	return out
}

// The acceptance criterion, proven through the whole pipeline: submitting a
// request tells the approvers whose turn it is -- and never the maker. The
// control is what makes the exclusion real: the very same run notifies someone
// else, so an empty feed for the maker cannot pass by accident.
func TestApproval_Submit_notifies_approvers_not_maker_end_to_end(t *testing.T) {
	f := newOutboxFixture(t, "PNE")

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.e2e.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.e2e.appr@test.local")

	// 5M stays inside the single-step (office tier) chain.
	req := f.submit(t, maker, "Router 5M", "5000000")
	f.drainPipeline(t)

	rows := f.feed(t, approver)
	require.Len(t, rows, 1, "an eligible approver must be told it is their turn")
	assert.Equal(t, sqlc.SharedNotificationTypeApprovalPending, rows[0].Type)
	require.NotNil(t, rows[0].EntityType)
	assert.Equal(t, "requests", *rows[0].EntityType)
	require.NotNil(t, rows[0].EntityID)
	assert.Equal(t, req.ID, *rows[0].EntityID)
	require.NotNil(t, rows[0].DedupKey)
	assert.Equal(t, "request:"+req.ID.String()+":step:1", *rows[0].DedupKey)
	assert.JSONEq(t, `{"request_type":"asset_create","step":"1"}`, string(rows[0].Params))

	assert.Empty(t, f.feed(t, maker),
		"the maker must never be asked to approve their own request")
}

// Chain advance is the other half: approving a non-final step must tell the
// NEXT step's approvers, keyed to the new step.
func TestApproval_ChainAdvance_notifies_new_step_end_to_end(t *testing.T) {
	f := newOutboxFixture(t, "PNA")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.adv.maker@test.local")
	// Both are Kepala Kanwil at Wilayah, so scope covers the step-1 (Cabang) and
	// step-2 (Wilayah) tier offices alike: the only thing separating them at
	// step 2 is that one of them already decided step 1.
	decider := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "pending.adv.decider@test.local")
	fresh := seedUserAtOffice(t, f.pool, f.wilayahRole, f.tree.WilayahID, "pending.adv.fresh@test.local")

	// 150M spans the three-step chain: office -> wilayah -> pusat.
	req := f.submit(t, maker, "Laptop 150M", "150000000")
	f.drainPipeline(t)

	// Step 1: both are eligible, so both are told.
	assert.Equal(t, []string{"request:" + req.ID.String() + ":step:1"}, f.pendingKeys(t, decider))
	assert.Equal(t, []string{"request:" + req.ID.String() + ":step:1"}, f.pendingKeys(t, fresh))

	caller := buildCaller(decider, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	require.Equal(t, int32(2), out.CurrentStep)
	f.drainPipeline(t)

	// Only the new step remains: step 1's turn has passed, so the auto-resolve
	// in the advance transaction cleared it out of every feed.
	assert.Equal(t, []string{"request:" + req.ID.String() + ":step:2"},
		f.pendingKeys(t, fresh), "the new step must reach an approver who has not decided yet")

	// The SoD rules survive the trip through the pipeline: the approver who
	// already decided step 1 is not asked again, and the maker is never asked.
	assert.Empty(t, f.pendingKeys(t, decider),
		"a prior approver must not be notified about the next step, and their passed step is cleared")
	assert.Empty(t, f.pendingKeys(t, maker))
}

// A user outside the request's office scope must not even learn the request
// exists. The control: the same role inside the scope IS notified in the same
// run.
func TestApproval_Submit_does_not_notify_out_of_scope_user_end_to_end(t *testing.T) {
	f := newOutboxFixture(t, "PNS")

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.scope.maker@test.local")
	inScope := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.scope.in@test.local")
	sibling := seedSiblingOffice(t, f.pool, f.tree)
	outOfScope := seedUserAtOffice(t, f.pool, f.officeRoleID, sibling, "pending.scope.out@test.local")

	f.submit(t, maker, "Router 5M", "5000000")
	f.drainPipeline(t)

	assert.Len(t, f.feed(t, inScope), 1, "control: the same role in scope is notified")
	assert.Empty(t, f.feed(t, outOfScope),
		"a request in another branch must not surface in an out-of-scope feed")
}

// The stale-event case through the real pipeline: the request is decided before
// the consumer ever runs, so the event it finds is already obsolete. Nothing
// must be written -- the notification would be unactionable.
func TestApproval_StaleEvent_after_decision_notifies_nobody_end_to_end(t *testing.T) {
	f := newOutboxFixture(t, "PNX")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.stale.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.stale.appr@test.local")

	// Submit enqueues request_submitted, but the pipeline is NOT drained: the
	// event sits in the outbox while the request is decided out from under it.
	req := f.submit(t, maker, "Router 5M", "5000000")
	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedRequestStatusApproved, out.Status)

	f.drainPipeline(t)

	assert.Empty(t, f.pendingKeys(t, approver),
		"a step already decided must not produce a 'waiting for you' nobody can act on")
	// The decision notification still lands: the pipeline ran, so the empty
	// approval_pending feed above is a real skip, not a pipeline that did nothing.
	require.Len(t, f.feed(t, maker), 1)
	assert.Equal(t, sqlc.SharedNotificationTypeApprovalDecided, f.feed(t, maker)[0].Type)
}

// Cancelling has the same effect and its own branch: a cancelled request's
// pending event must resolve to nobody.
func TestApproval_StaleEvent_after_cancel_notifies_nobody_end_to_end(t *testing.T) {
	f := newOutboxFixture(t, "PNC")
	ctx := context.Background()

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.cancel.maker@test.local")
	approver := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.cancel.appr@test.local")

	req := f.submit(t, maker, "Router 5M", "5000000")
	_, err := f.svc.Cancel(ctx, req.ID, maker)
	require.NoError(t, err)

	f.drainPipeline(t)

	assert.Empty(t, f.feed(t, approver), "a cancelled request must notify nobody")
}

// Redelivery of a real submit event must not duplicate anyone's feed: the
// dedup key, not the ack, is what makes the at-least-once consumer safe.
func TestApproval_Submit_redelivery_yields_one_row_per_recipient(t *testing.T) {
	f := newOutboxFixture(t, "PNR")

	maker := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.dup.maker@test.local")
	a1 := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.dup.a1@test.local")
	a2 := seedUserAtOffice(t, f.pool, f.officeRoleID, f.tree.CabangID, "pending.dup.a2@test.local")

	f.submit(t, maker, "Router 5M", "5000000")

	// Drain twice: the second pass re-reads nothing new from the outbox, so
	// replay the same event through the consumer by re-publishing it.
	f.drainPipeline(t)
	republishOutbox(t, f.pool)
	f.drainPipeline(t)

	assert.Len(t, f.feed(t, a1), 1, "redelivery must not duplicate a recipient's feed")
	assert.Len(t, f.feed(t, a2), 1)
	assert.Empty(t, f.feed(t, maker))
}

// republishOutbox clears published_at so the relay claims and publishes every
// row a second time -- the same logical event delivered twice, exactly what a
// crash between the DB commit and the XACK produces.
func republishOutbox(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE notification.outbox SET published_at = NULL WHERE deleted_at IS NULL`)
	require.NoError(t, err)
}
