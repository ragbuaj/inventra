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
