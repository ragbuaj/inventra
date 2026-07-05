-- Depreciation period state machine + module seeds. Spec:
-- docs/superpowers/specs/2026-07-05-depreciation-module-design.md · ADR-0010 stage 1.

CREATE TYPE shared.depreciation_period_status AS ENUM ('open', 'computed', 'closed');

CREATE TABLE depreciation.depreciation_periods (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  period        date NOT NULL,
  status        shared.depreciation_period_status NOT NULL DEFAULT 'open',
  computed_at   timestamptz,
  computed_by   uuid REFERENCES identity.users (id),
  closed_at     timestamptz,
  closed_by     uuid REFERENCES identity.users (id),
  asset_count   int NOT NULL DEFAULT 0,
  total_amount  numeric(18,2) NOT NULL DEFAULT 0,
  skipped_count int NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE UNIQUE INDEX uq_depr_period ON depreciation.depreciation_periods (period) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_depr_periods_set_updated BEFORE UPDATE ON depreciation.depreciation_periods
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE INDEX idx_depr_basis_period ON depreciation.depreciation_entries (basis, period);

-- Journal credit account (global; Superadmin-editable later via app_settings CRUD).
INSERT INTO identity.app_settings (key, value, value_type, description)
SELECT 'depreciation.accumulated_gl_account', '1.2.9.001', 'string',
       'GL account credited by the depreciation journal (Akumulasi Penyusutan) — placeholder, confirm with bank COA'
WHERE NOT EXISTS (SELECT 1 FROM identity.app_settings WHERE key = 'depreciation.accumulated_gl_account' AND deleted_at IS NULL);

-- Permissions: Superadmin ONLY (PRD §2.1: konfigurasi & jalankan depresiasi).
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('depreciation.view'), ('depreciation.manage')) AS p(key)
WHERE r.deleted_at IS NULL AND r.name = 'Superadmin'
ON CONFLICT DO NOTHING;

-- Data-scope for module 'depreciation' (mirror 000021 pattern).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'depreciation', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
