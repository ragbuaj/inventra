-- Employees (asset custodians) with data-scoping by office.

-- name: ListEmployees :many
SELECT * FROM masterdata.employees
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY name
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountEmployees :one
SELECT count(*) FROM masterdata.employees
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR code ILIKE '%' || sqlc.arg(search) || '%'
  );

-- name: GetEmployee :one
SELECT * FROM masterdata.employees
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: CreateEmployee :one
INSERT INTO masterdata.employees (
  code, name, email, avatar_key, department_id, position_id, office_id, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateEmployee :one
UPDATE masterdata.employees
SET code = sqlc.arg(code),
    name = sqlc.arg(name),
    email = sqlc.narg(email),
    avatar_key = sqlc.narg(avatar_key),
    department_id = sqlc.narg(department_id),
    position_id = sqlc.narg(position_id),
    office_id = sqlc.arg(office_id),
    status = sqlc.arg(status)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
RETURNING *;

-- name: SoftDeleteEmployee :execrows
UPDATE masterdata.employees SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]));
