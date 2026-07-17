-- Outbox: written inside the caller's business transaction, so a rollback leaves
-- no orphan event and a commit can never lose one.

-- name: EnqueueOutbox :one
INSERT INTO notification.outbox (event_type, aggregate_type, aggregate_id, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- Same FOR UPDATE SKIP LOCKED claim the import worker uses, so two relays never
-- publish the same row.
-- name: ClaimUnpublishedOutbox :many
SELECT * FROM notification.outbox
WHERE published_at IS NULL AND deleted_at IS NULL
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT $1;

-- Only ever called after XADD succeeds: an unpublished row is retried next tick
-- rather than lost.
-- name: MarkOutboxPublished :exec
UPDATE notification.outbox SET published_at = now() WHERE id = $1;

-- Sweeper: one instance at a time. Transaction-scoped exclusive lock, released
-- automatically at COMMIT/ROLLBACK (precedent: AdvisoryLockDepreciation).
-- name: AdvisoryLockNotificationSweep :exec
SELECT pg_advisory_xact_lock(hashtext('notification.sweep'));

-- Outbox-side idempotency for the sweeper's due scan. uq_notif_dedup guards the
-- notifications table, not the outbox: without this guard every sweep tick would
-- enqueue the same reminder again and the relay would faithfully publish each
-- one. The insert and its existence check are ONE statement, so correctness does
-- not rest on the advisory lock alone.
--
-- Identity is (schedule, due date), read out of the payload -- the same identity
-- as the notification's dedup_key. `deleted_at IS NULL` mirrors uq_notif_dedup's
-- partial predicate so both sides forget at the same retention boundary: a
-- schedule still overdue after its reminder is purged earns a fresh one.
-- name: EnqueueMaintenanceDueOutbox :execrows
INSERT INTO notification.outbox (event_type, aggregate_type, aggregate_id, payload)
SELECT @event_type::text, @aggregate_type::text, @aggregate_id::uuid, @payload::jsonb
WHERE NOT EXISTS (
  SELECT 1 FROM notification.outbox
  WHERE event_type = @event_type::text
    AND aggregate_id = @aggregate_id::uuid
    AND payload->>'due_date' = @payload::jsonb->>'due_date'
    AND deleted_at IS NULL
);

-- Notifications: the permanent per-user feed.

-- ON CONFLICT DO NOTHING against uq_notif_dedup is what makes the at-least-once
-- consumer safe -- redelivery cannot duplicate a row.
-- name: CreateNotification :exec
INSERT INTO notification.notifications
  (user_id, type, params, entity_type, entity_id, dedup_key)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING;

-- unread_only and read_only are mutually exclusive flags carrying the tri-state
-- "read" filter: both false means no filter (the whole feed).
-- name: ListNotifications :many
SELECT * FROM notification.notifications
WHERE user_id = @user_id AND deleted_at IS NULL
  AND (NOT @unread_only::boolean OR read_at IS NULL)
  AND (NOT @read_only::boolean OR read_at IS NOT NULL)
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- Same predicate as ListNotifications so total always matches the filtered page.
-- name: CountNotifications :one
SELECT count(*) FROM notification.notifications
WHERE user_id = @user_id AND deleted_at IS NULL
  AND (NOT @unread_only::boolean OR read_at IS NULL)
  AND (NOT @read_only::boolean OR read_at IS NOT NULL);

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notification.notifications
WHERE user_id = @user_id AND read_at IS NULL AND deleted_at IS NULL;

-- user_id is part of the predicate, not just the lookup: marking someone else's
-- notification read must affect zero rows (the handler turns that into a 404).
-- name: MarkNotificationRead :one
UPDATE notification.notifications
SET read_at = now()
WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL
RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notification.notifications
SET read_at = now()
WHERE user_id = @user_id AND read_at IS NULL AND deleted_at IS NULL;

-- Auto-resolve: a notification whose turn has passed is soft-deleted, not just
-- marked read -- it cannot be acted on, so it should not sit in the feed.

-- Clears exactly one step. Every recipient of a step shares the same dedup_key
-- (only user_id differs), so an exact match already sweeps all of them.
-- Prefer this over the prefix form whenever a single step is meant: the prefix
-- 'request:<id>:step:1' also matches step:10, step:11, ...
-- name: SoftDeleteNotificationsByDedupKey :exec
UPDATE notification.notifications
SET deleted_at = now()
WHERE dedup_key = @dedup_key AND deleted_at IS NULL;

-- Prefix form, for sweeping every step of a request at once. Callers must pass
-- a prefix that cannot straddle a boundary -- 'request:<id>:step:' with the
-- trailing colon, never 'request:<id>:step:<n>'. The keys carry no LIKE
-- metacharacter (no '%' or '_'), so no escaping is needed.
-- name: SoftDeleteNotificationsByDedupPrefix :exec
UPDATE notification.notifications
SET deleted_at = now()
WHERE dedup_key LIKE @prefix || '%' AND deleted_at IS NULL;

-- Retention purge. Soft delete keeps the convention; because every index is
-- partial on deleted_at IS NULL, purged rows leave the indexes entirely, so the
-- feed and unread count stay fast however large the table grows.
-- name: PurgeNotifications :exec
UPDATE notification.notifications
SET deleted_at = now()
WHERE deleted_at IS NULL AND created_at < @cutoff;

-- name: PurgeOutbox :exec
UPDATE notification.outbox
SET deleted_at = now()
WHERE deleted_at IS NULL AND published_at IS NOT NULL AND created_at < @cutoff;
