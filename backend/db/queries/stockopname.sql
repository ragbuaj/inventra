-- name: CreateOpnameSession :one
INSERT INTO stockopname.stock_opname_sessions (office_id, name, period, started_by_id)
VALUES (sqlc.arg(office_id), sqlc.narg(name), sqlc.arg(period), sqlc.arg(started_by_id))
RETURNING *;

-- name: SnapshotSessionItems :exec
INSERT INTO stockopname.stock_opname_items (session_id, asset_id, expected, result)
SELECT sqlc.arg(session_id), a.id, true, 'pending'
FROM asset.assets a
WHERE a.office_id = sqlc.arg(office_id)
  AND a.status <> 'disposed'
  AND a.deleted_at IS NULL;

-- name: GetOpnameSession :one
SELECT sqlc.embed(s), o.name AS office_name,
       su.name AS started_by_name, cu.name AS closed_by_name
FROM stockopname.stock_opname_sessions s
LEFT JOIN masterdata.offices o ON o.id = s.office_id AND o.deleted_at IS NULL
LEFT JOIN identity.users su ON su.id = s.started_by_id AND su.deleted_at IS NULL
LEFT JOIN identity.users cu ON cu.id = s.closed_by_id AND cu.deleted_at IS NULL
WHERE s.id = sqlc.arg(id) AND s.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListOpnameSessions :many
SELECT sqlc.embed(s), o.name AS office_name
FROM stockopname.stock_opname_sessions s
LEFT JOIN masterdata.offices o ON o.id = s.office_id AND o.deleted_at IS NULL
WHERE s.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.opname_session_status IS NULL OR s.status = sqlc.narg(status))
ORDER BY s.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountOpnameSessions :one
SELECT count(*)
FROM stockopname.stock_opname_sessions s
WHERE s.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR s.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.opname_session_status IS NULL OR s.status = sqlc.narg(status));

-- name: SetSessionStatus :one
UPDATE stockopname.stock_opname_sessions
SET status = sqlc.arg(status),
    closed_by_id = sqlc.narg(closed_by_id),
    closed_at = sqlc.narg(closed_at)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SessionKpis :one
SELECT
  count(*)::bigint AS total,
  count(*) FILTER (WHERE result = 'found')::bigint AS found,
  count(*) FILTER (WHERE result = 'pending')::bigint AS pending,
  count(*) FILTER (WHERE result IN ('not_found','damaged','misplaced'))::bigint AS variance
FROM stockopname.stock_opname_items
WHERE session_id = sqlc.arg(session_id) AND deleted_at IS NULL;

-- name: ListOpnameItemsEnriched :many
SELECT sqlc.embed(it), a.name AS asset_name, a.asset_tag AS asset_tag,
       o.name AS office_name, rm.name AS room_name, fl.name AS floor_name,
       cu.name AS counted_by_name
FROM stockopname.stock_opname_items it
LEFT JOIN asset.assets a ON a.id = it.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.rooms rm ON rm.id = a.room_id AND rm.deleted_at IS NULL
LEFT JOIN masterdata.floors fl ON fl.id = rm.floor_id AND fl.deleted_at IS NULL
LEFT JOIN identity.users cu ON cu.id = it.counted_by_id AND cu.deleted_at IS NULL
WHERE it.session_id = sqlc.arg(session_id) AND it.deleted_at IS NULL
  AND (sqlc.narg(result)::shared.opname_item_result IS NULL OR it.result = sqlc.narg(result))
ORDER BY a.name;

-- name: GetOpnameItem :one
SELECT * FROM stockopname.stock_opname_items
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id) AND deleted_at IS NULL;

-- name: SetOpnameItemResult :one
UPDATE stockopname.stock_opname_items
SET result = sqlc.arg(result), note = sqlc.narg(note),
    counted_by_id = sqlc.arg(counted_by_id), counted_at = now()
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id) AND deleted_at IS NULL
RETURNING *;

-- name: SetItemFollowup :one
UPDATE stockopname.stock_opname_items
SET followup_request_id = sqlc.arg(followup_request_id)
WHERE id = sqlc.arg(id) AND session_id = sqlc.arg(session_id) AND deleted_at IS NULL
RETURNING *;

-- name: GetOpnameItemByTag :one
SELECT it.* FROM stockopname.stock_opname_items it
JOIN asset.assets a ON a.id = it.asset_id
WHERE it.session_id = sqlc.arg(session_id) AND it.deleted_at IS NULL
  AND a.asset_tag = sqlc.arg(asset_tag);

-- (scan reuses assets.sql GetAssetByTag; scope enforced in the service)

-- NOTE: :one + ON CONFLICT DO NOTHING → a conflict returns pgx.ErrNoRows (no row inserted); the caller treats that as "already present".
-- name: InsertUnexpectedItem :one
INSERT INTO stockopname.stock_opname_items (session_id, asset_id, expected, result)
VALUES (sqlc.arg(session_id), sqlc.arg(asset_id), false, 'pending')
ON CONFLICT (session_id, asset_id) WHERE deleted_at IS NULL DO NOTHING
RETURNING *;
