-- Departments are office-scoped master data (legacy-parity Fase 6 made them
-- per-office). Reads/writes are filtered by the caller's office data scope; legacy
-- departments with a NULL office_id are shared reference data visible to everyone
-- but mutable only by a global-scope caller (enforced in the service layer).

-- name: ListDepartments :many
SELECT * FROM masterdata.departments
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool
       OR office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR office_id IS NULL)
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY name
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountDepartments :one
SELECT count(*) FROM masterdata.departments
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool
       OR office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR office_id IS NULL)
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR code ILIKE '%' || sqlc.arg(search) || '%'
  );

-- name: GetDepartment :one
-- Read visibility includes NULL-office (global) departments.
SELECT * FROM masterdata.departments
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool
       OR office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR office_id IS NULL);

-- name: CreateDepartment :one
INSERT INTO masterdata.departments (name, code, office_id, floor_id, is_active)
VALUES (sqlc.arg(name), sqlc.narg(code), sqlc.narg(office_id), sqlc.narg(floor_id), sqlc.arg(is_active))
RETURNING *;

-- name: UpdateDepartment :one
-- Write scope EXCLUDES NULL-office rows: only a global-scope caller may mutate a
-- shared/global department (a scoped caller can read but not edit it).
UPDATE masterdata.departments
SET name = sqlc.arg(name),
    code = sqlc.narg(code),
    office_id = sqlc.narg(office_id),
    floor_id = sqlc.narg(floor_id),
    is_active = sqlc.arg(is_active)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
RETURNING *;

-- name: SoftDeleteDepartment :execrows
UPDATE masterdata.departments SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]));
