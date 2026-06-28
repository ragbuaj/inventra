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
  sqlc.arg(asset_tag),sqlc.arg(name),sqlc.arg(category_id),sqlc.arg(brand_id),
  sqlc.arg(model_id),sqlc.arg(room_id),sqlc.arg(office_id),sqlc.arg(unit_id),
  'available',sqlc.arg(serial_number),sqlc.arg(purchase_date),sqlc.arg(purchase_cost),
  sqlc.arg(vendor_id),sqlc.arg(po_number),sqlc.arg(funding_source),sqlc.arg(warranty_expiry),
  COALESCE(sqlc.arg(specifications),'{}')::jsonb,sqlc.arg(asset_class),sqlc.arg(capitalized),
  sqlc.arg(acquisition_bast_no),sqlc.arg(created_by_id),sqlc.arg(notes)
) RETURNING *;

-- name: UpdateAsset :one
UPDATE asset.assets SET
  name = sqlc.arg(name), category_id = sqlc.arg(category_id), brand_id = sqlc.arg(brand_id),
  model_id = sqlc.arg(model_id), room_id = sqlc.arg(room_id), unit_id = sqlc.arg(unit_id),
  serial_number = sqlc.arg(serial_number), purchase_date = sqlc.arg(purchase_date),
  vendor_id = sqlc.arg(vendor_id), po_number = sqlc.arg(po_number),
  funding_source = sqlc.arg(funding_source), warranty_expiry = sqlc.arg(warranty_expiry),
  specifications = COALESCE(sqlc.arg(specifications),'{}')::jsonb, notes = sqlc.arg(notes)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL RETURNING *;

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

-- name: CreateAttachment :one
INSERT INTO asset.asset_attachments (
  asset_id, kind, object_key, thumbnail_key, original_filename, size_bytes, mime_type, created_by_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

-- name: ListAttachments :many
SELECT * FROM asset.asset_attachments
WHERE asset_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetAttachment :one
SELECT * FROM asset.asset_attachments WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteAttachment :execrows
UPDATE asset.asset_attachments SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateAssetDocument :one
INSERT INTO asset.asset_documents (
  asset_id, doc_type, doc_no, doc_date, counterparty, related_request_id, created_by_id
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAssetDocuments :many
SELECT * FROM asset.asset_documents
WHERE asset_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetAssetDocument :one
SELECT * FROM asset.asset_documents WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateAssetDocument :one
UPDATE asset.asset_documents
SET doc_type = $2, doc_no = $3, doc_date = $4, counterparty = $5, related_request_id = $6
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetAssetDocumentObjectKey :one
UPDATE asset.asset_documents
SET object_key = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteAssetDocument :execrows
UPDATE asset.asset_documents SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: GetAssetByTag :one
SELECT * FROM asset.assets WHERE asset_tag = $1 AND deleted_at IS NULL;

-- name: GetAssetLabelByID :one
SELECT a.asset_tag, a.name, o.code AS office_code, c.name AS category_name, a.purchase_date
FROM asset.assets a
JOIN masterdata.offices o ON o.id = a.office_id
JOIN masterdata.categories c ON c.id = a.category_id
WHERE a.id = $1 AND a.deleted_at IS NULL;

-- name: GetAssetLabelByTag :one
SELECT a.asset_tag, a.name, o.code AS office_code, c.name AS category_name, a.purchase_date
FROM asset.assets a
JOIN masterdata.offices o ON o.id = a.office_id
JOIN masterdata.categories c ON c.id = a.category_id
WHERE a.asset_tag = $1 AND a.deleted_at IS NULL;
