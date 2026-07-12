-- Migration 000032: Seed identity.role_permissions for 'masterdata.employee.manage'.
--
-- The bulk-import module gates employee batch imports behind
-- 'masterdata.employee.manage' (see internal/importer/service.go
-- PermissionKey), but no migration ever granted this key to any role —
-- including Superadmin ('full catalog' in 000005_seed_identity.up.sql predates
-- this permission key) — so employee import was unusable by anyone. Mirror the
-- roles that already hold 'masterdata.office.manage' (which gates employee
-- CRUD's plain page, per master/import.vue's PERMISSION_BY_TARGET comment):
-- superadmin and kepala_kanwil.
--
-- ON CONFLICT guards idempotency against the unique index on
-- (role_id, permission_key) WHERE deleted_at IS NULL.
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, 'masterdata.employee.manage'
FROM identity.roles r
WHERE r.code IN ('superadmin', 'kepala_kanwil')
ON CONFLICT (role_id, permission_key) WHERE deleted_at IS NULL DO NOTHING;
