-- Bulk import jobs — `import` schema. See docs/DATABASE.md §4.5.
-- Tracks CSV/XLSX imports (FR-2.11 / FR-7.5b); source file & error report live in MinIO.

CREATE SCHEMA IF NOT EXISTS import;

CREATE TABLE import.import_jobs (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target           text NOT NULL,
  format           text NOT NULL,
  filename         text NOT NULL,
  object_key       text,
  status           shared.import_status NOT NULL DEFAULT 'pending',
  total_rows       int NOT NULL DEFAULT 0,
  success_rows     int NOT NULL DEFAULT 0,
  failed_rows      int NOT NULL DEFAULT 0,
  error_report_key text,
  created_by_id    uuid NOT NULL REFERENCES identity.users (id),
  finished_at      timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz
);
CREATE INDEX idx_import_created_by ON import.import_jobs (created_by_id);
CREATE INDEX idx_import_status ON import.import_jobs (status);
CREATE TRIGGER trg_import_jobs_set_updated BEFORE UPDATE ON import.import_jobs
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
