//go:build integration

// Integration tests for the check-in outbox enqueue: the producer half of the
// asset_returned notification. These cover what only a real Postgres can prove
// -- that the event shares the business transaction, and that a rollback takes
// the event with it. The consumer half (event to notification row) is covered
// end-to-end in internal/notification.
package assignment_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/assignment"
)

// outboxRows reads every outbox row for one assignment aggregate.
func (h *harness) outboxRows(t *testing.T, assignmentID uuid.UUID) []assignment.AssignmentCheckinEvent {
	t.Helper()
	rows, err := h.pool.Query(context.Background(),
		`SELECT payload FROM notification.outbox
		 WHERE event_type = $1 AND aggregate_type = $2 AND aggregate_id = $3
		 ORDER BY created_at`,
		assignment.EventAssignmentCheckin, assignment.AggregateAssignments, assignmentID)
	require.NoError(t, err)
	defer rows.Close()

	var out []assignment.AssignmentCheckinEvent
	for rows.Next() {
		var raw []byte
		require.NoError(t, rows.Scan(&raw))
		var ev assignment.AssignmentCheckinEvent
		require.NoError(t, json.Unmarshal(raw, &ev))
		out = append(out, ev)
	}
	require.NoError(t, rows.Err())
	return out
}

// countCheckinOutbox reports how many check-in events exist in total, so a test
// can assert on "none anywhere", not merely "none for this aggregate".
func (h *harness) countCheckinOutbox(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notification.outbox WHERE event_type = $1`,
		assignment.EventAssignmentCheckin).Scan(&n))
	return n
}

// The core producer contract: a check-in by someone other than the checker-out
// enqueues exactly one self-contained event addressed to the checker-out.
func TestAssignment_Checkin_enqueues_outbox_event(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00020", "Laptop Outbox", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.outbox@test.local")
	other := h.seedManager(t, h.office, "other.outbox@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.outbox@test.local", "EMP-OB-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, other, assignment.CheckinInput{})
	require.NoError(t, err)

	evs := h.outboxRows(t, a.ID)
	require.Len(t, evs, 1)
	ev := evs[0]
	assert.Equal(t, a.ID, ev.AssignmentID)
	assert.Equal(t, assetID, ev.AssetID)
	// The recipient is the user who checked out, never the custodian employee.
	assert.Equal(t, manager, ev.AssignedByID)
	assert.Equal(t, other, ev.CheckedInByID)
	// The event must be self-contained: the consumer runs later and must not
	// have to re-read the asset for its i18n params.
	assert.Equal(t, "OFC-ASG-2026-00020", ev.AssetTag)
	assert.Equal(t, "Laptop Outbox", ev.AssetName)
}

// Self-notification suppression, at the producer: no recipient means no event.
func TestAssignment_Checkin_by_same_user_enqueues_nothing(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00021", "Printer Sendiri", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.self@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.self@test.local", "EMP-SF-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	// The same manager who checked out does the check-in.
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, manager, assignment.CheckinInput{})
	require.NoError(t, err)

	assert.Empty(t, h.outboxRows(t, a.ID))
	assert.Equal(t, 0, h.countCheckinOutbox(t), "no check-in event may exist anywhere")

	// Suppressing the event must not have disturbed the business change.
	assert.Equal(t, "available", string(h.getAssetStatus(t, assetID)))
}

// The transactional-outbox guarantee: a check-in that fails leaves no event
// behind. The failure is forced through the real business guard (a non-active
// assignment) rather than a stubbed error, so the tx boundary is genuinely
// exercised.
func TestAssignment_Checkin_failure_leaves_no_outbox_row(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00022", "Router Rollback", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.rollback@test.local")
	other := h.seedManager(t, h.office, "other.rollback@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.rollback@test.local", "EMP-RB-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	// First check-in succeeds and enqueues one event.
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, other, assignment.CheckinInput{})
	require.NoError(t, err)
	require.Len(t, h.outboxRows(t, a.ID), 1)

	// The second check-in is rejected before the tx opens: still exactly one
	// event, never two.
	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, other, assignment.CheckinInput{})
	require.ErrorIs(t, err, assignment.ErrNotActive)
	assert.Len(t, h.outboxRows(t, a.ID), 1)
}

// A check-in rejected by the scope guard must leave neither a business change
// nor an event: the enqueue cannot outlive a rolled-back / never-started tx.
func TestAssignment_Checkin_out_of_scope_leaves_no_outbox_row(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00023", "Scanner Scope", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.scope.ob@test.local")
	other := h.seedManager(t, h.sibling, "other.scope.ob@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.scope.ob@test.local", "EMP-SC-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.sibling}, a.ID, other, assignment.CheckinInput{})
	require.ErrorIs(t, err, assignment.ErrNotFound)

	assert.Empty(t, h.outboxRows(t, a.ID))
	assert.Equal(t, 0, h.countCheckinOutbox(t))
	assert.Equal(t, "assigned", string(h.getAssetStatus(t, assetID)), "the asset must be untouched")
}

// A check-in that also flags maintenance still notifies: the event is
// independent of which status the asset lands in.
func TestAssignment_Checkin_needs_maintenance_still_enqueues(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	assetID := seedAsset(t, h.pool, "OFC-ASG-2026-00024", "AC Rusak", h.catID, h.office, "available")
	manager := h.seedManager(t, h.office, "manager.maint.ob@test.local")
	other := h.seedManager(t, h.office, "other.maint.ob@test.local")
	_, empID := h.seedStaf(t, h.office, "staf.maint.ob@test.local", "EMP-MO-1")

	a, err := h.asvc.Checkout(ctx, false, []uuid.UUID{h.office}, manager, assignment.CheckoutInput{
		AssetID: assetID, EmployeeID: empID, CheckoutDate: "2026-07-06",
	})
	require.NoError(t, err)

	_, _, err = h.asvc.Checkin(ctx, false, []uuid.UUID{h.office}, a.ID, other, assignment.CheckinInput{NeedsMaintenance: true})
	require.NoError(t, err)

	evs := h.outboxRows(t, a.ID)
	require.Len(t, evs, 1)
	assert.Equal(t, manager, evs[0].AssignedByID)
	assert.Equal(t, "AC Rusak", evs[0].AssetName)
	assert.Equal(t, "under_maintenance", string(h.getAssetStatus(t, assetID)))
}
