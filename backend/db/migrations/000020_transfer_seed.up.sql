-- Migration 000020: seed data for the asset transfer (mutasi) module.
-- Tables already exist (000015_fam_tables); this seeds approval bands, permissions,
-- and data-scope so the transfer endpoints are usable. See
-- docs/superpowers/specs/2026-07-02-asset-transfer-mutasi-design.md.

-- Approval thresholds for asset_transfer (placeholder bands, mirror asset_disposal).
-- Unique constraint: (request_type, amount_from, step_order).
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order) VALUES
  ('asset_transfer', 0,         50000000, 'office',  1),
  ('asset_transfer', 50000000,  NULL,     'office',  1),
  ('asset_transfer', 50000000,  NULL,     'wilayah', 2)
ON CONFLICT DO NOTHING;

-- Permissions: transfer.manage (submit/ship/receive) + transfer.view (read).
-- Superadmin via '*'; operational roles get both; Staf gets neither (cannot mutate/see).
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('transfer.manage'), ('transfer.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Data-scope for the 'transfers' module (mirror 'assets' from 000016).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'transfers', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
