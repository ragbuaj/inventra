//go:build integration

// Integration tests for the outbox relay against a real Postgres + Redis.
// relay.go exposes an exported Tick so each test drives one publish pass
// deterministically instead of waiting on the polling loop.
package notification_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/notification"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// harness bundles the fixtures every relay test needs.
type harness struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	q    *sqlc.Queries
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	return &harness{pool: pool, rdb: rdb, q: sqlc.New(pool)}
}

// resetOutbox clears the outbox and the stream. testsupport.Reset only
// truncates identity/masterdata/audit, so the notification schema is cleared
// here.
func (h *harness) resetOutbox(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_, err := h.pool.Exec(ctx, "TRUNCATE notification.outbox, notification.notifications RESTART IDENTITY CASCADE")
	require.NoError(t, err)
	require.NoError(t, h.rdb.Del(ctx, notification.StreamKey).Err())
}

// enqueue writes one outbox row, standing in for a business service's
// in-transaction enqueue.
func (h *harness) enqueue(t *testing.T, eventType string, payload map[string]any) sqlc.NotificationOutbox {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	row, err := h.q.EnqueueOutbox(context.Background(), sqlc.EnqueueOutboxParams{
		EventType:     eventType,
		AggregateType: "requests",
		AggregateID:   uuid.New(),
		Payload:       raw,
	})
	require.NoError(t, err)
	return row
}

// publishedAt reads back the outbox row's published_at.
func (h *harness) publishedAt(t *testing.T, id uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	err := h.pool.QueryRow(context.Background(),
		"SELECT published_at FROM notification.outbox WHERE id = $1", id).Scan(&at)
	require.NoError(t, err)
	return at
}

// streamEntries reads the whole stream.
func (h *harness) streamEntries(t *testing.T) []redis.XMessage {
	t.Helper()
	msgs, err := h.rdb.XRange(context.Background(), notification.StreamKey, "-", "+").Result()
	require.NoError(t, err)
	return msgs
}

func TestRelayPublishesClaimedRowExactlyOnce(t *testing.T) {
	h := newHarness(t)
	h.resetOutbox(t)
	ctx := context.Background()

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	row := h.enqueue(t, "request_decided", map[string]any{"result": "approved"})

	n, err := relay.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// published_at is set only after XADD succeeded.
	assert.NotNil(t, h.publishedAt(t, row.ID))

	msgs := h.streamEntries(t)
	require.Len(t, msgs, 1)
	vals := msgs[0].Values
	assert.Equal(t, row.ID.String(), vals[notification.FieldOutboxID])
	assert.Equal(t, "request_decided", vals[notification.FieldEventType])
	assert.Equal(t, "requests", vals[notification.FieldAggregateType])
	assert.Equal(t, row.AggregateID.String(), vals[notification.FieldAggregateID])
	assert.JSONEq(t, `{"result":"approved"}`, vals[notification.FieldPayload].(string))

	// A second tick must not republish an already-published row.
	n, err = relay.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Len(t, h.streamEntries(t), 1)
}

func TestRelayXAddFailureLeavesRowUnpublishedForRetry(t *testing.T) {
	h := newHarness(t)
	h.resetOutbox(t)
	ctx := context.Background()

	// A closed client makes every XADD fail, standing in for Redis being down.
	broken := redis.NewClient(&redis.Options{Addr: h.rdb.Options().Addr})
	require.NoError(t, broken.Close())

	row := h.enqueue(t, "request_decided", map[string]any{"result": "rejected"})

	failing := notification.NewRelay(h.q, h.pool, broken, 10000, time.Second)
	n, err := failing.Tick(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, n)

	// The no-lost-events guarantee: nothing was published, so published_at must
	// still be NULL and the row must still be claimable.
	assert.Nil(t, h.publishedAt(t, row.ID))
	assert.Empty(t, h.streamEntries(t))

	// The next tick, against a healthy Redis, delivers it.
	healthy := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	n, err = healthy.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.NotNil(t, h.publishedAt(t, row.ID))
	require.Len(t, h.streamEntries(t), 1)
}

func TestRelayPartialBatchFailureLeavesWholeBatchUnpublished(t *testing.T) {
	h := newHarness(t)
	h.resetOutbox(t)
	ctx := context.Background()

	rows := []sqlc.NotificationOutbox{
		h.enqueue(t, "request_decided", map[string]any{"n": 1}),
		h.enqueue(t, "request_decided", map[string]any{"n": 2}),
	}

	broken := redis.NewClient(&redis.Options{Addr: h.rdb.Options().Addr})
	require.NoError(t, broken.Close())

	relay := notification.NewRelay(h.q, h.pool, broken, 10000, time.Second)
	_, err := relay.Tick(ctx)
	require.Error(t, err)

	for _, row := range rows {
		assert.Nil(t, h.publishedAt(t, row.ID))
	}

	healthy := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	n, err := healthy.Tick(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Len(t, h.streamEntries(t), 2)
}

func TestRelayConcurrentRelaysDoNotDoublePublish(t *testing.T) {
	h := newHarness(t)
	h.resetOutbox(t)
	ctx := context.Background()

	const rowCount = 20
	ids := make([]uuid.UUID, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		ids = append(ids, h.enqueue(t, "request_decided", map[string]any{"n": i}).ID)
	}

	// Two relays racing on the same outbox: FOR UPDATE SKIP LOCKED inside a
	// transaction is what stops both from claiming the same row.
	a := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	b := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)

	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for _, relay := range []*notification.Relay{a, b} {
		wg.Add(1)
		go func(r *notification.Relay) {
			defer wg.Done()
			n, err := r.Tick(ctx)
			if err != nil {
				return
			}
			mu.Lock()
			total += n
			mu.Unlock()
		}(relay)
	}
	wg.Wait()

	// A row skipped by one relay is claimed by the next tick, so drain.
	for i := 0; i < 5; i++ {
		n, err := a.Tick(ctx)
		require.NoError(t, err)
		if n == 0 {
			break
		}
		total += n
	}

	assert.Equal(t, rowCount, total)
	for _, id := range ids {
		assert.NotNil(t, h.publishedAt(t, id))
	}

	// The real assertion: no row reached the stream twice.
	msgs := h.streamEntries(t)
	assert.Len(t, msgs, rowCount)
	seen := map[string]bool{}
	for _, m := range msgs {
		id := m.Values[notification.FieldOutboxID].(string)
		assert.False(t, seen[id], "outbox row %s published twice", id)
		seen[id] = true
	}
	assert.Len(t, seen, rowCount)
}

func TestRelayTrimsStreamToMaxLen(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// MAXLEN ~ trims whole macro nodes only (default 100 entries per node), so
	// a trimmed stream is asserted against an untrimmed control run rather than
	// an exact length.
	const total = 250

	drain := func(r *notification.Relay) {
		for {
			n, err := r.Tick(ctx)
			require.NoError(t, err)
			if n == 0 {
				return
			}
		}
	}

	h.resetOutbox(t)
	for i := 0; i < total; i++ {
		h.enqueue(t, "request_decided", map[string]any{"n": i})
	}
	drain(notification.NewRelay(h.q, h.pool, h.rdb, 0, time.Second)) // maxLen 0 disables trimming
	untrimmed, err := h.rdb.XLen(ctx, notification.StreamKey).Result()
	require.NoError(t, err)
	assert.EqualValues(t, total, untrimmed)

	h.resetOutbox(t)
	for i := 0; i < total; i++ {
		h.enqueue(t, "request_decided", map[string]any{"n": i})
	}
	drain(notification.NewRelay(h.q, h.pool, h.rdb, 1, time.Second))
	trimmed, err := h.rdb.XLen(ctx, notification.StreamKey).Result()
	require.NoError(t, err)
	assert.Less(t, trimmed, untrimmed, "stream must be trimmed toward MAXLEN")
}

func TestRelayTickOnEmptyOutboxIsNoOp(t *testing.T) {
	h := newHarness(t)
	h.resetOutbox(t)

	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, time.Second)
	n, err := relay.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, h.streamEntries(t))
}

func TestRelayRunStopsOnContextCancel(t *testing.T) {
	h := newHarness(t)
	h.resetOutbox(t)

	// A non-positive poll must default rather than panic in time.NewTicker.
	relay := notification.NewRelay(h.q, h.pool, h.rdb, 10000, 0)
	row := h.enqueue(t, "request_decided", map[string]any{"n": 1})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		relay.Run(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return h.publishedAt(t, row.ID) != nil
	}, 10*time.Second, 100*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
