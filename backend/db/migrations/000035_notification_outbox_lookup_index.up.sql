-- Backs the sweeper's outbox-side idempotency guard
-- (EnqueueMaintenanceDueOutbox), which asks "has this schedule's reminder for
-- this due date already been enqueued?" once per due schedule, every sweep tick.
--
-- idx_outbox_unpublished cannot serve it: that index is partial on
-- `published_at IS NULL`, and the rows this guard looks for are precisely the
-- ones the relay has already published. Without this index each check falls back
-- to a sequential scan over the whole retention window of the outbox.
CREATE INDEX IF NOT EXISTS idx_outbox_event_aggregate
  ON notification.outbox (event_type, aggregate_id)
  WHERE deleted_at IS NULL;
