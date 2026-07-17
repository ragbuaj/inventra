// This file implements the fan-out consumer: the second half of the
// notification pipeline. The relay publishes business events onto the Redis
// Stream; this consumer group reads them, resolves each event's recipients, and
// writes one notification row per recipient.
//
// The consumer copies the worker lifecycle of relay.go / internal/importer:
// NewConsumer, Run with a ticker, and an exported Tick so integration tests
// drive it deterministically instead of racing the polling loop.
//
// Delivery is at-least-once: a message is acked only after its notification
// rows are committed, so a crash in between redelivers it. Duplicate rows are
// impossible anyway because CreateNotification is ON CONFLICT DO NOTHING
// against uq_notif_dedup -- the dedup key, not the ack, is what makes
// reprocessing safe.
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// ConsumerGroup is the fan-out consumer group on StreamKey. A future channel
// (email) joins as a second group on the same stream without touching the
// producer.
const ConsumerGroup = "notification-fanout"

// consumerBatchSize is how many messages one Tick reads (and how many it
// auto-claims) per pass.
const consumerBatchSize = 100

// defaultClaimMinIdle is how long a message must sit unacked in the Pending
// Entries List before another consumer may take it over. It must comfortably
// exceed a healthy processing time, or a live consumer's in-flight message
// would be stolen mid-work.
const defaultClaimMinIdle = 60 * time.Second

// errUndecodable marks a payload that no retry can fix. Redelivering the same
// bytes cannot make them parse, so the consumer acks instead of leaving a
// poison pill in the PEL forever.
var errUndecodable = errors.New("notification: undecodable event payload")

// eventHandler turns one event payload into notification rows. Returning an
// error keeps the message unacked so it is retried; returning errUndecodable
// acks it.
type eventHandler func(ctx context.Context, payload []byte) error

// Consumer reads published events off the Redis Stream and fans them out into
// per-user notification rows.
type Consumer struct {
	q        *sqlc.Queries
	rdb      *redis.Client
	name     string
	poll     time.Duration
	minIdle  time.Duration
	handlers map[string]eventHandler
}

// NewConsumer constructs a Consumer. name identifies this instance within the
// consumer group and must be unique per process (an empty name defaults to
// host-pid) -- two instances sharing a name would each see the other's pending
// messages as their own. A non-positive poll defaults to 2s (a zero-value
// time.Duration would make time.NewTicker panic); a non-positive minIdle
// defaults to defaultClaimMinIdle.
func NewConsumer(q *sqlc.Queries, rdb *redis.Client, name string, poll, minIdle time.Duration) *Consumer {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	if minIdle <= 0 {
		minIdle = defaultClaimMinIdle
	}
	if name == "" {
		host, err := os.Hostname()
		if err != nil || host == "" {
			host = "consumer"
		}
		name = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	c := &Consumer{q: q, rdb: rdb, name: name, poll: poll, minIdle: minIdle}
	// Dispatch is a map keyed on the outbox event_type, so adding an event type
	// is one entry plus its handler.
	c.handlers = map[string]eventHandler{
		approval.EventRequestDecided: c.handleRequestDecided,
	}
	return c
}

// Run polls at the configured interval until ctx is cancelled. Errors from an
// individual tick are logged and swallowed: an unacked message stays in the PEL
// and is retried, so a transient failure must not stop the loop.
func (c *Consumer) Run(ctx context.Context) {
	ticker := time.NewTicker(c.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.Tick(ctx); err != nil {
				slog.Error("notification consumer tick failed", "err", err)
			}
		}
	}
}

// Tick runs one consume pass -- take over stranded messages, then read new ones
// -- and reports how many messages were acked. Exposed for integration tests to
// drive the consumer deterministically instead of relying on the polling loop.
func (c *Consumer) Tick(ctx context.Context) (int, error) {
	if err := c.ensureGroup(ctx); err != nil {
		return 0, err
	}
	claimed, claimErr := c.claimStranded(ctx)
	fresh, readErr := c.readNew(ctx)
	return claimed + fresh, errors.Join(claimErr, readErr)
}

// ensureGroup creates the consumer group idempotently. Run on every tick, not
// once at startup: the group lives in Redis, so a Redis restart without AOF (or
// a stream deleted underneath us) would otherwise leave the consumer reading
// from a group that no longer exists. Start "0" rather than "$" so a group
// created after the relay has already published still sees the entries already
// on the stream.
func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, StreamKey, ConsumerGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// claimStranded takes over messages left unacked longer than minIdle -- either
// by a consumer that died mid-message, or by this consumer after a failed
// attempt -- and processes them. This is the retry path: without it a crashed
// consumer's in-flight messages would sit in the PEL forever.
func (c *Consumer) claimStranded(ctx context.Context) (int, error) {
	msgs, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   StreamKey,
		Group:    ConsumerGroup,
		Consumer: c.name,
		MinIdle:  c.minIdle,
		Start:    "0-0",
		Count:    consumerBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return c.handleMessages(ctx, msgs)
}

// readNew reads messages never delivered to this group. Block is -1 (no
// blocking) so a Tick on an empty stream returns immediately instead of parking
// the caller.
func (c *Consumer) readNew(ctx context.Context) (int, error) {
	streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: c.name,
		Streams:  []string{StreamKey, ">"},
		Count:    consumerBatchSize,
		Block:    -1,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	acked := 0
	var errs []error
	for _, s := range streams {
		n, hErr := c.handleMessages(ctx, s.Messages)
		acked += n
		errs = append(errs, hErr)
	}
	return acked, errors.Join(errs...)
}

// handleMessages processes a batch and acks each message that succeeded,
// reporting how many were acked. A failing message is logged and skipped rather
// than aborting the batch: each message has its own ack, so one poison message
// must not starve the ones behind it.
func (c *Consumer) handleMessages(ctx context.Context, msgs []redis.XMessage) (int, error) {
	acked := 0
	var errs []error
	for _, msg := range msgs {
		if err := c.process(ctx, msg); err != nil {
			// Deliberately NOT acked: the message stays in the PEL and a later
			// tick re-claims it via XAUTOCLAIM.
			slog.Error("notification consumer: message failed, leaving unacked for retry",
				"id", msg.ID, "err", err)
			errs = append(errs, err)
			continue
		}
		if err := c.rdb.XAck(ctx, StreamKey, ConsumerGroup, msg.ID).Err(); err != nil {
			// The rows are already committed; a failed ack only means the
			// message is redelivered, which uq_notif_dedup makes a no-op.
			errs = append(errs, err)
			continue
		}
		acked++
	}
	return acked, errors.Join(errs...)
}

// process routes one message to its handler. An unrecognized event type is
// acked, not retried: no redelivery can make it recognizable, and leaving it
// unacked would wedge it in the PEL to be re-claimed on every pass forever.
// Other event types (request_submitted, chain_advanced, assignment_checkin,
// maintenance_due) land here as handlers in later tasks.
func (c *Consumer) process(ctx context.Context, msg redis.XMessage) error {
	eventType, _ := msg.Values[FieldEventType].(string)
	handler, ok := c.handlers[eventType]
	if !ok {
		slog.Warn("notification consumer: unknown event type, acking",
			"id", msg.ID, "event_type", eventType)
		return nil
	}
	payload, _ := msg.Values[FieldPayload].(string)
	err := handler(ctx, []byte(payload))
	if errors.Is(err, errUndecodable) {
		slog.Error("notification consumer: undecodable payload, acking",
			"id", msg.ID, "event_type", eventType, "err", err)
		return nil
	}
	return err
}

// handleRequestDecided turns a terminally decided approval request into one
// approval_decided notification for the maker. The recipient is carried by the
// event (MakerID, from requests.requested_by_id), so no re-read of state that
// may have moved on is needed.
func (c *Consumer) handleRequestDecided(ctx context.Context, payload []byte) error {
	var ev approval.RequestDecidedEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return fmt.Errorf("%w: %v", errUndecodable, err)
	}
	if ev.RequestID == uuid.Nil || ev.MakerID == uuid.Nil {
		return fmt.Errorf("%w: missing request_id or maker_id", errUndecodable)
	}

	// Only interpolation params, never rendered text: the client renders the
	// sentence via i18n, so storing an Indonesian string here would freeze the
	// notification into one locale.
	params, err := json.Marshal(map[string]string{
		"request_type": string(ev.RequestType),
		"status":       string(ev.Status),
	})
	if err != nil {
		return err
	}

	// A request is decided terminally at most once (rejected, or approved at the
	// final step), so the request id alone identifies this event -- no outcome
	// needed in the key. That, plus ON CONFLICT DO NOTHING on
	// (user_id, dedup_key), is what makes redelivery a no-op.
	dedupKey := "request:" + ev.RequestID.String() + ":decided"
	entityType := approval.AggregateRequests
	entityID := ev.RequestID

	return c.q.CreateNotification(ctx, sqlc.CreateNotificationParams{
		UserID:     ev.MakerID,
		Type:       sqlc.SharedNotificationTypeApprovalDecided,
		Params:     params,
		EntityType: &entityType,
		EntityID:   &entityID,
		DedupKey:   &dedupKey,
	})
}
