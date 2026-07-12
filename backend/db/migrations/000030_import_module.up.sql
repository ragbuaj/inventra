-- Import module: batch approval for assets, per-row detail, job routing office.
-- See docs/superpowers/specs/2026-07-12-import-module-design.md.

ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'validated';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'confirmed';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'executing';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'awaiting_approval';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'cancelled';

ALTER TYPE shared.request_type ADD VALUE IF NOT EXISTS 'asset_import';

ALTER TABLE import.import_jobs
  ADD COLUMN IF NOT EXISTS office_id     uuid REFERENCES masterdata.offices (id),
  ADD COLUMN IF NOT EXISTS request_id    uuid REFERENCES approval.requests (id),
  ADD COLUMN IF NOT EXISTS confirmed_at  timestamptz,
  ADD COLUMN IF NOT EXISTS error_key     text;

CREATE INDEX IF NOT EXISTS idx_import_office ON import.import_jobs (office_id);

CREATE TABLE IF NOT EXISTS import.import_rows (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id      uuid NOT NULL REFERENCES import.import_jobs (id) ON DELETE CASCADE,
  row_no      int  NOT NULL,
  data        jsonb NOT NULL DEFAULT '{}',
  valid       boolean NOT NULL DEFAULT false,
  errors      jsonb NOT NULL DEFAULT '[]',
  result_ref  text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_import_rows_job_rowno
  ON import.import_rows (job_id, row_no) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_import_rows_job_valid ON import.import_rows (job_id, valid);
CREATE TRIGGER trg_import_rows_set_updated BEFORE UPDATE ON import.import_rows
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
