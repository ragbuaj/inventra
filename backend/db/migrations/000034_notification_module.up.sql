-- Notification module: transactional outbox + per-user in-app feed.
-- See docs/superpowers/specs/2026-07-17-notifications-design.md.
--
-- Redis Streams is transport only. Permanent state lives in these two tables
-- (outbox then notifications); losing Redis loses no state, because the relay
-- republishes from the outbox.

CREATE SCHEMA IF NOT EXISTS notification;

CREATE TYPE shared.notification_type AS ENUM (
  'approval_pending',
  'approval_decided',
  'maintenance_due',
  'asset_returned'
);

-- Business events, written in the SAME transaction as the change they describe.
-- That is what removes the dual-write hole: a rollback leaves no orphan event,
-- and a commit can never lose one.
CREATE TABLE IF NOT EXISTS notification.outbox (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type     text NOT NULL,
  aggregate_type text NOT NULL,
  aggregate_id   uuid NOT NULL,
  payload        jsonb NOT NULL DEFAULT '{}',
  published_at   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);

-- Backs the relay's claim query (unpublished rows, oldest first).
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished
  ON notification.outbox (created_at)
  WHERE published_at IS NULL AND deleted_at IS NULL;

CREATE TRIGGER trg_outbox_set_updated BEFORE UPDATE ON notification.outbox
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- The per-user feed. Rendered text is never stored: rows carry `type` + `params`
-- and the client renders them through i18n, so switching locale keeps working.
CREATE TABLE IF NOT EXISTS notification.notifications (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES identity.users (id),
  type        shared.notification_type NOT NULL,
  params      jsonb NOT NULL DEFAULT '{}',
  entity_type text,
  entity_id   uuid,
  dedup_key   text,
  read_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_notif_user_unread
  ON notification.notifications (user_id)
  WHERE read_at IS NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notif_user_created
  ON notification.notifications (user_id, created_at DESC)
  WHERE deleted_at IS NULL;

-- Makes the at-least-once consumer safe: redelivering an event cannot duplicate a
-- notification, because the insert uses ON CONFLICT DO NOTHING against this index.
CREATE UNIQUE INDEX IF NOT EXISTS uq_notif_dedup
  ON notification.notifications (user_id, dedup_key)
  WHERE dedup_key IS NOT NULL AND deleted_at IS NULL;

CREATE TRIGGER trg_notif_set_updated BEFORE UPDATE ON notification.notifications
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
