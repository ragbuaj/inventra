-- Transfer condition + planned date + returned state (spec 2026-07-05, decisions #1/#2).
CREATE TYPE shared.transfer_condition AS ENUM ('baik', 'rusak_ringan', 'rusak_berat');

-- Postgres >= 12 allows ADD VALUE inside the migration transaction as long as the
-- new value is not used in the same transaction (it isn't — only later requests use it).
ALTER TYPE shared.transfer_status ADD VALUE IF NOT EXISTS 'returned';

ALTER TABLE transfer.asset_transfers
  ADD COLUMN condition_sent shared.transfer_condition,
  ADD COLUMN transfer_date  date,
  ADD COLUMN return_note    text;
