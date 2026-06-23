-- Floors (within an office). Listed per office; single-row ops carry the
-- office scope (all_scope OR office_id = ANY(office_ids)).

-- name: ListFloorsByOffice :many
SELECT * FROM masterdata.floors
WHERE deleted_at IS NULL AND office_id = sqlc.arg(office_id)
  AND (sqlc.arg(search)::text = '' OR name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY name
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountFloorsByOffice :one
SELECT count(*) FROM masterdata.floors
WHERE deleted_at IS NULL AND office_id = sqlc.arg(office_id)
  AND (sqlc.arg(search)::text = '' OR name ILIKE '%' || sqlc.arg(search) || '%');

-- name: GetFloor :one
SELECT * FROM masterdata.floors
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: CreateFloor :one
INSERT INTO masterdata.floors (office_id, name, level)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateFloor :one
UPDATE masterdata.floors
SET office_id = sqlc.arg(office_id), name = sqlc.arg(name), level = sqlc.narg(level)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
RETURNING *;

-- name: SoftDeleteFloor :execrows
UPDATE masterdata.floors SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR office_id = ANY(sqlc.arg(office_ids)::uuid[]));
