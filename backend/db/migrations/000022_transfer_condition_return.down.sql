ALTER TABLE transfer.asset_transfers
  DROP COLUMN IF EXISTS condition_sent,
  DROP COLUMN IF EXISTS transfer_date,
  DROP COLUMN IF EXISTS return_note;

DROP TYPE IF EXISTS shared.transfer_condition;

-- NOTE: PostgreSQL cannot remove a value from an enum; the 'returned' value on
-- shared.transfer_status intentionally survives the down migration (harmless).
