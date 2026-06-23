-- User management queries (Superadmin). All respect soft delete.

-- name: ListUsers :many
SELECT * FROM identity.users
WHERE deleted_at IS NULL
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR email ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountUsers :one
SELECT count(*) FROM identity.users
WHERE deleted_at IS NULL
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR email ILIKE '%' || sqlc.arg(search) || '%'
  );

-- name: UpdateUser :one
UPDATE identity.users
SET name = $2,
    role_id = $3,
    office_id = $4,
    employee_id = $5,
    status = $6
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :execrows
UPDATE identity.users
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
