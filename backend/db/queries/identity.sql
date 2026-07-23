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

-- name: GetUserByLogin :one
-- Login lookup: match by email (citext, case-insensitive) OR username (NIP).
SELECT * FROM identity.users
WHERE (email = sqlc.arg(identifier)::citext OR username = sqlc.arg(identifier))
  AND deleted_at IS NULL;

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

-- name: UpdateUserPassword :exec
UPDATE identity.users
SET password_hash = $2, password_changed_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateUserName :one
UPDATE identity.users SET name = $2 WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserEmail :one
UPDATE identity.users SET email = $2 WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserAvatarKey :exec
UPDATE identity.users SET avatar_key = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserProfile :one
SELECT u.id, u.name, u.email, u.role_id, u.office_id, u.employee_id, u.status,
       u.avatar_key, u.google_id, u.created_at,
       e.phone AS employee_phone,
       r.name  AS role_name,
       o.name  AS office_name,
       e.name  AS employee_name,
       e.code  AS employee_code,
       e.status AS employee_status,
       d.name  AS department_name,
       p.name  AS position_name
FROM identity.users u
LEFT JOIN masterdata.employees   e ON e.id = u.employee_id     AND e.deleted_at IS NULL
LEFT JOIN identity.roles         r ON r.id = u.role_id         AND r.deleted_at IS NULL
LEFT JOIN masterdata.offices     o ON o.id = u.office_id       AND o.deleted_at IS NULL
LEFT JOIN masterdata.departments d ON d.id = e.department_id   AND d.deleted_at IS NULL
LEFT JOIN masterdata.positions   p ON p.id = e.position_id     AND p.deleted_at IS NULL
WHERE u.id = $1 AND u.deleted_at IS NULL;
