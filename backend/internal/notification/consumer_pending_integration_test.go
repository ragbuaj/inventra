//go:build integration

// Integration tests for the request_submitted / chain_advanced -> approval_pending
// handler. Shares the harness of consumer_integration_test.go (same test package).
//
// Recipients are resolved through the ApproverResolver seam, stubbed here so
// each fan-out shape (nobody, one, many, stale) is exercised directly. The
// producer-side counterpart -- a real approval.Service.Submit driving this same
// pipeline with real eligibility -- lives in
// internal/approval/pending_notification_integration_test.go, where the real
// office/role/scope fixtures are.
package notification_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/notification"
)

// stubResolver stands in for *approval.Service. It records what it was asked,
// so a test can assert the handler passes the event's step through rather than
// assuming the current one.
type stubResolver struct {
	approvers []uuid.UUID
	err       error
	calls     []stubCall
}

type stubCall struct {
	requestID uuid.UUID
	step      int32
}

func (s *stubResolver) ApproversForStep(_ context.Context, requestID uuid.UUID, step int32) ([]uuid.UUID, error) {
	s.calls = append(s.calls, stubCall{requestID: requestID, step: step})
	if s.err != nil {
		return nil, s.err
	}
	return s.approvers, nil
}

// newPendingConsumer builds a consumer wired to the given resolver, with a
// ~instant min-idle so the XAUTOCLAIM path needs no waiting.
func (h *harness) newPendingConsumer(name string, r notification.ApproverResolver) *notification.Consumer {
	return notification.NewConsumer(h.q, h.rdb, r, nil, name, time.Second, time.Millisecond)
}

// enqueuePending writes a request_submitted or chain_advanced outbox row
// carrying the real event payload struct, so the test exercises the same wire
// contract approval writes.
func (h *harness) enqueuePending(t *testing.T, eventType string, ev approval.RequestPendingEvent) sqlc.NotificationOutbox {
	t.Helper()
	raw, err := json.Marshal(ev)
	require.NoError(t, err)
	row, err := h.q.EnqueueOutbox(context.Background(), sqlc.EnqueueOutboxParams{
		EventType:     eventType,
		AggregateType: approval.AggregateRequests,
		AggregateID:   ev.RequestID,
		Payload:       raw,
	})
	require.NoError(t, err)
	return row
}

// drain runs the relay then one consumer tick, moving every outbox row all the
// way into the notifications table.
func (h *harness) drain(t *testing.T, c *notification.Consumer) int {
	t.Helper()
	ctx := context.Background()
	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)
	n, err := c.Tick(ctx)
	require.NoError(t, err)
	return n
}

func pendingEvent(requestID uuid.UUID, step int32) approval.RequestPendingEvent {
	return approval.RequestPendingEvent{
		RequestID:   requestID,
		RequestType: sqlc.SharedRequestTypeAssetCreate,
		Step:        step,
	}
}

// The dedup key is a contract, not an implementation detail: the stale-
// notification sweep soft-deletes a passed step's notifications by this exact
// string. If this test and that sweep ever disagree, the sweep silently finds
// nothing.
func TestConsumerPendingDedupKeyFormat(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	approver := h.seedUser(t, "pending-dedup@e2e.local")
	requestID := uuid.New()
	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(requestID, 3))

	n := h.drain(t, h.newPendingConsumer("c1", &stubResolver{approvers: []uuid.UUID{approver}}))
	assert.Equal(t, 1, n)

	rows := h.notifications(t, approver)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].DedupKey)
	assert.Equal(t, "request:"+requestID.String()+":step:3", *rows[0].DedupKey,
		"the stale-notification sweep targets this exact format")
}

// One event, N recipients: every eligible approver gets their own row, and each
// row is addressed to exactly one user.
func TestConsumerPendingFansOutToEveryApprover(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	a1 := h.seedUser(t, "pending-fan1@e2e.local")
	a2 := h.seedUser(t, "pending-fan2@e2e.local")
	a3 := h.seedUser(t, "pending-fan3@e2e.local")
	requestID := uuid.New()
	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(requestID, 1))

	n := h.drain(t, h.newPendingConsumer("c1", &stubResolver{approvers: []uuid.UUID{a1, a2, a3}}))
	assert.Equal(t, 1, n, "one message, however many rows it produced")

	for _, u := range []uuid.UUID{a1, a2, a3} {
		rows := h.notifications(t, u)
		require.Len(t, rows, 1, "each approver gets exactly one row")
		assert.Equal(t, sqlc.SharedNotificationTypeApprovalPending, rows[0].Type)
		assert.Equal(t, u, rows[0].UserID)
		require.NotNil(t, rows[0].EntityType)
		assert.Equal(t, "requests", *rows[0].EntityType)
		require.NotNil(t, rows[0].EntityID)
		assert.Equal(t, requestID, *rows[0].EntityID)
		assert.False(t, rows[0].ReadAt.Valid, "a fresh notification must be unread")
		// Params carry i18n interpolation values, never rendered text.
		assert.JSONEq(t, `{"request_type":"asset_create","step":"1"}`, string(rows[0].Params))
	}
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// chain_advanced fans out identically to request_submitted -- same handler, same
// key shape -- but keyed to the NEW step, so it cannot collide with the step
// that just passed.
func TestConsumerPendingChainAdvancedKeysTheNewStep(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	approver := h.seedUser(t, "pending-adv@e2e.local")
	requestID := uuid.New()
	stub := &stubResolver{approvers: []uuid.UUID{approver}}

	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(requestID, 1))
	h.drain(t, h.newPendingConsumer("c1", stub))
	h.enqueuePending(t, approval.EventChainAdvanced, pendingEvent(requestID, 2))
	h.drain(t, h.newPendingConsumer("c1", stub))

	rows := h.notifications(t, approver)
	require.Len(t, rows, 2, "step 2 must not be deduped against step 1")
	keys := []string{*rows[0].DedupKey, *rows[1].DedupKey}
	assert.Contains(t, keys, "request:"+requestID.String()+":step:1")
	assert.Contains(t, keys, "request:"+requestID.String()+":step:2")

	// The handler resolves against the step the EVENT names, not a live lookup.
	require.Len(t, stub.calls, 2)
	assert.Equal(t, int32(1), stub.calls[0].step)
	assert.Equal(t, int32(2), stub.calls[1].step)
	assert.Equal(t, requestID, stub.calls[1].requestID)
}

// At-least-once made safe: redelivering the same event must not double up any
// recipient's feed.
func TestConsumerPendingRedeliveryYieldsOneRowPerRecipient(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	a1 := h.seedUser(t, "pending-dup1@e2e.local")
	a2 := h.seedUser(t, "pending-dup2@e2e.local")
	requestID := uuid.New()
	ev := pendingEvent(requestID, 1)
	stub := &stubResolver{approvers: []uuid.UUID{a1, a2}}

	// The identical event twice: exactly what a crash between the DB commit and
	// the XACK produces.
	h.enqueuePending(t, approval.EventRequestSubmitted, ev)
	h.enqueuePending(t, approval.EventRequestSubmitted, ev)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	published, err := relay.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, published)

	n, err := h.newPendingConsumer("c1", stub).Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "both messages must be acked")

	assert.Len(t, h.notifications(t, a1), 1, "the dedup key must collapse the redelivery")
	assert.Len(t, h.notifications(t, a2), 1)
}

// The stale-event decision, and the reason ApproversForStep checks state at all:
// uq_notif_dedup is partial on deleted_at IS NULL, so a soft-deleted row does
// NOT block reinsertion. Without this skip, a redelivered event for a step the
// chain has passed would resurrect a "waiting for you" that nobody can act on.
func TestConsumerPendingStaleEventWritesNothingAndIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	approver := h.seedUser(t, "pending-stale@e2e.local")
	requestID := uuid.New()
	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(requestID, 1))

	stub := &stubResolver{err: approval.ErrStepPassed}
	n := h.drain(t, h.newPendingConsumer("c1", stub))

	assert.Equal(t, 1, n, "a stale event is acked, not retried forever")
	assert.Empty(t, h.notifications(t, approver), "a passed step must not be announced")
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// A deleted request is the same class of unfixable: ack, write nothing.
func TestConsumerPendingVanishedRequestIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	requestID := uuid.New()
	h.enqueuePending(t, approval.EventChainAdvanced, pendingEvent(requestID, 2))

	n := h.drain(t, h.newPendingConsumer("c1", &stubResolver{err: approval.ErrNotFound}))
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// Nobody eligible is a legitimate outcome (every candidate is the maker, or out
// of scope), not a failure: ack and write nothing.
func TestConsumerPendingNoEligibleApproversIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(uuid.New(), 1))

	n := h.drain(t, h.newPendingConsumer("c1", &stubResolver{approvers: nil}))
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// A resolver failure is transient (the DB is down, Redis scope cache is
// unreachable): the message must stay in the PEL to be retried, never acked.
func TestConsumerPendingResolverFailureStaysInPEL(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	approver := h.seedUser(t, "pending-retry@e2e.local")
	requestID := uuid.New()
	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(requestID, 1))

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	stub := &stubResolver{err: errors.New("scope service unavailable")}
	consumer := h.newPendingConsumer("c1", stub)
	n, err := consumer.Tick(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.EqualValues(t, 1, h.pendingCount(t))

	// Genuinely retried, not merely parked: once the resolver recovers, the
	// re-claimed message succeeds and the PEL drains.
	stub.err = nil
	stub.approvers = []uuid.UUID{approver}
	n, err = consumer.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
	assert.Len(t, h.notifications(t, approver), 1)
}

// One unwritable recipient (no such user -> FK violation) must not cost the
// others their notification, and the message must still be retried.
func TestConsumerPendingPartialFailureKeepsGoodRowsAndRetries(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	good := h.seedUser(t, "pending-partial-good@e2e.local")
	ghost := uuid.New()
	requestID := uuid.New()
	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(requestID, 1))

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	stub := &stubResolver{approvers: []uuid.UUID{ghost, good}}
	consumer := h.newPendingConsumer("c1", stub)
	n, err := consumer.Tick(ctx)
	require.Error(t, err, "the failing recipient must surface")
	assert.Equal(t, 0, n)

	// No transaction wraps the fan-out, and that is the point: the good row is
	// already committed and the message is still pending for retry.
	assert.Len(t, h.notifications(t, good), 1,
		"one bad recipient must not roll back a good one")
	assert.EqualValues(t, 1, h.pendingCount(t))

	// The retry is idempotent for the row that already landed.
	stub.approvers = []uuid.UUID{good}
	n, err = consumer.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Len(t, h.notifications(t, good), 1, "the retry must not duplicate the good row")
}

// A malformed pending payload is unfixable by retry: ack it rather than wedge
// the PEL.
func TestConsumerPendingUndecodablePayloadIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	h.enqueue(t, approval.EventRequestSubmitted, map[string]any{"request_id": "not-a-uuid"})

	n := h.drain(t, h.newPendingConsumer("c1", &stubResolver{}))
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
}

// Well-formed JSON that names no step cannot be resolved against anything: same
// treatment.
func TestConsumerPendingPayloadWithoutStepIsAcked(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)

	stub := &stubResolver{}
	h.enqueue(t, approval.EventChainAdvanced, map[string]any{
		"request_id": uuid.New().String(), "request_type": "asset_create",
	})

	n := h.drain(t, h.newPendingConsumer("c1", stub))
	assert.Equal(t, 1, n)
	assert.EqualValues(t, 0, h.pendingCount(t))
	assert.Empty(t, stub.calls, "a step-less event must never reach the resolver")
}

// A consumer built without a resolver must not silently swallow approval
// events: a misconfigured process is a bug to be seen, and the event is kept.
func TestConsumerPendingWithoutResolverKeepsEventForRetry(t *testing.T) {
	h := newHarness(t)
	h.resetConsumer(t)
	ctx := context.Background()

	h.enqueuePending(t, approval.EventRequestSubmitted, pendingEvent(uuid.New(), 1))
	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.NoError(t, err)

	n, err := h.newConsumer("c1").Tick(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.EqualValues(t, 1, h.pendingCount(t), "the event must survive the misconfiguration")
}
