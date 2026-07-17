//go:build integration

// End-to-end tests for the asset_returned notification: a real check-in through
// assignment.Service, the real relay, the real consumer, and the row that lands
// in the recipient's feed. Nothing here is mocked, so the wire contract between
// the producer (assignment) and the consumer (notification) is actually
// exercised rather than asserted twice from both sides.
//
// This lives in package assignment_test -- an external test package -- so it may
// import notification even though notification imports assignment. Production
// code in assignment must never import notification; the enqueue is a generated
// sqlc call on qtx, so it does not need to.
package assignment_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/assignment"
	"github.com/ragbuaj/inventra/internal/notification"
)

// drainPipeline runs the relay then the consumer once each, moving every
// pending outbox row all the way into the notifications table. Both expose an
// exported Tick, so the test is deterministic instead of racing the poll loops.
func (h *harness) drainPipeline(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)
	// The real approval.Service resolves approval_pending recipients: a borrow
	// goes through the approval chain, so this pipeline carries those events too
	// and a nil resolver would (correctly) refuse them.
	_, err = notification.NewConsumer(h.q, h.rdb, h.apprSvc, nil, "checkin-e2e", time.Second, time.Millisecond).Tick(ctx)
	require.NoError(t, err)
}

// feed reads a user's notifications straight from the table.
func (h *harness) feed(t *testing.T, userID uuid.UUID) []sqlc.NotificationNotification {
	t.Helper()
	rows, err := h.q.ListNotifications(context.Background(), sqlc.ListNotificationsParams{
		UserID: userID, Lim: 100, Off: 0,
	})
	require.NoError(t, err)
	return rows
}

// The acceptance criterion, proven through the whole pipeline: checking an asset
// back in notifies the user who checked it out.
func TestAssignment_Checkin_notifies_checkout_user_end_to_end(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00030", "Laptop Kembali", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.e2e@test.local")
	other := h.seedManager(t, h.office, "other.e2e@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.e2e@test.local", "EMP-E2-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, other, assignment.CheckinInput{})
	require.NoError(t, err)

	h.drainPipeline(t)

	rows := h.feed(t, manager)
	require.Len(t, rows, 1)
	got := rows[0]
	assert.Equal(t, manager, got.UserID)
	assert.Equal(t, sqlc.SharedNotificationTypeAssetReturned, got.Type)
	require.NotNil(t, got.EntityType)
	assert.Equal(t, "assets", *got.EntityType)
	require.NotNil(t, got.EntityID)
	assert.Equal(t, assetID, *got.EntityID)
	require.NotNil(t, got.DedupKey)
	assert.Equal(t, "assignment:"+a.ID.String()+":checkin", *got.DedupKey)
	assert.False(t, got.ReadAt.Valid, "a fresh notification must be unread")

	// Params carry i18n interpolation values, never rendered text.
	assert.JSONEq(t, `{"asset_tag":"OFC-ASG-2026-00030","asset_name":"Laptop Kembali"}`, string(got.Params))

	// The person who did the check-in is not a recipient.
	assert.Empty(t, h.feed(t, other))
}

// Self-notification: the person who checked out returning the asset themselves
// gets nothing. Paired with the test above, which proves the same pipeline DOES
// deliver when the actors differ -- so an empty feed here means suppression, not
// a pipeline that never worked.
func TestAssignment_Checkin_by_same_user_notifies_nobody_end_to_end(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00031", "Printer Sendiri E2E", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.self.e2e@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.self.e2e@test.local", "EMP-E2-2")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, manager, assignment.CheckinInput{})
	require.NoError(t, err)

	h.drainPipeline(t)

	assert.Empty(t, h.feed(t, manager), "checking in your own check-out must notify nobody")

	var total int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM notification.notifications`).Scan(&total))
	assert.Equal(t, 0, total, "no notification may exist for anyone")
}

// Two check-out/check-in cycles on the same asset are two distinct events: the
// dedup key is per assignment, so it must not collapse them into one.
func TestAssignment_Checkin_two_cycles_yield_two_notifications(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00032", "Monitor Siklus", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.cycles@test.local")
	other := h.seedManager(t, h.office, "other.cycles@test.local")
	_, emp1 := h.seedStaf(t, h.office, "staf.cycles1@test.local", "EMP-CY-1")
	_, emp2 := h.seedStaf(t, h.office, "staf.cycles2@test.local", "EMP-CY-2")

	a1, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: emp1, CheckoutDate: "2026-07-01",
	})
	require.NoError(t, err)
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a1.ID, other, assignment.CheckinInput{})
	require.NoError(t, err)

	a2, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: emp2, CheckoutDate: "2026-07-05",
	})
	require.NoError(t, err)
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a2.ID, other, assignment.CheckinInput{})
	require.NoError(t, err)

	h.drainPipeline(t)

	rows := h.feed(t, manager)
	require.Len(t, rows, 2)
	keys := []string{*rows[0].DedupKey, *rows[1].DedupKey}
	assert.ElementsMatch(t, []string{
		"assignment:" + a1.ID.String() + ":checkin",
		"assignment:" + a2.ID.String() + ":checkin",
	}, keys)
}

// A borrow approved through the approval engine checks the asset out in the
// approver's name, so the approver -- not the requesting Staf -- is the one
// notified when it comes back. Documents the consequence of keying on
// assigned_by_id.
func TestAssignment_Checkin_after_borrow_notifies_the_approver(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00033", "Proyektor Pinjam", h.catID, h.office, "available")
	stafID, _ := h.seedStaf(t, h.office, "staf.borrow.notif@test.local", "EMP-BN-1")
	approverID := h.seedManager(t, h.office, "approver.borrow.notif@test.local")

	req, err := h.asvc.SubmitBorrow(ctx, buildCaller(stafID, h.stafRl, false, []uuid.UUID{h.office}),
		assignment.BorrowInput{AssetID: assetID})
	require.NoError(t, err)

	_, err = h.apprSvc.Decide(ctx, req.ID,
		buildCaller(approverID, h.managerRl, false, []uuid.UUID{h.office}), true, nil)
	require.NoError(t, err)

	rows, err := h.asvc.ListByAsset(ctx, assetID, true, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	a := rows[0].AssignmentAssignment
	require.Equal(t, approverID, a.AssignedByID)

	// A third party checks it back in, so nothing is suppressed.
	returner := h.seedManager(t, h.office, "returner.borrow.notif@test.local")
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, returner, assignment.CheckinInput{})
	require.NoError(t, err)

	h.drainPipeline(t)

	rows2 := h.feed(t, approverID)
	require.Len(t, rows2, 1)
	assert.Equal(t, sqlc.SharedNotificationTypeAssetReturned, rows2[0].Type)

	// The Staf custodian is deliberately not notified of the return:
	// employee_id is not a user. The Staf's only notification is the
	// approval_decided one from their own borrow request.
	for _, n := range h.feed(t, stafID) {
		assert.NotEqual(t, sqlc.SharedNotificationTypeAssetReturned, n.Type,
			"the custodian must not receive an asset_returned notification")
	}
}
