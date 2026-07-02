-- Migration 000021: seed data for the asset disposal module (tables already exist,
-- 000015; asset_disposal thresholds already seeded, 000016). See
-- docs/superpowers/specs/2026-07-02-disposal-design.md.

-- Permissions: disposal.manage (submit/BAST) + disposal.view (read).
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('disposal.manage'), ('disposal.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Data-scope for the 'disposals' module (mirror 'transfers' from 000020).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'disposals', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
