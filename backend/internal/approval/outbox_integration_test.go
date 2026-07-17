//go:build integration

package approval_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

// outboxFixture is the common wiring the enqueue tests share.
type outboxFixture struct {
	pool         *pgxpool.Pool
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

	f := outboxFixture{pool: pool}
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

	rows := outboxFor(t, f.pool, req.ID)
	require.Len(t, rows, 1, "final approve must enqueue exactly one event")
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

	rows := outboxFor(t, f.pool, req.ID)
	require.Len(t, rows, 1, "reject must enqueue exactly one event")
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

	okRows := outboxFor(t, f.pool, okReq.ID)
	require.Len(t, okRows, 1)
	noRows := outboxFor(t, f.pool, noReq.ID)
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

	assert.Empty(t, outboxFor(t, f.pool, req.ID),
		"a rolled-back business tx must leave no outbox row")

	// The business change rolled back too: the request is still pending.
	after, err := sqlc.New(f.pool).GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, after.Status)
}

// TestApproval_Outbox_ChainAdvance_DoesNotEnqueue pins the task boundary: an
// approve that only moves the chain forward is not a terminal decision, so it
// emits nothing here. The "next approver's turn" event is a separate concern.
func TestApproval_Outbox_ChainAdvance_DoesNotEnqueue(t *testing.T) {
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
	assert.Empty(t, outboxFor(t, f.pool, req.ID), "chain advance must not enqueue request_decided")

	caller2 := buildCaller(approver2, f.wilayahRole, false, []uuid.UUID{f.tree.WilayahID, f.tree.CabangID})
	out, err = f.svc.Decide(ctx, req.ID, caller2, true, nil)
	require.NoError(t, err)
	require.Equal(t, int32(3), out.CurrentStep)
	assert.Empty(t, outboxFor(t, f.pool, req.ID), "chain advance must not enqueue request_decided")

	// Only the final step is terminal, and it enqueues exactly one event.
	caller3 := buildCaller(approver3, f.pusatRoleID, true, nil)
	out, err = f.svc.Decide(ctx, req.ID, caller3, true, nil)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedRequestStatusApproved, out.Status)

	rows := outboxFor(t, f.pool, req.ID)
	require.Len(t, rows, 1, "a three-step chain must yield exactly one request_decided")
	assert.Equal(t, sqlc.SharedRequestStatusApproved, decodeDecided(t, rows[0].Payload).Status)
}
