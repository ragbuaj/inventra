-- Revert 000032: remove the 'masterdata.employee.manage' grants seeded for
-- superadmin and kepala_kanwil.
DELETE FROM identity.role_permissions
WHERE permission_key = 'masterdata.employee.manage'
  AND role_id IN (
    SELECT id FROM identity.roles WHERE code IN ('superadmin', 'kepala_kanwil')
  );
