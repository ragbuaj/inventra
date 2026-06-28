-- Identity module queries. Schema-qualified (see DATABASE.md §1.2).

-- name: GetRoleByCode :one
SELECT * FROM identity.roles
WHERE code = $1 AND deleted_at IS NULL;

-- name: ListRoles :many
SELECT * FROM identity.roles
WHERE deleted_at IS NULL
ORDER BY name;

-- name: GetUserByID :one
SELECT * FROM identity.users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM identity.users
WHERE email = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO identity.users (name, email, password_hash, role_id, office_id, employee_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListRolePermissions :many
SELECT permission_key FROM identity.role_permissions
WHERE role_id = $1 AND deleted_at IS NULL
ORDER BY permission_key;

-- name: ListDataScopePolicies :many
SELECT * FROM identity.data_scope_policies
WHERE role_id = $1 AND deleted_at IS NULL;

-- name: LinkGoogleID :exec
UPDATE identity.users
SET google_id = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetAppSetting :one
SELECT value FROM identity.app_settings WHERE key = $1 AND deleted_at IS NULL;

-- name: GetRole :one
SELECT * FROM identity.roles WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateRole :one
INSERT INTO identity.roles (code, name, description, is_system)
VALUES ($1, $2, $3, false)
RETURNING *;

-- name: UpdateRole :one
UPDATE identity.roles
SET code = $2, name = $3, description = $4
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteRole :execrows
UPDATE identity.roles SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: CountUsersByRole :one
SELECT count(*) FROM identity.users WHERE role_id = $1 AND deleted_at IS NULL;

-- name: InsertRolePermission :one
INSERT INTO identity.role_permissions (role_id, permission_key)
VALUES ($1, $2)
RETURNING *;

-- name: SoftDeleteRolePermissionsByRole :execrows
UPDATE identity.role_permissions SET deleted_at = now()
WHERE role_id = $1 AND deleted_at IS NULL;

-- name: InsertDataScopePolicy :one
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
VALUES ($1, $2, $3)
RETURNING *;

-- name: SoftDeleteDataScopePoliciesByRole :execrows
UPDATE identity.data_scope_policies SET deleted_at = now()
WHERE role_id = $1 AND deleted_at IS NULL;

-- name: InsertFieldPermission :one
INSERT INTO identity.field_permissions (entity, field, role_id, can_view, can_edit)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: SoftDeleteFieldPermissionsByRole :execrows
UPDATE identity.field_permissions SET deleted_at = now()
WHERE role_id = $1 AND deleted_at IS NULL;
