-- name: CreateTransfer :one
INSERT INTO transfer.asset_transfers (
  asset_id, from_office_id, to_office_id, to_room_id, status,
  reason, requested_by_id, approved_by_id, request_id, condition_sent, transfer_date
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(from_office_id), sqlc.arg(to_office_id), sqlc.narg(to_room_id),
  'approved', sqlc.narg(reason), sqlc.arg(requested_by_id), sqlc.narg(approved_by_id), sqlc.narg(request_id),
  sqlc.narg(condition_sent), sqlc.narg(transfer_date)
)
RETURNING *;

-- name: GetTransfer :one
-- Scoped: caller must have the from- or to-office in scope. Plain (unenriched)
-- row — used internally by Ship/Receive/RejectReceive, which only need the
-- base columns to validate state + perform the update.
SELECT * FROM transfer.asset_transfers
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: GetTransferEnriched :one
-- Scoped: caller must have the from- or to-office in scope. Adds resolved
-- asset/office/room/actor display names for the detail view. LEFT JOINs keep
-- the row visible (with nil names) even when a joined entity was soft-deleted.
SELECT sqlc.embed(tr),
       a.name     AS asset_name,
       a.asset_tag AS asset_tag,
       fo.name    AS from_office_name,
       tof.name   AS to_office_name,
       rm.name    AS to_room_name,
       ru.name    AS requested_by_name,
       rcu.name   AS received_by_name
FROM transfer.asset_transfers tr
LEFT JOIN asset.assets a        ON a.id  = tr.asset_id        AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices fo ON fo.id = tr.from_office_id  AND fo.deleted_at IS NULL
LEFT JOIN masterdata.offices tof ON tof.id = tr.to_office_id  AND tof.deleted_at IS NULL
LEFT JOIN masterdata.rooms rm   ON rm.id = tr.to_room_id      AND rm.deleted_at IS NULL
LEFT JOIN identity.users ru     ON ru.id = tr.requested_by_id AND ru.deleted_at IS NULL
LEFT JOIN identity.users rcu    ON rcu.id = tr.received_by_id AND rcu.deleted_at IS NULL
WHERE tr.id = sqlc.arg(id) AND tr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR tr.from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR tr.to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListTransfersEnriched :many
SELECT sqlc.embed(tr),
       a.name     AS asset_name,
       a.asset_tag AS asset_tag,
       fo.name    AS from_office_name,
       tof.name   AS to_office_name,
       rm.name    AS to_room_name,
       ru.name    AS requested_by_name,
       rcu.name   AS received_by_name
FROM transfer.asset_transfers tr
LEFT JOIN asset.assets a        ON a.id  = tr.asset_id        AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices fo ON fo.id = tr.from_office_id  AND fo.deleted_at IS NULL
LEFT JOIN masterdata.offices tof ON tof.id = tr.to_office_id  AND tof.deleted_at IS NULL
LEFT JOIN masterdata.rooms rm   ON rm.id = tr.to_room_id      AND rm.deleted_at IS NULL
LEFT JOIN identity.users ru     ON ru.id = tr.requested_by_id AND ru.deleted_at IS NULL
LEFT JOIN identity.users rcu    ON rcu.id = tr.received_by_id AND rcu.deleted_at IS NULL
WHERE tr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR tr.from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR tr.to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.transfer_status IS NULL OR tr.status = sqlc.narg(status))
ORDER BY tr.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountTransfers :one
SELECT count(*) FROM transfer.asset_transfers
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.transfer_status IS NULL OR status = sqlc.narg(status));

-- name: ListTransfersByAssetEnriched :many
-- Per-asset history, scoped by from- or to-office. Same enrichment as
-- GetTransferEnriched/ListTransfersEnriched.
SELECT sqlc.embed(tr),
       a.name     AS asset_name,
       a.asset_tag AS asset_tag,
       fo.name    AS from_office_name,
       tof.name   AS to_office_name,
       rm.name    AS to_room_name,
       ru.name    AS requested_by_name,
       rcu.name   AS received_by_name
FROM transfer.asset_transfers tr
LEFT JOIN asset.assets a        ON a.id  = tr.asset_id        AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices fo ON fo.id = tr.from_office_id  AND fo.deleted_at IS NULL
LEFT JOIN masterdata.offices tof ON tof.id = tr.to_office_id  AND tof.deleted_at IS NULL
LEFT JOIN masterdata.rooms rm   ON rm.id = tr.to_room_id      AND rm.deleted_at IS NULL
LEFT JOIN identity.users ru     ON ru.id = tr.requested_by_id AND ru.deleted_at IS NULL
LEFT JOIN identity.users rcu    ON rcu.id = tr.received_by_id AND rcu.deleted_at IS NULL
WHERE tr.asset_id = sqlc.arg(asset_id) AND tr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR tr.from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR tr.to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY tr.created_at DESC;

-- name: SetTransferShipped :one
UPDATE transfer.asset_transfers
SET status = 'in_transit', shipped_date = sqlc.arg(shipped_date)
WHERE id = sqlc.arg(id) AND status = 'approved' AND deleted_at IS NULL
RETURNING *;

-- name: SetTransferReceived :one
UPDATE transfer.asset_transfers
SET status = 'received',
    received_date = sqlc.arg(received_date),
    received_by_id = sqlc.arg(received_by_id),
    bast_no = sqlc.narg(bast_no),
    to_room_id = COALESCE(sqlc.narg(to_room_id), to_room_id)
WHERE id = sqlc.arg(id) AND status = 'in_transit' AND deleted_at IS NULL
RETURNING *;

-- name: SetTransferReturned :one
-- Receiving side declines the shipment: terminal 'returned', asset never moved.
UPDATE transfer.asset_transfers
SET status = 'returned',
    return_note = sqlc.narg(return_note),
    received_by_id = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'in_transit' AND deleted_at IS NULL
RETURNING *;

-- name: GetOpenTransferForAsset :one
-- Guard: an asset may have at most one non-terminal transfer row.
SELECT * FROM transfer.asset_transfers
WHERE asset_id = sqlc.arg(asset_id) AND deleted_at IS NULL
  AND status IN ('approved', 'in_transit')
LIMIT 1;

-- name: CountPendingTransferRequestsForAsset :one
-- Guard: an asset may have at most one pending asset_transfer approval request.
SELECT count(*) FROM approval.requests
WHERE type = 'asset_transfer' AND target_id = sqlc.arg(asset_id)
  AND status = 'pending' AND deleted_at IS NULL;
