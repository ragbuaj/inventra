-- Depreciation engine queries. See docs/DATABASE.md §4.4 and spec 2026-07-05.

-- name: AdvisoryLockDepreciation :exec
-- Transaction-scoped exclusive lock; released automatically at COMMIT/ROLLBACK.
SELECT pg_advisory_xact_lock(hashtext('depreciation.compute'));

-- name: GetDepreciationPeriod :one
SELECT * FROM depreciation.depreciation_periods WHERE period = $1 AND deleted_at IS NULL;

-- name: ListDepreciationPeriods :many
SELECT * FROM depreciation.depreciation_periods WHERE deleted_at IS NULL ORDER BY period DESC;

-- name: LastClosedPeriod :one
SELECT period FROM depreciation.depreciation_periods
WHERE status = 'closed' AND deleted_at IS NULL ORDER BY period DESC LIMIT 1;

-- name: CountOpenEarlierPeriods :one
SELECT count(*) FROM depreciation.depreciation_periods
WHERE deleted_at IS NULL AND period < $1 AND status <> 'closed';

-- name: UpsertPeriodComputed :one
INSERT INTO depreciation.depreciation_periods (period, status, computed_at, computed_by, asset_count, total_amount, skipped_count)
VALUES (sqlc.arg(period), 'computed', now(), sqlc.arg(computed_by), sqlc.arg(asset_count), sqlc.arg(total_amount), sqlc.arg(skipped_count))
ON CONFLICT (period) WHERE deleted_at IS NULL
DO UPDATE SET status = 'computed', computed_at = now(), computed_by = EXCLUDED.computed_by,
              asset_count = EXCLUDED.asset_count, total_amount = EXCLUDED.total_amount,
              skipped_count = EXCLUDED.skipped_count
RETURNING *;

-- name: SetPeriodClosed :one
UPDATE depreciation.depreciation_periods SET status = 'closed', closed_at = now(), closed_by = $2
WHERE period = $1 AND status = 'computed' AND deleted_at IS NULL RETURNING *;

-- name: DeleteEntriesAfterWatermark :exec
-- Regeneration window: everything past the closed watermark up to the target period.
DELETE FROM depreciation.depreciation_entries
WHERE deleted_at IS NULL AND period > sqlc.arg(watermark) AND period <= sqlc.arg(target);

-- name: DeleteEntriesThrough :exec
-- First-ever run (no watermark): clear everything ≤ target.
DELETE FROM depreciation.depreciation_entries
WHERE deleted_at IS NULL AND period <= sqlc.arg(target);

-- name: LastEntryAtOrBefore :one
SELECT * FROM depreciation.depreciation_entries
WHERE asset_id = $1 AND basis = $2 AND deleted_at IS NULL AND period <= $3
ORDER BY period DESC LIMIT 1;

-- name: InsertDepreciationEntry :exec
INSERT INTO depreciation.depreciation_entries (asset_id, basis, period, opening_value, depreciation_amount, closing_value, method)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAssetsForDepreciation :many
-- Every capitalized, non-deleted asset with its category (engine resolves/skips per-asset).
SELECT sqlc.embed(a), sqlc.embed(c)
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
WHERE a.deleted_at IS NULL;

-- name: UpdateAssetDepreciationSummary :exec
UPDATE asset.assets SET accumulated_depreciation = sqlc.arg(accumulated), book_value = sqlc.arg(book_value)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListAssetEntries :many
SELECT * FROM depreciation.depreciation_entries
WHERE asset_id = $1 AND deleted_at IS NULL ORDER BY basis, period;

-- name: ListEntriesForPeriod :many
-- Schedule/journal source: entries of one period+basis joined to asset+category+office.
SELECT sqlc.embed(e), sqlc.embed(a),
       c.name AS category_name, c.gl_account_code AS gl_account_code,
       o.name AS office_name
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.period = sqlc.arg(period) AND e.basis = sqlc.arg(basis)
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: SumAssetAmounts :one
SELECT COALESCE(SUM(depreciation_amount), 0)::text FROM depreciation.depreciation_entries
WHERE asset_id = $1 AND basis = $2 AND deleted_at IS NULL;
