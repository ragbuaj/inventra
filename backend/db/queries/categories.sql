-- Asset category master data (masterdata.categories). Respects soft delete.

-- name: ListCategories :many
SELECT * FROM masterdata.categories
WHERE deleted_at IS NULL
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR coalesce(code, '') ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY name
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountCategories :one
SELECT count(*) FROM masterdata.categories
WHERE deleted_at IS NULL
  AND (
    sqlc.arg(search)::text = ''
    OR name ILIKE '%' || sqlc.arg(search) || '%'
    OR coalesce(code, '') ILIKE '%' || sqlc.arg(search) || '%'
  );

-- name: GetCategory :one
SELECT * FROM masterdata.categories
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateCategory :one
INSERT INTO masterdata.categories (
  name, code, parent_id, default_depreciation_method,
  default_useful_life_months, default_salvage_rate,
  asset_class, default_fiscal_group, default_fiscal_life_months,
  gl_account_code, capitalization_threshold, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateCategory :one
UPDATE masterdata.categories
SET name = $2,
    code = $3,
    parent_id = $4,
    default_depreciation_method = $5,
    default_useful_life_months = $6,
    default_salvage_rate = $7,
    asset_class = $8,
    default_fiscal_group = $9,
    default_fiscal_life_months = $10,
    gl_account_code = $11,
    capitalization_threshold = $12,
    is_active = $13
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCategory :execrows
UPDATE masterdata.categories
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListCategoryTree :many
-- The full non-deleted category set (no pagination) for client-side tree building.
SELECT * FROM masterdata.categories
WHERE deleted_at IS NULL
ORDER BY name;
