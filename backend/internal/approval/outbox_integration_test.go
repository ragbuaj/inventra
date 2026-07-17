//go:build integration

package approval_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/storage"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// outboxRow is the subset of notification.outbox the enqueue tests assert on.
type outboxRow struct {
	EventType     string
	AggregateType string
	AggregateID   uuid.UUID
	Payload       []byte
}

// outboxFor returns every outbox row written for the given aggregate id, oldest first.
func outboxFor(t *testing.T, pool *pgxpool.Pool, aggregateID uuid.UUID) []outboxRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT event_type, aggregate_type, aggregate_id, payload
		 FROM notification.outbox
		 WHERE aggregate_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at`, aggregateID)
	require.NoError(t, err)
	defer rows.Close()

	var out []outboxRow
	for rows.Next() {
		var r outboxRow
		require.NoError(t, rows.Scan(&r.EventType, &r.AggregateType, &r.AggregateID, &r.Payload))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// outboxOfType narrows outboxFor to one event type. Since Submit and every
// chain advance now enqueue their own events, a test about one event type must
// say so rather than count every row for the aggregate.
func outboxOfType(t *testing.T, pool *pgxpool.Pool, aggregateID uuid.UUID, eventType string) []outboxRow {
	t.Helper()
	var out []outboxRow
	for _, r := range outboxFor(t, pool, aggregateID) {
		if r.EventType == eventType {
			out = append(out, r)
		}
	}
	return out
}

// outboxFixture is the common wiring the enqueue tests share.
type outboxFixture struct {
	pool         *pgxpool.Pool
	rdb          *redis.Client
	svc          *approval.Service
	assetSvc     *asset.Service
	tree         tieredOfficeTree
	catID        uuid.UUID
	officeRoleID uuid.UUID
	wilayahRole  uuid.UUID
	pusatRoleID  uuid.UUID
}

func newOutboxFixture(t *testing.T, categoryCode string) outboxFixture {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	resetAll(t, pool)

	f := outboxFixture{pool: pool, rdb: rdb}
	f.tree = seedTieredOfficeTree(t, pool)
	f.catID = seedCategory(t, pool, categoryCode)
	f.officeRoleID = lookupRole(t, pool, "Kepala Unit")
	f.wilayahRole = lookupRole(t, pool, "Kepala Kanwil")
	f.pusatRoleID = lookupRole(t, pool, "Superadmin")

	q := sqlc.New(pool)
	f.svc = approval.NewService(q, pool, authz.NewScopeService(q, rdb), rdb)
	f.assetSvc = asset.NewService(q, pool, storage.NewFake(), 0, "")
	f.svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, f.assetSvc.CreateExecutor())
	return f
}

func (f outboxFixture) submit(t *testing.T, maker uuid.UUID, name, amount string) sqlc.ApprovalRequest {
	t.Helper()
	catIDStr := f.catID.String()
	officeIDStr := f.tree.CabangID.String()
	payload, err := json.Marshal(asset.AssetCreatePayload{
		Name:       name,
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})
	require.NoError(t, err)

	req, err := f.svc.Submit(context.Background(), approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   amount,
		OfficeID: f.tree.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)
	return req
}

// decodeDecided unmarshals an outbox payload into the typed event.
func decodeDecided(t *testing.T, payload []byte) approval.RequestDecidedEvent {
	t.Helper()
	var ev approval.RequestDecidedEvent
	require.NoError(t, json.Unmarshal(payload, &ev))
	return ev
}

// TestApproval_Outbox_ApproveFinal_Enqueues verifies that approving the final
// step writes exactly one request_decided outbox row naming the maker as the
// recipient, in the same transaction as the business change.
func TestApproval_Outbox_ApproveFinal_Enqueues(t *testing.T) {
	f := newOutboxFixture(t, "OBA")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.approve.maker@test.local")
	approver := seedUser(t, f.pool, f.officeRoleID, "outbox.approve.appr@test.local")

	// 5M stays inside the single-step (office) tier.
	req := f.submit(t, maker, "Router 5M", "5000000")

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedRequestStatusApproved, out.Status)

	rows := outboxOfType(t, f.pool, req.ID, approval.EventRequestDecided)
	require.Len(t, rows, 1, "final approve must enqueue exactly one request_decided event")
	assert.Equal(t, "request_decided", rows[0].EventType)
	assert.Equal(t, "requests", rows[0].AggregateType)
	assert.Equal(t, req.ID, rows[0].AggregateID)

	ev := decodeDecided(t, rows[0].Payload)
	assert.Equal(t, req.ID, ev.RequestID)
	assert.Equal(t, sqlc.SharedRequestTypeAssetCreate, ev.RequestType)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, ev.Status)
	assert.Equal(t, maker, ev.MakerID, "recipient must be the maker (requests.requested_by_id)")
	require.NotNil(t, ev.DecidedByID)
	assert.Equal(t, approver, *ev.DecidedByID)

	// The business side-effect and the event are both present: same tx, both committed.
	_, total, err := f.assetSvc.List(ctx, asset.ListInput{AllScope: true, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

// TestApproval_Outbox_Reject_Enqueues verifies the reject branch enqueues exactly
// one event whose payload distinguishes the outcome from an approval.
func TestApproval_Outbox_Reject_Enqueues(t *testing.T) {
	f := newOutboxFixture(t, "OBR")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.reject.maker@test.local")
	approver := seedUser(t, f.pool, f.officeRoleID, "outbox.reject.appr@test.local")

	req := f.submit(t, maker, "Switch 5M", "5000000")

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	note := "tidak layak"
	out, err := f.svc.Decide(ctx, req.ID, caller, false, &note)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedRequestStatusRejected, out.Status)

	rows := outboxOfType(t, f.pool, req.ID, approval.EventRequestDecided)
	require.Len(t, rows, 1, "reject must enqueue exactly one request_decided event")
	assert.Equal(t, "request_decided", rows[0].EventType)
	assert.Equal(t, req.ID, rows[0].AggregateID)

	ev := decodeDecided(t, rows[0].Payload)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, ev.Status, "payload must distinguish reject from approve")
	assert.Equal(t, maker, ev.MakerID)
	require.NotNil(t, ev.DecidedByID)
	assert.Equal(t, approver, *ev.DecidedByID)

	// Rejection creates no asset, and the event still records the outcome.
	_, total, err := f.assetSvc.List(ctx, asset.ListInput{AllScope: true, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

// TestApproval_Outbox_RejectAndApprove_DifferPerRequest pins the two outcomes
// side by side: each request carries its own event and its own status, so the
// consumer can tell them apart without re-reading the request row.
func TestApproval_Outbox_RejectAndApprove_DifferPerRequest(t *testing.T) {
	f := newOutboxFixture(t, "OBD")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.both.maker@test.local")
	approver := seedUser(t, f.pool, f.officeRoleID, "outbox.both.appr@test.local")
	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})

	okReq := f.submit(t, maker, "Approved 5M", "5000000")
	noReq := f.submit(t, maker, "Rejected 5M", "5000000")

	_, err := f.svc.Decide(ctx, okReq.ID, caller, true, nil)
	require.NoError(t, err)
	_, err = f.svc.Decide(ctx, noReq.ID, caller, false, nil)
	require.NoError(t, err)

	okRows := outboxOfType(t, f.pool, okReq.ID, approval.EventRequestDecided)
	require.Len(t, okRows, 1)
	noRows := outboxOfType(t, f.pool, noReq.ID, approval.EventRequestDecided)
	require.Len(t, noRows, 1)

	assert.Equal(t, sqlc.SharedRequestStatusApproved, decodeDecided(t, okRows[0].Payload).Status)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, decodeDecided(t, noRows[0].Payload).Status)
}

// TestApproval_Outbox_RollbackLeavesNoRow is the core guarantee of the
// transactional outbox. The enqueue runs before the executor, so this request
// really does insert an outbox row inside the transaction; the executor then
// fails on a malformed brand_id and the rollback must take the event with it.
// A row surviving here would mean the event was written outside the tx.
func TestApproval_Outbox_RollbackLeavesNoRow(t *testing.T) {
	f := newOutboxFixture(t, "OBX")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.rollback.maker@test.local")
	approver := seedUser(t, f.pool, f.officeRoleID, "outbox.rollback.appr@test.local")

	badBrandID := "not-a-uuid"
	catIDStr := f.catID.String()
	officeIDStr := f.tree.CabangID.String()
	payload, err := json.Marshal(asset.AssetCreatePayload{
		Name:       "Bad Brand Asset",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
		BrandID:    &badBrandID,
	})
	require.NoError(t, err)

	req, err := f.svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "5000000",
		OfficeID: f.tree.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	_, err = f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, asset.ErrInvalidRef)

	assert.Empty(t, outboxOfType(t, f.pool, req.ID, approval.EventRequestDecided),
		"a rolled-back business tx must leave no outbox row")
	// Control: Submit's own tx committed, so its event is still there. The
	// rollback took the failed tx's event and nothing else.
	assert.Len(t, outboxOfType(t, f.pool, req.ID, approval.EventRequestSubmitted), 1)

	// The business change rolled back too: the request is still pending.
	after, err := sqlc.New(f.pool).GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, after.Status)
}

// TestApproval_Outbox_ChainAdvance_DoesNotEnqueueDecided keeps the two events
// apart: an approve that only moves the chain forward is not a terminal
// decision, so it emits chain_advanced and never request_decided. Only the
// final step is terminal.
func TestApproval_Outbox_ChainAdvance_DoesNotEnqueueDecided(t *testing.T) {
	f := newOutboxFixture(t, "OBC")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.chain.maker@test.local")
	approver1 := seedUser(t, f.pool, f.officeRoleID, "outbox.chain.appr1@test.local")
	approver2 := seedUser(t, f.pool, f.wilayahRole, "outbox.chain.appr2@test.local")
	approver3 := seedUser(t, f.pool, f.pusatRoleID, "outbox.chain.appr3@test.local")

	// 150M spans the full three-step chain (office, wilayah, pusat).
	req := f.submit(t, maker, "Laptop 150M", "150000000")

	caller1 := buildCaller(approver1, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	out, err := f.svc.Decide(ctx, req.ID, caller1, true, nil)
	require.NoError(t, err)
	require.Equal(t, int32(2), out.CurrentStep)
	assert.Empty(t, outboxOfType(t, f.pool, req.ID, approval.EventRequestDecided),
		"chain advance must not enqueue request_decided")

	caller2 := buildCaller(approver2, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	out, err = f.svc.Decide(ctx, req.ID, caller2, true, nil)
	require.NoError(t, err)
	require.Equal(t, int32(3), out.CurrentStep)
	assert.Empty(t, outboxOfType(t, f.pool, req.ID, approval.EventRequestDecided),
		"chain advance must not enqueue request_decided")

	// Only the final step is terminal, and it enqueues exactly one event.
	caller3 := buildCaller(approver3, f.pusatRoleID, true, nil)
	out, err = f.svc.Decide(ctx, req.ID, caller3, true, nil)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedRequestStatusApproved, out.Status)

	rows := outboxOfType(t, f.pool, req.ID, approval.EventRequestDecided)
	require.Len(t, rows, 1, "a three-step chain must yield exactly one request_decided")
	assert.Equal(t, sqlc.SharedRequestStatusApproved, decodeDecided(t, rows[0].Payload).Status)
}

// decodePending unmarshals an outbox payload into the typed pending event.
func decodePending(t *testing.T, payload []byte) approval.RequestPendingEvent {
	t.Helper()
	var ev approval.RequestPendingEvent
	require.NoError(t, json.Unmarshal(payload, &ev))
	return ev
}

// TestApproval_Outbox_Submit_EnqueuesPending: submitting must announce that the
// first step is waiting, in the same transaction that creates the request.
func TestApproval_Outbox_Submit_EnqueuesPending(t *testing.T) {
	f := newOutboxFixture(t, "OBS")

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.submit.maker@test.local")
	req := f.submit(t, maker, "Router 5M", "5000000")

	rows := outboxOfType(t, f.pool, req.ID, approval.EventRequestSubmitted)
	require.Len(t, rows, 1, "submit must enqueue exactly one request_submitted event")
	assert.Equal(t, "requests", rows[0].AggregateType)
	assert.Equal(t, req.ID, rows[0].AggregateID)

	ev := decodePending(t, rows[0].Payload)
	assert.Equal(t, req.ID, ev.RequestID)
	assert.Equal(t, sqlc.SharedRequestTypeAssetCreate, ev.RequestType)
	assert.Equal(t, int32(1), ev.Step, "submit announces the first step")

	// The payload deliberately names no recipient: the consumer resolves them.
	assert.NotContains(t, string(rows[0].Payload), "maker_id")
}

// blockOutboxInserts installs a trigger that makes every insert into
// notification.outbox fail, and removes it when the test ends. It is the only
// way to force a failure AFTER the enqueue statement has run inside the
// business transaction, which is exactly what the transactional-outbox claim
// rests on: with the enqueue outside the tx, the business change below would
// survive.
func blockOutboxInserts(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION notification.test_block_outbox() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'outbox insert blocked by test'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER trg_test_block_outbox BEFORE INSERT ON notification.outbox
		FOR EACH ROW EXECUTE FUNCTION notification.test_block_outbox();`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(),
			`DROP TRIGGER IF EXISTS trg_test_block_outbox ON notification.outbox;
			 DROP FUNCTION IF EXISTS notification.test_block_outbox();`)
		require.NoError(t, err)
	})
}

// TestApproval_Outbox_SubmitRollback_LeavesNoRow: the enqueue shares Submit's
// transaction, so a failing enqueue rolls the whole submit back -- no event and
// no request. This is the documented trade: an outbox insert can only fail when
// the database is unavailable, in which case the business write fails anyway.
func TestApproval_Outbox_SubmitRollback_LeavesNoRow(t *testing.T) {
	f := newOutboxFixture(t, "OBSR")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.submit.rb@test.local")
	blockOutboxInserts(t, f.pool)

	catIDStr := f.catID.String()
	officeIDStr := f.tree.CabangID.String()
	payload, err := json.Marshal(asset.AssetCreatePayload{
		Name:       "Rolled Back",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})
	require.NoError(t, err)

	_, err = f.svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "5000000",
		OfficeID: f.tree.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.Error(t, err, "a failed enqueue must fail the submit")

	var events int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM notification.outbox`).Scan(&events))
	assert.Zero(t, events, "a rolled-back business tx must leave no outbox row")

	// The load-bearing half: the request went with it. A row here would prove
	// the enqueue was not in the same transaction.
	var requests int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM approval.requests WHERE requested_by_id = $1`, maker).Scan(&requests))
	assert.Zero(t, requests, "the business change must roll back with the event")
}

// TestApproval_Outbox_ChainAdvanceRollback_LeavesNoRow: same guarantee on the
// advance branch -- a failing enqueue must undo the advance itself.
func TestApproval_Outbox_ChainAdvanceRollback_LeavesNoRow(t *testing.T) {
	f := newOutboxFixture(t, "OBAR")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.advrb.maker@test.local")
	approver := seedUser(t, f.pool, f.officeRoleID, "outbox.advrb.appr@test.local")
	// 150M spans three steps, so approving step 1 advances rather than decides.
	req := f.submit(t, maker, "Laptop 150M", "150000000")

	blockOutboxInserts(t, f.pool)

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	_, err := f.svc.Decide(ctx, req.ID, caller, true, nil)
	require.Error(t, err, "a failed enqueue must fail the decide")

	assert.Empty(t, outboxOfType(t, f.pool, req.ID, approval.EventChainAdvanced),
		"a rolled-back business tx must leave no outbox row")

	// The advance itself rolled back: still waiting on step 1, undecided.
	after, err := sqlc.New(f.pool).GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), after.CurrentStep, "the advance must roll back with the event")
	assert.Equal(t, sqlc.SharedRequestStatusPending, after.Status)
}

// TestApproval_Outbox_ChainAdvance_EnqueuesPending: each advance must announce
// the step that is NOW waiting, not the one just decided -- getting that off by
// one would notify the wrong tier and mis-key the dedup key.
func TestApproval_Outbox_ChainAdvance_EnqueuesPending(t *testing.T) {
	f := newOutboxFixture(t, "OBCP")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.adv.maker@test.local")
	approver1 := seedUser(t, f.pool, f.officeRoleID, "outbox.adv.appr1@test.local")
	approver2 := seedUser(t, f.pool, f.wilayahRole, "outbox.adv.appr2@test.local")
	approver3 := seedUser(t, f.pool, f.pusatRoleID, "outbox.adv.appr3@test.local")

	// 150M spans the full three-step chain (office, wilayah, pusat).
	req := f.submit(t, maker, "Laptop 150M", "150000000")
	require.Len(t, outboxOfType(t, f.pool, req.ID, approval.EventChainAdvanced), 0,
		"nothing has advanced yet")

	caller1 := buildCaller(approver1, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	_, err := f.svc.Decide(ctx, req.ID, caller1, true, nil)
	require.NoError(t, err)

	rows := outboxOfType(t, f.pool, req.ID, approval.EventChainAdvanced)
	require.Len(t, rows, 1)
	assert.Equal(t, int32(2), decodePending(t, rows[0].Payload).Step,
		"the event must announce the step now waiting, not the one just decided")

	caller2 := buildCaller(approver2, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	_, err = f.svc.Decide(ctx, req.ID, caller2, true, nil)
	require.NoError(t, err)

	rows = outboxOfType(t, f.pool, req.ID, approval.EventChainAdvanced)
	require.Len(t, rows, 2)
	assert.Equal(t, int32(3), decodePending(t, rows[1].Payload).Step)

	// The final step is terminal: it decides, it does not advance.
	caller3 := buildCaller(approver3, f.pusatRoleID, true, nil)
	_, err = f.svc.Decide(ctx, req.ID, caller3, true, nil)
	require.NoError(t, err)
	assert.Len(t, outboxOfType(t, f.pool, req.ID, approval.EventChainAdvanced), 2,
		"the final approve is a decision, not an advance")
}

// TestApproval_Outbox_Reject_DoesNotEnqueuePending: a rejected request has no
// next step, so nobody is asked to act on it.
func TestApproval_Outbox_Reject_DoesNotEnqueuePending(t *testing.T) {
	f := newOutboxFixture(t, "OBRP")
	ctx := context.Background()

	maker := seedUser(t, f.pool, f.officeRoleID, "outbox.rejp.maker@test.local")
	approver := seedUser(t, f.pool, f.officeRoleID, "outbox.rejp.appr@test.local")

	// 150M has further steps to advance to -- rejecting must still stop the chain.
	req := f.submit(t, maker, "Laptop 150M", "150000000")

	caller := buildCaller(approver, f.officeRoleID, false, []uuid.UUID{f.tree.CabangID})
	_, err := f.svc.Decide(ctx, req.ID, caller, false, nil)
	require.NoError(t, err)

	assert.Empty(t, outboxOfType(t, f.pool, req.ID, approval.EventChainAdvanced),
		"a rejected request must not announce a next step")
}
