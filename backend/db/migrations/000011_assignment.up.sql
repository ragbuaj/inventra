-- Assignment / check-out / check-in — `assignment` schema. See docs/DATABASE.md §4.4 and PRD §3.3.

CREATE SCHEMA IF NOT EXISTS assignment;

CREATE TABLE assignment.assignments (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id       uuid NOT NULL REFERENCES asset.assets (id),
  employee_id    uuid NOT NULL REFERENCES masterdata.employees (id),
  assigned_by_id uuid NOT NULL REFERENCES identity.users (id),
  checkout_date  timestamptz NOT NULL DEFAULT now(),
  due_date       date,
  checkin_date   timestamptz,
  condition_out  text,
  condition_in   text,
  status         shared.assignment_status NOT NULL DEFAULT 'active',
  notes          text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
-- At most one active assignment per asset.
CREATE UNIQUE INDEX uq_assignments_active_asset ON assignment.assignments (asset_id)
  WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX idx_assignments_asset_id ON assignment.assignments (asset_id);
CREATE INDEX idx_assignments_employee_id ON assignment.assignments (employee_id);
CREATE INDEX idx_assignments_status ON assignment.assignments (status);
CREATE INDEX idx_assignments_assigned_by ON assignment.assignments (assigned_by_id);
CREATE TRIGGER trg_assignments_set_updated BEFORE UPDATE ON assignment.assignments
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
