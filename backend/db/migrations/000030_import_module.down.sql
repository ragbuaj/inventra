DROP TABLE IF EXISTS import.import_rows;

DROP INDEX IF EXISTS import.idx_import_office;
ALTER TABLE import.import_jobs
  DROP COLUMN IF EXISTS office_id,
  DROP COLUMN IF EXISTS request_id,
  DROP COLUMN IF EXISTS confirmed_at,
  DROP COLUMN IF EXISTS error_key;

-- NOTE: shared.import_status / shared.request_type enum values are NOT removed
-- (Postgres cannot DROP an enum label). They are inert if unused.
