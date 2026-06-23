-- Offices (hierarchy) with data-scoping. all_scope bypasses the office filter
-- (global scope); otherwise only offices whose id is in office_ids are returned.

-- name: ListOffices :many
SELECT * FROM masterdata.offices
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY name
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountOffices :one
SELECT count(*) FROM masterdata.offices
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR code ILIKE '%' || sqlc.arg(search) || '%'
  );

-- name: GetOffice :one
SELECT * FROM masterdata.offices WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateOffice :one
INSERT INTO masterdata.offices (
  parent_id, office_type_id, province_id, city_id, name, code, address, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateOffice :one
UPDATE masterdata.offices
SET parent_id = $2, office_type_id = $3, province_id = $4, city_id = $5,
    name = $6, code = $7, address = $8, is_active = $9
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteOffice :execrows
UPDATE masterdata.offices SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
