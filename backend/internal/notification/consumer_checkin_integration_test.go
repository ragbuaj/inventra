//go:build integration

// Integration tests for the assignment_checkin -> asset_returned handler.
// Shares the harness of consumer_integration_test.go (same test package).
//
// The producer-side counterpart -- a real assignment.Service.Checkin driving
// this same pipeline, including self-notification suppression -- lives in
// internal/assignment/checkin_notification_integration_test.go, where the real
// office/asset/user fixtures are.
package notification_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/assignment"
	"github.com/ragbuaj/inventra/internal/notification"
)

// checkinEvent builds a well-formed check-in event for the given recipient. The
// producer suppresses self-notification, so an event reaching the consumer
// always has a checker-in distinct from the recipient.
func checkinEvent(assignedBy uuid.UUID) assignment.AssignmentCheckinEvent {
	return assignment.AssignmentCheckinEvent{
		AssignmentID:  uuid.New(),
		AssetID:       uuid.New(),
		AssetTag:      "OFC-IT-2026-00001",
		AssetName:     "Laptop Dinas",
		AssignedByID:  assignedBy,
		CheckedInByID: uuid.New(),
	}
}

// enqueueCheckin writes an assignment_checkin outbox row carrying the real
// event payload struct, so the test exercises the same wire contract assignment
// writes.
func (h *harness) enqueueCheckin(t *testing.T, ev assignment.AssignmentCheckinEvent) sqlc.NotificationOutbox {
	t.Helper()
	raw, err := json.Marshal(ev)
	require.NoError(t, err)
	row, err := h.q.EnqueueOutbox(context.Background(), sqlc.EnqueueOutboxParams{
		EventType:     assignment.EventAssignmentCheckin,
		AggregateType: assignment.AggregateAssignments,
		AggregateID:   ev.AssignmentID,
		Payload:       raw,
	})
	require.NoError(t, err)
	return row
}

// A check-in born through the whole pipeline: outbox row, real relay, stream,
// real consumer, table.
func TestConsumerCheckoutUserGetsAssetReturnedEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	recipient := h.seedUser(t, "checkout-user@e2e.local")
	ev := checkinEvent(recipient)
	h.enqueueCheckin(t, ev)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	published, err := relay.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, published)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	rows := h.notifications(t, recipient)
	require.Len(t, rows, 1)
	got := rows[0]
	assert.Equal(t, recipient, got.UserID)
	assert.Equal(t, sqlc.SharedNotificationTypeAssetReturned, got.Type)
	require.NotNil(t, got.EntityType)
	assert.Equal(t, "assets", *got.EntityType)
	require.NotNil(t, got.EntityID)
	assert.Equal(t, ev.AssetID, *got.EntityID)
	require.NotNil(t, got.DedupKey)
	assert.Equal(t, "assignment:"+ev.AssignmentID.String()+":checkin", *got.DedupKey)
	assert.False(t, got.ReadAt.Valid, "a fresh notification must be unread")

	// Params carry the i18n interpolation values, never rendered text.
	assert.JSONEq(t, `{"asset_tag":"OFC-IT-2026-00001","asset_name":"Laptop Dinas"}`, string(got.Params))

	assert.EqualValues(t, 0, h.pendingCount(t))
}

// At-least-once: a redelivered check-in must not duplicate the row.
func TestConsumerCheckinDeliveredTwiceYieldsOneNotification(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	recipient := h.seedUser(t, "checkout-dup@e2e.local")
	ev := checkinEvent(recipient)
	raw, err := json.Marshal(ev)
	require.NoError(t, err)

	// The same logical event twice: exactly what a crash between the DB commit
	// and the XACK produces.
	for i := 0; i < 2; i++ {
		require.NoError(t, h.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: notification.StreamKey,
			Values: map[string]any{
				notification.FieldOutboxID:      uuid.New().String(),
				notification.FieldEventType:     assignment.EventAssignmentCheckin,
				notification.FieldAggregateType: assignment.AggregateAssignments,
				notification.FieldAggregateID:   ev.AssignmentID.String(),
				notification.FieldPayload:       string(raw),
			},
		}).Err())
	}

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "both messages must be acked")

	assert.Len(t, h.notifications(t, recipient), 1, "the dedup key must collapse the redelivery")
}

// Two check-ins of different assignments are distinct events: the dedup key is
// per assignment and must not collapse them.
func TestConsumerTwoCheckinsOfDifferentAssignmentsAreDistinct(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	recipient := h.seedUser(t, "checkout-two@e2e.local")
	h.enqueueCheckin(t, checkinEvent(recipient))
	h.enqueueCheckin(t, checkinEvent(recipient))

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)
	_, err = h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)

	assert.Len(t, h.notifications(t, recipient), 2)
}

// A check-in payload without a recipient cannot be fixed by retrying: it must
// be acked, not left to loop in the PEL forever.
func TestConsumerCheckinWithoutRecipientIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	h.enqueue(t, assignment.EventAssignmentCheckin, map[string]any{
		"assignment_id": uuid.New().String(),
		"asset_id":      uuid.New().String(),
		"asset_tag":     "OFC-IT-2026-00002",
	})

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// A check-in payload that is not JSON at all is equally unfixable by retry.
func TestConsumerUndecodableCheckinPayloadIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	require.NoError(t, h.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: notification.StreamKey,
		Values: map[string]any{
			notification.FieldOutboxID:      uuid.New().String(),
			notification.FieldEventType:     assignment.EventAssignmentCheckin,
			notification.FieldAggregateType: assignment.AggregateAssignments,
			notification.FieldAggregateID:   uuid.New().String(),
			notification.FieldPayload:       "{not json",
		},
	}).Err())

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// A check-in for a recipient who no longer exists violates the user_id FK: a
// retryable failure, so the message must stay in the PEL rather than be acked.
func TestConsumerCheckinForUnknownRecipientStaysInPEL(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	h.enqueueCheckin(t, checkinEvent(uuid.New()))

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.EqualValues(t, 1, h.pendingCount(t))
}

// The two event types must not interfere: each lands as its own notification
// with its own type, from one stream and one tick.
func TestConsumerCheckinAndDecidedCoexistInOneFeed(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	user := h.seedUser(t, "both-types@e2e.local")
	h.enqueueDecided(t, decidedEvent(user, sqlc.SharedRequestStatusApproved))
	h.enqueueCheckin(t, checkinEvent(user))

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	published, err := relay.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, published)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	rows := h.notifications(t, user)
	require.Len(t, rows, 2)
	types := []sqlc.SharedNotificationType{rows[0].Type, rows[1].Type}
	assert.ElementsMatch(t, []sqlc.SharedNotificationType{
		sqlc.SharedNotificationTypeApprovalDecided,
		sqlc.SharedNotificationTypeAssetReturned,
	}, types)
}
