// This file implements the outbox relay: the transport half of the
// notification pipeline. Business services write an event into
// notification.outbox inside their own transaction (no dual-write); the relay
// claims unpublished rows and republishes them onto a Redis Stream, where the
// fan-out consumer group turns each event into per-user notifications.
//
// The relay copies internal/importer/worker.go: NewRelay, Run with a ticker,
// and an exported Tick so integration tests drive it deterministically instead
// of racing the polling loop.
package notification

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// StreamKey is the Redis Stream the relay publishes to and the fan-out
// consumer group reads from.
const StreamKey = "notification.events"

// Stream field names of one published event. The consumer resolves recipients
// from these alone, so they must carry everything fan-out needs.
const (
	FieldOutboxID      = "outbox_id"
	FieldEventType     = "event_type"
	FieldAggregateType = "aggregate_type"
	FieldAggregateID   = "aggregate_id"
	FieldPayload       = "payload"
)

// relayBatchSize is how many outbox rows one Tick claims. Bounded so a backlog
// is drained across several ticks rather than in one long-held transaction.
const relayBatchSize = 100

// Relay moves rows from notification.outbox onto the Redis Stream.
type Relay struct {
	q      *sqlc.Queries
	pool   *pgxpool.Pool
	rdb    *redis.Client
	maxLen int64
	poll   time.Duration
}

// NewRelay constructs a Relay. poll is the interval between ticks in Run; a
// non-positive poll defaults to 2s (a zero-value time.Duration would make
// time.NewTicker panic). A non-positive maxLen disables stream trimming.
func NewRelay(q *sqlc.Queries, pool *pgxpool.Pool, rdb *redis.Client, maxLen int64, poll time.Duration) *Relay {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	return &Relay{q: q, pool: pool, rdb: rdb, maxLen: maxLen, poll: poll}
}

// Run polls at the configured interval until ctx is cancelled. Errors from an
// individual tick are logged and swallowed: an unpublished row keeps its NULL
// published_at and is retried next tick, so a transient failure must not stop
// the loop.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.Tick(ctx); err != nil {
				slog.Error("notification relay tick failed", "err", err)
			}
		}
	}
}

// Tick runs one relay pass and reports how many events were published.
// Exposed for integration tests to drive the relay deterministically instead
// of relying on the polling loop.
func (r *Relay) Tick(ctx context.Context) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	// ClaimUnpublishedOutbox uses FOR UPDATE SKIP LOCKED, whose row locks last
	// only as long as this transaction: everything below must stay inside it or
	// two relays could claim and publish the same row.
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := r.q.WithTx(tx)

	rows, err := qtx.ClaimUnpublishedOutbox(ctx, relayBatchSize)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	published := 0
	for _, row := range rows {
		if err := r.publish(ctx, row); err != nil {
			// Do NOT mark published: published_at stays NULL and the next tick
			// retries the row. Losing an event is the one thing this pipeline
			// must never do, so a publish failure aborts the batch and discards
			// the marks already made in this transaction rather than risking a
			// partially-marked, partially-published claim.
			return 0, err
		}
		if err := qtx.MarkOutboxPublished(ctx, row.ID); err != nil {
			return 0, err
		}
		published++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return published, nil
}

// publish XADDs one outbox row onto the stream, trimming to maxLen. Trimming is
// approximate (MAXLEN ~) because the stream is transport, not storage: the
// outbox remains the durable record, so exact trim boundaries buy nothing and
// cost latency.
func (r *Relay) publish(ctx context.Context, row sqlc.NotificationOutbox) error {
	payload := row.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	args := &redis.XAddArgs{
		Stream: StreamKey,
		Values: map[string]any{
			FieldOutboxID:      row.ID.String(),
			FieldEventType:     row.EventType,
			FieldAggregateType: row.AggregateType,
			FieldAggregateID:   row.AggregateID.String(),
			FieldPayload:       string(payload),
		},
	}
	if r.maxLen > 0 {
		args.MaxLen = r.maxLen
		args.Approx = true
	}
	return r.rdb.XAdd(ctx, args).Err()
}
