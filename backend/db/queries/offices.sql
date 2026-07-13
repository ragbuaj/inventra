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

-- name: GetOfficeByCode :one
-- Fresh, side-effect-free existence check used by the office importer's
-- Execute anti-poisoning pre-check (mirrors GetEmployeeByCode).
SELECT * FROM masterdata.offices WHERE code = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListOfficeTypesLookup :many
-- Flat id/name lookup for the office importer's "tipe" column. office_types
-- has no code column (only name), so the importer matches by name only.
SELECT id, name FROM masterdata.office_types WHERE deleted_at IS NULL;

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

-- name: ListOfficesMap :many
-- Geo-enriched, scoped office list for the Peta Lokasi screen: resolves
-- office-type/province/city names + a per-office (non-deleted) asset count.
SELECT
  o.id, o.name, o.code, o.address, o.latitude, o.longitude,
  ot.name AS office_type_name,
  ot.tier AS tier,
  p.name  AS province_name,
  c.name  AS city_name,
  (SELECT count(*) FROM asset.assets a
     WHERE a.office_id = o.id AND a.deleted_at IS NULL) AS asset_count
FROM masterdata.offices o
LEFT JOIN masterdata.office_types ot ON ot.id = o.office_type_id AND ot.deleted_at IS NULL
LEFT JOIN masterdata.provinces    p  ON p.id  = o.province_id    AND p.deleted_at IS NULL
LEFT JOIN masterdata.cities       c  ON c.id  = o.city_id        AND c.deleted_at IS NULL
WHERE o.deleted_at IS NULL
  AND o.is_active = true
  AND (sqlc.arg(all_scope)::bool OR o.id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY o.name;

-- name: ListOfficesTree :many
-- Full scoped office set (no pagination) for building the office hierarchy tree
-- client-side. Mirrors ListOffices' scope filter but without LIMIT/OFFSET/search.
SELECT * FROM masterdata.offices
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY name;

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
