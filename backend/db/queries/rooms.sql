-- Rooms (within a floor). Scope is derived from the room's floor -> office.

-- name: ListRoomsByFloor :many
SELECT * FROM masterdata.rooms
WHERE deleted_at IS NULL AND floor_id = sqlc.arg(floor_id)
  AND (sqlc.arg(search)::text = '' OR name ILIKE '%' || sqlc.arg(search) || '%' OR coalesce(code, '') ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY name
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountRoomsByFloor :one
SELECT count(*) FROM masterdata.rooms
WHERE deleted_at IS NULL AND floor_id = sqlc.arg(floor_id)
  AND (sqlc.arg(search)::text = '' OR name ILIKE '%' || sqlc.arg(search) || '%' OR coalesce(code, '') ILIKE '%' || sqlc.arg(search) || '%');

-- name: ListRoomsLookup :many
-- Flat room lookup (id, name, code, office_id) for the asset importer, scoped
-- to the caller's offices via the room's floor -> office chain. all_scope
-- bypasses the office filter; otherwise only rooms whose office is in office_ids
-- are returned.
SELECT r.id, r.name, r.code, f.office_id
FROM masterdata.rooms r
JOIN masterdata.floors f ON f.id = r.floor_id AND f.deleted_at IS NULL
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR f.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY r.name;

-- name: GetRoom :one
SELECT r.* FROM masterdata.rooms r
JOIN masterdata.floors f ON f.id = r.floor_id
WHERE r.id = sqlc.arg(id) AND r.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR f.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: CreateRoom :one
INSERT INTO masterdata.rooms (floor_id, name, code)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateRoom :one
UPDATE masterdata.rooms
SET floor_id = sqlc.arg(floor_id), name = sqlc.arg(name), code = sqlc.narg(code)
WHERE rooms.id = sqlc.arg(id) AND rooms.deleted_at IS NULL
  AND (
    sqlc.arg(all_scope)::bool
    OR (SELECT f.office_id FROM masterdata.floors f WHERE f.id = rooms.floor_id) = ANY(sqlc.arg(office_ids)::uuid[])
  )
RETURNING *;

-- name: SoftDeleteRoom :execrows
UPDATE masterdata.rooms SET deleted_at = now()
WHERE rooms.id = sqlc.arg(id) AND rooms.deleted_at IS NULL
  AND (
    sqlc.arg(all_scope)::bool
    OR (SELECT f.office_id FROM masterdata.floors f WHERE f.id = rooms.floor_id) = ANY(sqlc.arg(office_ids)::uuid[])
  );
