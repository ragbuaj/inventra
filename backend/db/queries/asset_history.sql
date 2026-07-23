-- Asset location + PIC history (spec 2026-07-23 legacy-parity, Fase 3).

-- name: InsertAssetLocationHistory :exec
INSERT INTO asset.asset_location_history (
  asset_id, office_id, floor_id, room_id, source, moved_by_id, transfer_id, note
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(office_id), sqlc.arg(floor_id), sqlc.arg(room_id),
  sqlc.arg(source), sqlc.arg(moved_by_id), sqlc.arg(transfer_id), sqlc.arg(note)
);

-- name: ListAssetLocationHistory :many
SELECT h.id, h.asset_id, h.office_id, h.floor_id, h.room_id, h.source, h.moved_at,
       h.moved_by_id, h.transfer_id, h.note,
       o.name AS office_name, f.name AS floor_name, r.name AS room_name, u.name AS moved_by_name
FROM asset.asset_location_history h
JOIN masterdata.offices o ON o.id = h.office_id
LEFT JOIN masterdata.floors f ON f.id = h.floor_id
LEFT JOIN masterdata.rooms r ON r.id = h.room_id
LEFT JOIN identity.users u ON u.id = h.moved_by_id
WHERE h.asset_id = $1 AND h.deleted_at IS NULL
ORDER BY h.moved_at DESC, h.created_at DESC;

-- name: CloseActivePIC :exec
UPDATE asset.asset_pic_history SET released_at = now()
WHERE asset_id = $1 AND released_at IS NULL AND deleted_at IS NULL;

-- name: InsertAssetPICHistory :exec
INSERT INTO asset.asset_pic_history (asset_id, pic_employee_id, assigned_by_id, note)
VALUES (sqlc.arg(asset_id), sqlc.arg(pic_employee_id), sqlc.arg(assigned_by_id), sqlc.arg(note));

-- name: ListAssetPICHistory :many
SELECT h.id, h.asset_id, h.pic_employee_id, h.assigned_at, h.released_at,
       h.assigned_by_id, h.note,
       e.name AS pic_name, e.code AS pic_code, u.name AS assigned_by_name
FROM asset.asset_pic_history h
JOIN masterdata.employees e ON e.id = h.pic_employee_id
LEFT JOIN identity.users u ON u.id = h.assigned_by_id
WHERE h.asset_id = $1 AND h.deleted_at IS NULL
ORDER BY h.assigned_at DESC, h.created_at DESC;
