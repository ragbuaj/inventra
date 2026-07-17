//go:build integration

// Integration tests for the fan-out consumer against a real Postgres + Redis.
// consumer.go exposes an exported Tick so each test drives one consume pass
// deterministically instead of waiting on the polling loop.
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
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/notification"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// resetConsumer clears the outbox, the notifications table and the stream. The
// group lives on the stream, so deleting the stream drops it too -- every test
// therefore starts with no group and relies on the consumer creating it.
func (h *harness) resetConsumer(t *testing.T) {
	t.Helper()
	testsupport.Reset(t, h.pool)
	h.resetOutbox(t)
}

// seedUser inserts an identity.users row. notifications.user_id is a real FK,
// so a recipient must exist before a notification can be written for them.
func (h *harness) seedUser(t *testing.T, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var roleID uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`INSERT INTO identity.roles (code, name) VALUES ($1, $2) RETURNING id`,
		"role-"+email, "role-"+email).Scan(&roleID))
	var id uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, status)
		 VALUES ($1, $2, $3, 'active') RETURNING id`,
		email, email, roleID).Scan(&id))
	return id
}

// enqueueDecided writes a request_decided outbox row carrying the real event
// payload struct, so the test exercises the same wire contract approval writes.
func (h *harness) enqueueDecided(t *testing.T, ev approval.RequestDecidedEvent) sqlc.NotificationOutbox {
	t.Helper()
	raw, err := json.Marshal(ev)
	require.NoError(t, err)
	row, err := h.q.EnqueueOutbox(context.Background(), sqlc.EnqueueOutboxParams{
		EventType:     approval.EventRequestDecided,
		AggregateType: approval.AggregateRequests,
		AggregateID:   ev.RequestID,
		Payload:       raw,
	})
	require.NoError(t, err)
	return row
}

// notifications reads a user's feed straight from the table.
func (h *harness) notifications(t *testing.T, userID uuid.UUID) []sqlc.NotificationNotification {
	t.Helper()
	rows, err := h.q.ListNotifications(context.Background(), sqlc.ListNotificationsParams{
		UserID: userID, Lim: 100, Off: 0,
	})
	require.NoError(t, err)
	return rows
}

// pendingCount reports how many messages sit unacked in the group's PEL.
func (h *harness) pendingCount(t *testing.T) int64 {
	t.Helper()
	res, err := h.rdb.XPending(context.Background(), notification.StreamKey, notification.ConsumerGroup).Result()
	if err == redis.Nil {
		return 0
	}
	require.NoError(t, err)
	return res.Count
}

// newConsumer builds a consumer whose min-idle is ~instant, so a test can drive
// the XAUTOCLAIM takeover path without waiting out a production idle window.
// The approver resolver is nil: these tests carry no approval_pending events,
// and the fan-out tests that do supply a real one.
func (h *harness) newConsumer(name string) *notification.Consumer {
	return notification.NewConsumer(h.q, h.rdb, nil, nil, name, time.Second, time.Millisecond)
}

// decidedEvent builds a well-formed event for the given maker.
func decidedEvent(maker uuid.UUID, status sqlc.SharedRequestStatus) approval.RequestDecidedEvent {
	decider := uuid.New()
	return approval.RequestDecidedEvent{
		RequestID:   uuid.New(),
		RequestType: sqlc.SharedRequestTypeAssetCreate,
		Status:      status,
		MakerID:     maker,
		DecidedByID: &decider,
	}
}

// The whole point of Phase 2: a real notification born through the entire
// pipeline -- outbox row, real relay, stream, real consumer, table -- so the
// contract between the two halves is actually exercised rather than mocked.
func TestConsumerMakerGetsApprovalDecidedEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	maker := h.seedUser(t, "maker@e2e.local")
	ev := decidedEvent(maker, sqlc.SharedRequestStatusApproved)
	h.enqueueDecided(t, ev)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	published, err := relay.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, published)

	consumer := h.newConsumer("c1")
	n, err := consumer.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	rows := h.notifications(t, maker)
	require.Len(t, rows, 1)
	got := rows[0]
	assert.Equal(t, maker, got.UserID)
	assert.Equal(t, sqlc.SharedNotificationTypeApprovalDecided, got.Type)
	require.NotNil(t, got.EntityType)
	assert.Equal(t, "requests", *got.EntityType)
	require.NotNil(t, got.EntityID)
	assert.Equal(t, ev.RequestID, *got.EntityID)
	require.NotNil(t, got.DedupKey)
	assert.Equal(t, "request:"+ev.RequestID.String()+":decided", *got.DedupKey)
	assert.False(t, got.ReadAt.Valid, "a fresh notification must be unread")

	// Params carry i18n interpolation values, never rendered text.
	assert.JSONEq(t, `{"request_type":"asset_create","status":"approved"}`, string(got.Params))

	// Acked: nothing left pending.
	assert.EqualValues(t, 0, h.pendingCount(t))
}

func TestConsumerRejectedCarriesRejectedStatus(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	maker := h.seedUser(t, "maker-reject@e2e.local")
	ev := decidedEvent(maker, sqlc.SharedRequestStatusRejected)
	h.enqueueDecided(t, ev)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	_, err = h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)

	rows := h.notifications(t, maker)
	require.Len(t, rows, 1)
	// Approve and reject must be distinguishable by params alone -- the type is
	// approval_decided for both.
	assert.JSONEq(t, `{"request_type":"asset_create","status":"rejected"}`, string(rows[0].Params))
}

// The load-bearing at-least-once guarantee: uq_notif_dedup + ON CONFLICT DO
// NOTHING mean a redelivered message cannot duplicate a row.
func TestConsumerProcessingSameMessageTwiceYieldsOneNotification(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	maker := h.seedUser(t, "maker-dup@e2e.local")
	ev := decidedEvent(maker, sqlc.SharedRequestStatusApproved)
	raw, err := json.Marshal(ev)
	require.NoError(t, err)

	// XADD the identical event twice: the same logical event delivered twice,
	// exactly what a crash between the DB commit and the XACK produces.
	for i := 0; i < 2; i++ {
		require.NoError(t, h.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: notification.StreamKey,
			Values: map[string]any{
				notification.FieldOutboxID:      uuid.New().String(),
				notification.FieldEventType:     approval.EventRequestDecided,
				notification.FieldAggregateType: approval.AggregateRequests,
				notification.FieldAggregateID:   ev.RequestID.String(),
				notification.FieldPayload:       string(raw),
			},
		}).Err())
	}

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "both messages must be acked")

	assert.Len(t, h.notifications(t, maker), 1, "the dedup key must collapse the redelivery")
}

// A failing message must not be acked, or the retry Redis Streams gives us for
// free is thrown away.
func TestConsumerFailingMessageIsNotAckedAndStaysInPEL(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	// A maker who does not exist violates the notifications.user_id FK, so
	// CreateNotification fails -- a retryable failure, unlike a bad payload.
	ev := decidedEvent(uuid.New(), sqlc.SharedRequestStatusApproved)
	h.enqueueDecided(t, ev)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	consumer := h.newConsumer("c1")
	n, err := consumer.Tick(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.EqualValues(t, 1, h.pendingCount(t), "a failed message must stay in the PEL")

	// It is genuinely retried, not merely parked: once the recipient exists the
	// re-claimed message succeeds and the PEL drains.
	_, err = h.pool.Exec(ctx,
		`INSERT INTO identity.roles (code, name) VALUES ('r-late', 'r-late')`)
	require.NoError(t, err)
	var roleID uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT id FROM identity.roles WHERE code = 'r-late'`).Scan(&roleID))
	_, err = h.pool.Exec(ctx,
		`INSERT INTO identity.users (id, name, email, role_id, status)
		 VALUES ($1, 'late', 'late@e2e.local', $2, 'active')`, ev.MakerID, roleID)
	require.NoError(t, err)

	n, err = consumer.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
	assert.Len(t, h.notifications(t, ev.MakerID), 1)
}

// A consumer that dies mid-message leaves it in the PEL; XAUTOCLAIM is what
// lets a live consumer take it over instead of it being stranded forever.
func TestConsumerXAutoClaimPicksUpMessageStrandedByDeadConsumer(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	maker := h.seedUser(t, "maker-claim@e2e.local")
	ev := decidedEvent(maker, sqlc.SharedRequestStatusApproved)
	h.enqueueDecided(t, ev)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	// "dead" reads the message into its own PEL entry and then never acks --
	// it crashed. Read via XREADGROUP directly so nothing is processed.
	require.NoError(t, h.rdb.XGroupCreateMkStream(ctx, notification.StreamKey, notification.ConsumerGroup, "0").Err())
	msgs, err := h.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    notification.ConsumerGroup,
		Consumer: "dead",
		Streams:  []string{notification.StreamKey, ">"},
		Count:    10,
		Block:    -1,
	}).Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Messages, 1)
	require.EqualValues(t, 1, h.pendingCount(t))

	// Let the entry idle past the consumer's min-idle window.
	time.Sleep(20 * time.Millisecond)

	// A fresh consumer sees nothing new (">" is drained) -- only XAUTOCLAIM can
	// rescue this message.
	alive := h.newConsumer("alive")
	n, err := alive.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "XAUTOCLAIM must take over the stranded message")

	assert.Len(t, h.notifications(t, maker), 1)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// An unknown event type must be acked. Later tasks add the other types; until
// then one must not be able to wedge the group.
func TestConsumerUnknownEventTypeIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	h.enqueue(t, "something_nobody_handles", map[string]any{"anything": true})

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t), "an unknown type must not sit in the PEL forever")
}

// A payload no retry can fix is acked too, for the same reason -- but it is a
// distinct path from an unknown type, so it gets its own test.
func TestConsumerUndecodablePayloadIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	require.NoError(t, h.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: notification.StreamKey,
		Values: map[string]any{
			notification.FieldOutboxID:      uuid.New().String(),
			notification.FieldEventType:     approval.EventRequestDecided,
			notification.FieldAggregateType: approval.AggregateRequests,
			notification.FieldAggregateID:   uuid.New().String(),
			notification.FieldPayload:       "{not json",
		},
	}).Err())

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// A well-formed JSON payload missing the recipient is equally unfixable by
// retry: it must be acked, not left to loop forever.
func TestConsumerPayloadWithoutMakerIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	h.enqueue(t, approval.EventRequestDecided, map[string]any{
		"request_id": uuid.New().String(), "status": "approved",
	})

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

func TestConsumerTickOnEmptyStreamIsNoOp(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	n, err := h.newConsumer("c1").Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// The group must be created idempotently: the second tick must not blow up on
// BUSYGROUP.
func TestConsumerRepeatedTicksAreIdempotent(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	consumer := h.newConsumer("c1")
	for i := 0; i < 3; i++ {
		n, err := consumer.Tick(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	}
}

// A message published BEFORE the group exists must still be delivered -- the
// group is created at "0", not "$".
func TestConsumerSeesEventsPublishedBeforeGroupExisted(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	maker := h.seedUser(t, "maker-early@e2e.local")
	h.enqueueDecided(t, decidedEvent(maker, sqlc.SharedRequestStatusApproved))

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	// The group has never existed at this point; the consumer creates it here.
	n, err := h.newConsumer("late-starter").Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Len(t, h.notifications(t, maker), 1)
}

// A batch must not be starved by one bad message: the healthy ones behind it
// still get processed and acked.
func TestConsumerOneFailingMessageDoesNotStarveTheRest(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	maker := h.seedUser(t, "maker-batch@e2e.local")
	// Ordered: the failing event (unknown recipient) is published first.
	bad := decidedEvent(uuid.New(), sqlc.SharedRequestStatusApproved)
	good := decidedEvent(maker, sqlc.SharedRequestStatusApproved)
	h.enqueueDecided(t, bad)
	h.enqueueDecided(t, good)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	published, err := relay.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, published)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.Error(t, err)
	assert.Equal(t, 1, n)

	assert.Len(t, h.notifications(t, maker), 1, "the healthy message must still land")
	assert.EqualValues(t, 1, h.pendingCount(t), "only the failing message stays pending")
}

func TestConsumerRunStopsOnContextCancel(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	maker := h.seedUser(t, "maker-run@e2e.local")
	h.enqueueDecided(t, decidedEvent(maker, sqlc.SharedRequestStatusApproved))

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	// A non-positive poll must default rather than panic in time.NewTicker.
	consumer := notification.NewConsumer(h.q, h.rdb, nil, nil, "runner", 0, time.Millisecond)

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		consumer.Run(runCtx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return len(h.notifications(t, maker)) == 1
	}, 10*time.Second, 100*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
