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

-- name: GetEmployeeByCode :one
SELECT * FROM masterdata.employees WHERE code = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListEmployeeCodes :many
-- All existing (non-deleted) employee codes, used by the employee importer to
-- detect collisions with user-supplied kode values during validation. Employee
-- codes are globally unique (uq_employees_code), so this set is deliberately
-- unscoped.
SELECT code FROM masterdata.employees WHERE deleted_at IS NULL;

-- name: CreateEmployee :one
INSERT INTO masterdata.employees (
  code, name, email, phone, avatar_key, department_id, position_id, office_id, status,
  company_id, executor_division_id
) VALUES (
  sqlc.arg(code), sqlc.arg(name), sqlc.narg(email), sqlc.narg(phone), sqlc.narg(avatar_key),
  sqlc.narg(department_id), sqlc.narg(position_id), sqlc.arg(office_id), sqlc.arg(status),
  sqlc.narg(company_id), sqlc.narg(executor_division_id)
)
RETURNING *;

-- name: UpdateEmployee :one
UPDATE masterdata.employees
SET code = sqlc.arg(code),
    name = sqlc.arg(name),
    email = sqlc.narg(email),
    phone = sqlc.narg(phone),
    avatar_key = sqlc.narg(avatar_key),
    department_id = sqlc.narg(department_id),
    position_id = sqlc.narg(position_id),
    office_id = sqlc.arg(office_id),
    status = sqlc.arg(status),
    company_id = sqlc.narg(company_id),
    executor_division_id = sqlc.narg(executor_division_id)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
RETURNING *;

-- name: GetDepartmentOffice :one
-- Returns a department's office_id (NULL for legacy global departments), used to
-- validate that an employee's department belongs to the employee's office.
SELECT office_id FROM masterdata.departments WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteEmployee :execrows
UPDATE masterdata.employees SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: UpdateEmployeePhone :exec
UPDATE masterdata.employees SET phone = $2 WHERE id = $1 AND deleted_at IS NULL;
