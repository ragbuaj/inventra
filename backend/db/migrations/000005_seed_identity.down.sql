-- Remove seeded rows for built-in roles.
DELETE FROM identity.role_permissions
WHERE role_id IN (SELECT id FROM identity.roles WHERE is_system);

DELETE FROM identity.data_scope_policies
WHERE role_id IN (SELECT id FROM identity.roles WHERE is_system);

DELETE FROM identity.roles WHERE is_system;
