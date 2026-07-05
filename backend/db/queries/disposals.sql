-- name: CreateDisposal :one
-- gain_loss is computed here (null-propagating): null when either input is null.
INSERT INTO disposal.disposals (
  asset_id, method, disposal_date, proceeds, book_value_at_disposal, gain_loss,
  bast_no, approved_by_id, request_id, created_by_id
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(method), sqlc.arg(disposal_date),
  sqlc.narg(proceeds), sqlc.narg(book_value_at_disposal),
  (sqlc.narg(proceeds)::numeric - sqlc.narg(book_value_at_disposal)::numeric),
  sqlc.narg(bast_no), sqlc.narg(approved_by_id), sqlc.narg(request_id), sqlc.narg(created_by_id)
)
RETURNING *;

-- name: GetDisposalEnriched :one
-- Scoped: caller must have the asset's office in scope (disposals have no office_id).
-- The asset JOIN stays the scope anchor (INNER — a disposal always has a live
-- asset) and doubles as the source of asset_name/asset_tag. LEFT JOINs keep the
-- row visible (with nil names) even when a joined office/user was soft-deleted.
SELECT sqlc.embed(d),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       cu.name     AS created_by_name
FROM disposal.disposals d
JOIN asset.assets a             ON a.id = d.asset_id
LEFT JOIN masterdata.offices o  ON o.id = a.office_id       AND o.deleted_at IS NULL
LEFT JOIN identity.users cu     ON cu.id = d.created_by_id  AND cu.deleted_at IS NULL
WHERE d.id = sqlc.arg(id) AND d.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListDisposalsEnriched :many
SELECT sqlc.embed(d),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       cu.name     AS created_by_name
FROM disposal.disposals d
JOIN asset.assets a             ON a.id = d.asset_id
LEFT JOIN masterdata.offices o  ON o.id = a.office_id       AND o.deleted_at IS NULL
LEFT JOIN identity.users cu     ON cu.id = d.created_by_id  AND cu.deleted_at IS NULL
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY d.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountDisposals :one
SELECT count(*) FROM disposal.disposals d
JOIN asset.assets a ON a.id = d.asset_id
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListDisposalsByAssetEnriched :many
SELECT sqlc.embed(d),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       cu.name     AS created_by_name
FROM disposal.disposals d
JOIN asset.assets a             ON a.id = d.asset_id
LEFT JOIN masterdata.offices o  ON o.id = a.office_id       AND o.deleted_at IS NULL
LEFT JOIN identity.users cu     ON cu.id = d.created_by_id  AND cu.deleted_at IS NULL
WHERE d.asset_id = sqlc.arg(asset_id) AND d.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY d.created_at DESC;

-- name: GetDisposalByAsset :one
-- Guard (office-unscoped): at most one live disposal per asset.
SELECT * FROM disposal.disposals
WHERE asset_id = sqlc.arg(asset_id) AND deleted_at IS NULL
LIMIT 1;

-- name: CountPendingDisposalRequestsForAsset :one
SELECT count(*) FROM approval.requests
WHERE type = 'asset_disposal' AND target_id = sqlc.arg(asset_id)
  AND status = 'pending' AND deleted_at IS NULL;

-- name: SetDisposalBastNo :one
UPDATE disposal.disposals SET bast_no = sqlc.arg(bast_no)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;
