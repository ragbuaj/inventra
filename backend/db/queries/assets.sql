-- Asset core queries (asset.assets + asset.asset_tag_counters).
-- Respects soft delete and caller data scope (all_scope / office_ids).

-- name: ListAssets :many
SELECT * FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(search)::text IS NULL OR name ILIKE '%' || sqlc.narg(search) || '%'
       OR asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR serial_number ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(asset_class)::shared.asset_class IS NULL OR asset_class = sqlc.narg(asset_class))
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAssets :one
SELECT count(*) FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(search)::text IS NULL OR name ILIKE '%' || sqlc.narg(search) || '%'
       OR asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR serial_number ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(asset_class)::shared.asset_class IS NULL OR asset_class = sqlc.narg(asset_class));

-- name: GetAsset :one
SELECT * FROM asset.assets WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateAsset :one
INSERT INTO asset.assets (
  asset_tag, name, category_id, brand_id, model_id, room_id, office_id, unit_id,
  status, serial_number, purchase_date, purchase_cost, vendor_id, po_number,
  funding_source, warranty_expiry, specifications, asset_class, capitalized,
  acquisition_bast_no, created_by_id, notes
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,'available',$9,$10,$11,$12,$13,$14,$15,
  COALESCE($16,'{}')::jsonb,$17,$18,$19,$20,$21
) RETURNING *;

-- name: UpdateAsset :one
UPDATE asset.assets SET
  name = $2, category_id = $3, brand_id = $4, model_id = $5, room_id = $6,
  unit_id = $7, serial_number = $8, purchase_date = $9, vendor_id = $10,
  po_number = $11, funding_source = $12, warranty_expiry = $13,
  specifications = COALESCE($14,'{}')::jsonb, notes = $15
WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- name: SetAssetStatus :one
UPDATE asset.assets SET status = $2 WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- name: SetAssetValuationExclusion :one
UPDATE asset.assets SET excluded_from_valuation = $2, valuation_exclusion_reason = $3
WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- name: BumpAssetTagCounter :one
INSERT INTO asset.asset_tag_counters (office_id, category_id, year, last_seq)
VALUES ($1, $2, $3, 1)
ON CONFLICT (office_id, category_id, year)
DO UPDATE SET last_seq = asset.asset_tag_counters.last_seq + 1
RETURNING last_seq;

-- name: GetOfficeCode :one
SELECT code FROM masterdata.offices WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCategoryCode :one
SELECT code FROM masterdata.categories WHERE id = $1 AND deleted_at IS NULL;
