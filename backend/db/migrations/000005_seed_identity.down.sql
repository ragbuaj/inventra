-- Remove seeded rows for built-in roles.
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE is_system);

DELETE FROM data_scope_policies
WHERE role_id IN (SELECT id FROM roles WHERE is_system);

DELETE FROM roles WHERE is_system;
