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

-- Notifications: the permanent per-user feed.

-- ON CONFLICT DO NOTHING against uq_notif_dedup is what makes the at-least-once
-- consumer safe -- redelivery cannot duplicate a row.
-- name: CreateNotification :exec
INSERT INTO notification.notifications
  (user_id, type, params, entity_type, entity_id, dedup_key)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING;

-- name: ListNotifications :many
SELECT * FROM notification.notifications
WHERE user_id = @user_id AND deleted_at IS NULL
  AND (NOT @unread_only::boolean OR read_at IS NULL)
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountNotifications :one
SELECT count(*) FROM notification.notifications
WHERE user_id = @user_id AND deleted_at IS NULL
  AND (NOT @unread_only::boolean OR read_at IS NULL);

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
