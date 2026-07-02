-- name: CreateTransfer :one
INSERT INTO transfer.asset_transfers (
  asset_id, from_office_id, to_office_id, to_room_id, status,
  reason, requested_by_id, approved_by_id, request_id
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(from_office_id), sqlc.arg(to_office_id), sqlc.narg(to_room_id),
  'approved', sqlc.narg(reason), sqlc.arg(requested_by_id), sqlc.narg(approved_by_id), sqlc.narg(request_id)
)
RETURNING *;

-- name: GetTransfer :one
-- Scoped: caller must have the from- or to-office in scope.
SELECT * FROM transfer.asset_transfers
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListTransfers :many
SELECT * FROM transfer.asset_transfers
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.transfer_status IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountTransfers :one
SELECT count(*) FROM transfer.asset_transfers
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.transfer_status IS NULL OR status = sqlc.narg(status));

-- name: ListTransfersByAsset :many
-- Per-asset history, scoped by from- or to-office.
SELECT * FROM transfer.asset_transfers
WHERE asset_id = sqlc.arg(asset_id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY created_at DESC;

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
