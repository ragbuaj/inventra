//go:build integration

// End-to-end tests for the transfer (mutasi) lifecycle notifications: a real
// Submit / approve / Ship / Receive / RejectReceive through the transfer +
// approval services, the real relay, and the real consumer resolving recipients
// through the real transfer.manage permission + "transfers" data scope. Nothing
// is mocked, so the wire contract between producer and consumer -- and the
// promise that each stage reaches exactly the right office -- is exercised
// rather than asserted twice from both sides.
package transfer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/notification"
	"github.com/ragbuaj/inventra/internal/testsupport"
	"github.com/ragbuaj/inventra/internal/transfer"
)

// notifPipeline wires the real relay + consumer over a dedicated Redis so a test
// can drain the notification outbox end-to-end. The consumer resolves transfer
// recipients through a real ScopeService, exactly as production does.
type notifPipeline struct {
	rdb   *redis.Client
	scope *authz.ScopeService
}

func newNotifPipeline(t *testing.T, h *harness) notifPipeline {
	t.Helper()
	rdb := testsupport.NewRedis(t)
	return notifPipeline{rdb: rdb, scope: authz.NewScopeService(h.q, rdb)}
}

// drain runs the relay once (outbox -> stream) then the consumer once
// (stream -> per-user notification rows), moving every pending event all the
// way into the feed. Both expose an exported Tick, so this is deterministic
// instead of racing the poll loops. Safe to call repeatedly across a test: the
// relay only republishes unpublished rows and the consumer group only reads new
// stream entries, so each drain flushes exactly the events since the last one.
func (p notifPipeline) drain(t *testing.T, h *harness) {
	t.Helper()
	ctx := context.Background()
	relay := notification.NewRelay(h.q, h.pool, p.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)
	_, err = notification.NewConsumer(h.q, p.rdb, h.apprSvc, p.scope, "transfer-notif-e2e", time.Second, time.Millisecond).Tick(ctx)
	require.NoError(t, err)
}

// notifTypesFor returns the set of non-deleted notification types in a user's feed.
func notifTypesFor(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) map[string]bool {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT type::text FROM notification.notifications WHERE user_id = $1 AND deleted_at IS NULL`, userID)
	require.NoError(t, err)
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var typ string
		require.NoError(t, rows.Scan(&typ))
		set[typ] = true
	}
	require.NoError(t, rows.Err())
	return set
}

// TestTransfer_Notifications_Lifecycle drives submit -> approve -> ship -> receive
// through the real services and the real notification pipeline, asserting each
// stage reaches exactly the right office:
//   - approved / received -> origin office (the shippers)
//   - in_transit          -> destination office (the receivers)
//
// and never leaks to an unrelated office. This is the gap the feature closes:
// before it, the destination office received no signal an asset was incoming.
func TestTransfer_Notifications_Lifecycle(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	pipe := newNotifPipeline(t, h)

	assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-NOTIF-1", "Laptop Notif", h.catID, h.fromOffice, "10000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.notif@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.notif@test.local")
	receiver := seedUser(t, h.pool, h.officeRoleID, h.toOffice, "receiver.notif@test.local")
	outsider := seedUser(t, h.pool, h.officeRoleID, h.otherOffice, "outsider.notif@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

	// submit + approve (10M < 50M -> single office-tier step)
	req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice, Reason: strptr("relok")})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
	require.NoError(t, err)

	// transfer_approved -> origin office (maker + checker), never destination/outsider.
	pipe.drain(t, h)
	assert.True(t, notifTypesFor(t, h.pool, maker)["transfer_approved"], "maker (origin) must get transfer_approved")
	assert.True(t, notifTypesFor(t, h.pool, checker)["transfer_approved"], "checker (origin) must get transfer_approved")
	assert.False(t, notifTypesFor(t, h.pool, receiver)["transfer_approved"], "destination must NOT get transfer_approved")
	assert.False(t, notifTypesFor(t, h.pool, outsider)["transfer_approved"], "unrelated office must NOT get transfer_approved")

	// ship -> transfer_in_transit -> destination office (receiver), never origin/outsider.
	_, err = h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
	require.NoError(t, err)
	pipe.drain(t, h)
	assert.True(t, notifTypesFor(t, h.pool, receiver)["transfer_in_transit"], "destination must get transfer_in_transit")
	assert.False(t, notifTypesFor(t, h.pool, maker)["transfer_in_transit"], "origin must NOT get transfer_in_transit")
	assert.False(t, notifTypesFor(t, h.pool, outsider)["transfer_in_transit"], "unrelated office must NOT get transfer_in_transit")

	// receive -> transfer_received -> origin office, never destination.
	_, _, err = h.tsvc.Receive(ctx, true, nil, receiver, row.ID, transfer.ReceiveInput{BastNo: strptr("BAST-N1")})
	require.NoError(t, err)
	pipe.drain(t, h)
	assert.True(t, notifTypesFor(t, h.pool, maker)["transfer_received"], "origin must get transfer_received")
	assert.False(t, notifTypesFor(t, h.pool, receiver)["transfer_received"], "destination must NOT get transfer_received")
}

// TestTransfer_Notifications_Returned verifies a declined shipment (RejectReceive)
// notifies the origin office it was returned, and does not reach the destination.
func TestTransfer_Notifications_Returned(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	pipe := newNotifPipeline(t, h)

	assetID := seedAssetWithCost(t, h.pool, "FROM-TRF-NOTIF-2", "Printer Notif", h.catID, h.fromOffice, "5000000")
	maker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "maker.notif.ret@test.local")
	checker := seedUser(t, h.pool, h.officeRoleID, h.fromOffice, "checker.notif.ret@test.local")
	receiver := seedUser(t, h.pool, h.officeRoleID, h.toOffice, "receiver.notif.ret@test.local")

	makerCaller := buildCaller(maker, h.officeRoleID, false, []uuid.UUID{h.fromOffice})
	checkerCaller := buildCaller(checker, h.officeRoleID, false, []uuid.UUID{h.fromOffice, h.toOffice})

	req, err := h.tsvc.Submit(ctx, makerCaller, transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
	require.NoError(t, err)
	final := approveThroughChain(t, h.apprSvc, req.ID, checkerCaller)
	require.Equal(t, sqlc.SharedRequestStatusApproved, final.Status)

	row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
	require.NoError(t, err)
	_, err = h.tsvc.Ship(ctx, true, nil, row.ID, transfer.ShipInput{})
	require.NoError(t, err)

	note := "kondisi tidak sesuai"
	_, err = h.tsvc.RejectReceive(ctx, true, nil, receiver, row.ID, &note)
	require.NoError(t, err)

	pipe.drain(t, h)
	assert.True(t, notifTypesFor(t, h.pool, maker)["transfer_returned"], "origin must get transfer_returned")
	assert.False(t, notifTypesFor(t, h.pool, receiver)["transfer_returned"], "destination must NOT get transfer_returned")
}
