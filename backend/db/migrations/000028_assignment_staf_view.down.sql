DELETE FROM identity.role_permissions
WHERE permission_key = 'assignment.view'
  AND role_id IN (
    SELECT id FROM identity.roles WHERE deleted_at IS NULL AND name = 'Staf'
  );
