DELETE FROM approval.approval_thresholds WHERE request_type = 'assignment';
DELETE FROM identity.role_permissions
WHERE permission_key = 'assignment.view'
  AND role_id IN (
    SELECT id FROM identity.roles WHERE deleted_at IS NULL AND name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
  );
