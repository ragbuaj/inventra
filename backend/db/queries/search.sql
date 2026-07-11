-- Global search (command palette). Each query returns the top matches for one
-- entity plus the full match count via a window function. Callers gate by
-- permission + data scope; queries only enforce the office scope filter.

-- name: SearchAssets :many
SELECT id, name, asset_tag, status, count(*) OVER()::bigint AS total
FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (name ILIKE '%' || sqlc.arg(q) || '%'
       OR asset_tag ILIKE '%' || sqlc.arg(q) || '%'
       OR serial_number ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchEmployees :many
SELECT id, name, code, count(*) OVER()::bigint AS total
FROM masterdata.employees
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (name ILIKE '%' || sqlc.arg(q) || '%' OR code ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchOffices :many
SELECT id, name, code, count(*) OVER()::bigint AS total
FROM masterdata.offices
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (name ILIKE '%' || sqlc.arg(q) || '%' OR code ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchUsers :many
SELECT id, name, email, count(*) OVER()::bigint AS total
FROM identity.users
WHERE deleted_at IS NULL
  AND (name ILIKE '%' || sqlc.arg(q) || '%' OR email ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchRequests :many
SELECT r.id, r.type, r.status, o.name AS office_name, count(*) OVER()::bigint AS total
FROM approval.requests r
LEFT JOIN masterdata.offices o ON o.id = r.office_id AND o.deleted_at IS NULL
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR r.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (r.reason ILIKE '%' || sqlc.arg(q) || '%' OR r.id::text ILIKE sqlc.arg(q) || '%')
ORDER BY r.created_at DESC
LIMIT sqlc.arg(lim);
