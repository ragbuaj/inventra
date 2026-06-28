-- Seed: built-in roles, default data-scope policies, and the default RBAC matrix.
-- Mirrors the seeded defaults described in PRD §2.1 and DATABASE.md §4.1.

-- 1) Built-in (system) roles.
INSERT INTO identity.roles (code, name, description, is_system) VALUES
  ('superadmin',    'Superadmin',    'Akses penuh sistem; kelola user, peran & konfigurasi otorisasi', true),
  ('kepala_kanwil', 'Kepala Kanwil', 'Kepala Kantor Wilayah; lingkup wilayah + seluruh kantor turunannya', true),
  ('kepala_unit',   'Kepala Unit',   'Kepala kantor (Cabang/Outlet); lingkup kantornya', true),
  ('manager',       'Manager',       'Asset manager / operasional aset dalam lingkup kantornya', true),
  ('staf',          'Staf',          'Pengguna aset; hanya data miliknya', true);

-- 2) Default data-scope policies (module '*' = default untuk semua modul).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, '*', v.scope::shared.scope_level
FROM identity.roles r
JOIN (VALUES
  ('superadmin',    'global'),
  ('kepala_kanwil', 'office_subtree'),
  ('kepala_unit',   'office_subtree'),
  ('manager',       'office_subtree'),
  ('staf',          'own')
) AS v(code, scope) ON v.code = r.code;

-- 3) Default RBAC per-action (role_permissions). Keys are the action catalog.
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, v.perm
FROM identity.roles r
JOIN (VALUES
  -- Superadmin: full catalog.
  ('superadmin', 'user.manage'),
  ('superadmin', 'role.manage'),
  ('superadmin', 'scope.manage'),
  ('superadmin', 'fieldperm.manage'),
  ('superadmin', 'audit.view'),
  ('superadmin', 'masterdata.global.manage'),
  ('superadmin', 'masterdata.office.manage'),
  ('superadmin', 'asset.view'),
  ('superadmin', 'asset.manage'),
  ('superadmin', 'request.create'),
  ('superadmin', 'request.decide'),
  ('superadmin', 'approval.config.manage'),
  ('superadmin', 'report.view'),
  ('superadmin', 'report.export'),
  ('superadmin', 'maintenance.manage'),
  ('superadmin', 'depreciation.manage'),
  ('superadmin', 'valuation.exclude.approve'),
  ('superadmin', 'assignment.manage'),
  -- Kepala Kanwil: oversight + approvals within wilayah.
  ('kepala_kanwil', 'masterdata.office.manage'),
  ('kepala_kanwil', 'asset.view'),
  ('kepala_kanwil', 'request.create'),
  ('kepala_kanwil', 'request.decide'),
  ('kepala_kanwil', 'valuation.exclude.approve'),
  ('kepala_kanwil', 'report.view'),
  ('kepala_kanwil', 'report.export'),
  ('kepala_kanwil', 'audit.view'),
  -- Kepala Unit: approvals + reports within unit.
  ('kepala_unit', 'asset.view'),
  ('kepala_unit', 'request.create'),
  ('kepala_unit', 'request.decide'),
  ('kepala_unit', 'report.view'),
  ('kepala_unit', 'report.export'),
  ('kepala_unit', 'audit.view'),
  -- Manager: day-to-day asset operations.
  ('manager', 'asset.view'),
  ('manager', 'asset.manage'),
  ('manager', 'request.create'),
  ('manager', 'request.decide'),
  ('manager', 'maintenance.manage'),
  ('manager', 'assignment.manage'),
  ('manager', 'report.view'),
  ('manager', 'report.export'),
  -- Staf: read own + submit requests.
  ('staf', 'asset.view'),
  ('staf', 'request.create'),
  ('staf', 'report.view')
) AS v(code, perm) ON v.code = r.code;
