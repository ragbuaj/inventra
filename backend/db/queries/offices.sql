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
SELECT * FROM masterdata.offices
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: CreateOffice :one
INSERT INTO masterdata.offices (
  parent_id, office_type_id, province_id, city_id, name, code, address, is_active, latitude, longitude
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateOffice :one
UPDATE masterdata.offices
SET parent_id = sqlc.narg(parent_id),
    office_type_id = sqlc.arg(office_type_id),
    province_id = sqlc.narg(province_id),
    city_id = sqlc.narg(city_id),
    name = sqlc.arg(name),
    code = sqlc.arg(code),
    address = sqlc.narg(address),
    is_active = sqlc.arg(is_active),
    latitude = sqlc.narg(latitude),
    longitude = sqlc.narg(longitude)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]))
RETURNING *;

-- name: SoftDeleteOffice :execrows
UPDATE masterdata.offices SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: GetOfficeAncestors :many
WITH RECURSIVE anc AS (
  SELECT o.id, o.parent_id, o.office_type_id
  FROM masterdata.offices o WHERE o.id = $1 AND o.deleted_at IS NULL
  UNION ALL
  SELECT o.id, o.parent_id, o.office_type_id
  FROM masterdata.offices o
  JOIN anc a ON o.id = a.parent_id
  WHERE o.deleted_at IS NULL
)
SELECT anc.id, anc.parent_id, ot.tier
FROM anc JOIN masterdata.office_types ot ON ot.id = anc.office_type_id;
