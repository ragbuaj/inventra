// This file implements the sweeper: the time-driven third of the notification
// pipeline. The relay and the consumer react to events a business transaction
// wrote; nothing writes an event when a maintenance schedule merely becomes due,
// because becoming due is the passage of time rather than an action. The sweeper
// supplies that missing producer, and does the retention purge in the same pass.
//
// The sweeper copies the worker pattern of relay.go / internal/importer: NewX,
// Run with a ticker, and an exported Tick so integration tests drive it
// deterministically instead of racing the polling loop.
package notification

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// EventMaintenanceDue is the outbox event_type the due scan enqueues. The
// sweeper deliberately goes through the outbox rather than writing notification
// rows directly: every notification then reaches users by one path, and a future
// second consumer group (email) receives this event without the sweeper knowing
// it exists.
const EventMaintenanceDue = "maintenance_due"

// AggregateMaintenanceSchedules is the outbox aggregate_type of a due reminder.
const AggregateMaintenanceSchedules = "maintenance_schedules"

// dueLookAheadDays is how far ahead the due scan looks. One day matches the
// mockup's "jatuh tempo besok": a reminder arrives the day before, and anything
// already overdue is swept in by the same `next_due_date <= today+1` bound.
//
// This is a constant rather than config because the value is load-bearing on the
// user-facing copy, not an operational knob: widening it would make the rendered
// sentence wrong. The tick rate (NOTIFICATION_SWEEP_POLL) is the knob.
const dueLookAheadDays = 1

// Sweeper enqueues maintenance_due events for schedules coming due and purges
// notifications and outbox rows past the retention window.
type Sweeper struct {
	q             *sqlc.Queries
	pool          *pgxpool.Pool
	retentionDays int
	poll          time.Duration
}

// NewSweeper constructs a Sweeper. poll is the interval between ticks in Run; a
// non-positive poll defaults to 1h (a zero-value time.Duration would make
// time.NewTicker panic). A non-positive retentionDays disables the purge, which
// is how an operator turns retention off without turning the due scan off too.
func NewSweeper(q *sqlc.Queries, pool *pgxpool.Pool, retentionDays int, poll time.Duration) *Sweeper {
	if poll <= 0 {
		poll = time.Hour
	}
	return &Sweeper{q: q, pool: pool, retentionDays: retentionDays, poll: poll}
}

// Run polls at the configured interval until ctx is cancelled. Errors from an
// individual tick are logged and swallowed: every step of a tick is idempotent,
// so the next tick redoes whatever this one failed to do, and a transient
// failure must not stop the loop.
func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.Tick(ctx); err != nil {
				slog.Error("notification sweeper tick failed", "err", err)
			}
		}
	}
}

// Tick runs one sweep pass and reports how many due reminders it enqueued.
// Exposed for integration tests to drive the sweeper deterministically instead
// of relying on the polling loop.
//
// The whole pass is one transaction that opens by taking
// pg_advisory_xact_lock(hashtext('notification.sweep')). The lock is
// transaction-scoped, so it is released on commit or rollback with no unlock
// path to leak, and it is what keeps two API instances from sweeping at once.
// It is a blocking lock rather than a try-lock: a tick that waits out a
// concurrent sweep and then finds nothing left to do is correct, and skipping
// would be indistinguishable from failing.
func (s *Sweeper) Tick(ctx context.Context) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := s.q.WithTx(tx)
	if err := qtx.AdvisoryLockNotificationSweep(ctx); err != nil {
		return 0, err
	}

	// Due scan before purge, so a reminder enqueued now is never a candidate for
	// the purge running in the same transaction.
	enqueued, err := s.scanDue(ctx, qtx)
	if err != nil {
		return 0, err
	}
	if err := s.purge(ctx, qtx); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return enqueued, nil
}

// MaintenanceDueEvent is the payload of a maintenance_due outbox row. It carries
// the asset's office so the consumer can filter recipients by scope without
// re-reading the asset, and the due date because that -- with the schedule id --
// is the event's identity on both the outbox and the notification side.
type MaintenanceDueEvent struct {
	ScheduleID uuid.UUID `json:"schedule_id"`
	AssetID    uuid.UUID `json:"asset_id"`
	AssetName  string    `json:"asset_name"`
	AssetTag   string    `json:"asset_tag"`
	OfficeID   uuid.UUID `json:"office_id"`
	// DueDate is "2006-01-02". EnqueueMaintenanceDueOutbox reads this exact key
	// out of the payload to decide whether the reminder is already enqueued.
	DueDate string `json:"due_date"`
}

// scanDue enqueues one maintenance_due event per schedule due within the
// look-ahead, skipping those already enqueued for the same due date.
func (s *Sweeper) scanDue(ctx context.Context, qtx *sqlc.Queries) (int, error) {
	dueBefore := pgtype.Date{
		Time:  time.Now().AddDate(0, 0, dueLookAheadDays),
		Valid: true,
	}
	rows, err := qtx.ListSchedulesDueBetween(ctx, dueBefore)
	if err != nil {
		return 0, err
	}

	enqueued := 0
	for _, row := range rows {
		sched := row.MaintenanceMaintenanceSchedule
		if !sched.NextDueDate.Valid {
			// The query already filters next_due_date IS NOT NULL; without a due
			// date there is no dedup identity, so skipping is the only safe act.
			continue
		}
		ev := MaintenanceDueEvent{
			ScheduleID: sched.ID,
			AssetID:    sched.AssetID,
			AssetName:  row.AssetName,
			AssetTag:   row.AssetTag,
			OfficeID:   row.OfficeID,
			DueDate:    sched.NextDueDate.Time.Format(dateLayout),
		}
		payload, err := json.Marshal(ev)
		if err != nil {
			return 0, err
		}
		n, err := qtx.EnqueueMaintenanceDueOutbox(ctx, sqlc.EnqueueMaintenanceDueOutboxParams{
			EventType:     EventMaintenanceDue,
			AggregateType: AggregateMaintenanceSchedules,
			AggregateID:   sched.ID,
			Payload:       payload,
		})
		if err != nil {
			return 0, err
		}
		enqueued += int(n)
	}
	return enqueued, nil
}

// purge soft-deletes notifications and already-published outbox rows older than
// the retention window. Soft delete is not cosmetic here: every index on both
// tables is partial on `deleted_at IS NULL`, so a purged row leaves the indexes
// entirely and the feed and unread count stay fast however large the tables get.
func (s *Sweeper) purge(ctx context.Context, qtx *sqlc.Queries) error {
	if s.retentionDays <= 0 {
		return nil
	}
	cutoff := pgtype.Timestamptz{
		Time:  time.Now().AddDate(0, 0, -s.retentionDays),
		Valid: true,
	}
	if err := qtx.PurgeNotifications(ctx, cutoff); err != nil {
		return err
	}
	return qtx.PurgeOutbox(ctx, cutoff)
}

// dateLayout is the wire format of MaintenanceDueEvent.DueDate. The consumer
// puts the same string into the notification's dedup key and params, so the
// client formats the date for display rather than the server picking a locale.
const dateLayout = "2006-01-02"
