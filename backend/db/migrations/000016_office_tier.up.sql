-- Migration 000016: Add office_types.tier + approval/asset feature seeds.
-- See docs/DATABASE.md §6 (planned migrations) and docs/PRD.md §2.4 (value-tiered approval).

-- Add explicit office tier (reuses shared.approver_level; cabang & outlet => 'office').
ALTER TABLE masterdata.office_types ADD COLUMN tier shared.approver_level;

-- Backfill tier for seeded office types (idempotent by name).
-- NOTE: no office_types rows are seeded in earlier migrations; these UPDATEs affect 0 rows
-- in the current dev DB but will correctly backfill any manually-inserted rows.
UPDATE masterdata.office_types SET tier = 'pusat'   WHERE name ILIKE '%pusat%'   AND deleted_at IS NULL;
UPDATE masterdata.office_types SET tier = 'wilayah' WHERE name ILIKE '%wilayah%' AND deleted_at IS NULL;
UPDATE masterdata.office_types SET tier = 'office'  WHERE (name ILIKE '%cabang%' OR name ILIKE '%unit%' OR name ILIKE '%outlet%') AND deleted_at IS NULL;

-- Approval thresholds (placeholder bands per PRD §2.4 — confirm with bank policy).
-- Unique constraint: (request_type, amount_from, step_order).
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order) VALUES
  ('asset_create',        0,          10000000,  'office',  1),
  ('asset_create',        10000000,   100000000, 'office',  1),
  ('asset_create',        10000000,   100000000, 'wilayah', 2),
  ('asset_create',        100000000,  NULL,      'office',  1),
  ('asset_create',        100000000,  NULL,      'wilayah', 2),
  ('asset_create',        100000000,  NULL,      'pusat',   3),
  ('asset_disposal',      0,          5000000,   'office',  1),
  ('asset_disposal',      5000000,    50000000,  'office',  1),
  ('asset_disposal',      5000000,    50000000,  'wilayah', 2),
  ('asset_disposal',      50000000,   NULL,      'office',  1),
  ('asset_disposal',      50000000,   NULL,      'wilayah', 2),
  ('asset_disposal',      50000000,   NULL,      'pusat',   3),
  ('valuation_exclusion', 0,          NULL,      'wilayah', 1);

-- Permissions: grant new action keys to roles.
-- Schema: identity.role_permissions (role_id uuid, permission_key text).
-- Existing seeds (000005) use keys like asset.read/asset.create/asset.update; these are new keys.
-- Grant matrix per brief:
--   Superadmin:                  all 5 keys
--   Manager, Kepala Kanwil, Kepala Unit: request.decide, asset.view, asset.manage
--   Staf:                        request.create, asset.view
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES
  ('request.create'),
  ('request.decide'),
  ('approval.config.manage'),
  ('asset.view'),
  ('asset.manage')
) AS p(key)
WHERE r.deleted_at IS NULL
  AND (
    (r.name = 'Superadmin')
    OR (r.name IN ('Manager', 'Kepala Kanwil', 'Kepala Unit') AND p.key IN ('request.decide', 'asset.view', 'asset.manage'))
    OR (r.name = 'Staf' AND p.key IN ('request.create', 'asset.view'))
  )
ON CONFLICT DO NOTHING;

-- Data-scope policies for new modules (assets, requests).
-- Existing seeds (000005) only seed module='*' (default). These add per-module overrides.
-- Staf scope on 'requests' is 'own'; for 'assets' falls through to ELSE => 'office'.
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, m.module, (CASE
    WHEN r.name = 'Superadmin'                                       THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager')       THEN 'office_subtree'
    WHEN r.name = 'Staf' AND m.module = 'requests'                   THEN 'own'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
CROSS JOIN (VALUES ('assets'), ('requests')) AS m(module)
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- Field permissions: restrict cost/value visibility on assets entity.
-- Fields: purchase_cost, book_value, accumulated_depreciation.
-- can_view: true only for the roles listed per field; can_edit: false (read-only mask).
INSERT INTO identity.field_permissions (entity, field, role_id, can_view, can_edit)
SELECT 'assets', f.field, r.id,
       (CASE
          WHEN f.field = 'purchase_cost'            AND r.name IN ('Superadmin', 'Manager') THEN true
          WHEN f.field = 'book_value'               AND r.name IN ('Superadmin', 'Manager') THEN true
          WHEN f.field = 'accumulated_depreciation' AND r.name = 'Superadmin'              THEN true
          ELSE false
        END),
       false
FROM identity.roles r
CROSS JOIN (VALUES ('purchase_cost'), ('book_value'), ('accumulated_depreciation')) AS f(field)
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
