-- Maintenance — `maintenance` schema. See docs/DATABASE.md §4.4 and PRD §3.4.

CREATE SCHEMA IF NOT EXISTS maintenance;

CREATE TABLE maintenance.maintenance_schedules (
  id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id                uuid NOT NULL REFERENCES asset.assets (id),
  maintenance_category_id uuid REFERENCES masterdata.maintenance_categories (id),
  interval_months         int NOT NULL,
  last_done_date          date,
  next_due_date           date,
  is_active               boolean NOT NULL DEFAULT true,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  deleted_at              timestamptz
);
CREATE INDEX idx_msched_asset_id ON maintenance.maintenance_schedules (asset_id);
CREATE INDEX idx_msched_category_id ON maintenance.maintenance_schedules (maintenance_category_id);
CREATE INDEX idx_msched_next_due ON maintenance.maintenance_schedules (next_due_date);
CREATE TRIGGER trg_msched_set_updated BEFORE UPDATE ON maintenance.maintenance_schedules
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE maintenance.maintenance_records (
  id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id                uuid NOT NULL REFERENCES asset.assets (id),
  maintenance_category_id uuid REFERENCES masterdata.maintenance_categories (id),
  problem_category_id     uuid REFERENCES masterdata.problem_categories (id),
  type                    shared.maintenance_type NOT NULL,
  status                  shared.maintenance_status NOT NULL DEFAULT 'scheduled',
  scheduled_date          date,
  completed_date          date,
  cost                    numeric(18,2),
  vendor_id               uuid REFERENCES masterdata.vendors (id),
  performed_by            text,
  description             text NOT NULL,
  reported_by_id          uuid REFERENCES identity.users (id),
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  deleted_at              timestamptz
);
CREATE INDEX idx_mrec_asset_id ON maintenance.maintenance_records (asset_id);
CREATE INDEX idx_mrec_status ON maintenance.maintenance_records (status);
CREATE INDEX idx_mrec_category_id ON maintenance.maintenance_records (maintenance_category_id);
CREATE INDEX idx_mrec_problem_id ON maintenance.maintenance_records (problem_category_id);
CREATE INDEX idx_mrec_vendor_id ON maintenance.maintenance_records (vendor_id);
CREATE INDEX idx_mrec_reported_by ON maintenance.maintenance_records (reported_by_id);
CREATE TRIGGER trg_mrec_set_updated BEFORE UPDATE ON maintenance.maintenance_records
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
