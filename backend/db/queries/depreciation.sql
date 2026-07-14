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
-- The DO UPDATE's WHERE guard makes a closed period unmatchable (0 rows →
-- pgx.ErrNoRows): even if a ComputePeriod raced past its status pre-check, it
-- can never flip a closed period back to 'computed'. The service maps that
-- ErrNoRows to ErrPeriodClosed and rolls back the regenerated entries.
INSERT INTO depreciation.depreciation_periods (period, status, computed_at, computed_by, asset_count, total_amount, skipped_count)
VALUES (sqlc.arg(period), 'computed', now(), sqlc.arg(computed_by), sqlc.arg(asset_count), sqlc.arg(total_amount), sqlc.arg(skipped_count))
ON CONFLICT (period) WHERE deleted_at IS NULL
DO UPDATE SET status = 'computed', computed_at = now(), computed_by = EXCLUDED.computed_by,
              asset_count = EXCLUDED.asset_count, total_amount = EXCLUDED.total_amount,
              skipped_count = EXCLUDED.skipped_count
WHERE depreciation.depreciation_periods.status <> 'closed'
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
-- Embeds the full category row (not just name/gl_account_code) so callers can
-- also re-resolve Params (method/life_months) via ResolveCommercial/ResolveFiscal.
SELECT sqlc.embed(e), sqlc.embed(a), sqlc.embed(c),
       o.name AS office_name
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.period = sqlc.arg(period) AND e.basis = sqlc.arg(basis)
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY a.name;

-- name: SumAssetAmounts :one
SELECT COALESCE(SUM(depreciation_amount), 0)::text FROM depreciation.depreciation_entries
WHERE asset_id = $1 AND basis = $2 AND deleted_at IS NULL;

-- name: GetAssetForUpdate :one
-- Row-locked read for RecordImpairment's read-modify-write (precedent:
-- approval.sql GetRequestForUpdate): a second concurrent impairment blocks
-- here until the first commits, then re-reads the post-commit book_value/
-- impairment_loss so deltas accumulate instead of clobbering (lost update).
SELECT * FROM asset.assets WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ApplyAssetImpairment :one
-- PSAK 48 impairment write-down: sets the money fields directly. No
-- depreciation entry is posted here — impairment is a separate loss, not a
-- depreciation expense. book_value is the DERIVED carrying amount (compute
-- rewrites it every run); impaired_book_value is the STABLE resume floor that
-- the compute's commercial resumption override picks up prospectively (see
-- RecordImpairment / regenerateBasis). Both are set to the recoverable amount
-- here; a later, deeper impairment lowers the floor further (correct).
UPDATE asset.assets
SET impairment_loss = sqlc.arg(impairment_loss),
    book_value = sqlc.arg(book_value),
    impaired_book_value = sqlc.arg(impaired_book_value)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: ScheduleRows :many
-- One asset-based, paginated schedule page. A row is included if it has an
-- entry for this period+basis (entry row) OR the asset is a parameterizable
-- "union" row (fully depreciated, no entry this period). The parameterizable
-- predicate mirrors ResolveCommercial/ResolveFiscal's Skip checks in SQL.
SELECT sqlc.embed(a), sqlc.embed(c),
       o.name AS office_name,
       (e.id IS NOT NULL)::boolean AS has_entry,
       e.method AS entry_method,
       CASE WHEN e.id IS NOT NULL THEN e.opening_value::text
            ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2)::text END AS opening,
       CASE WHEN e.id IS NOT NULL THEN e.depreciation_amount::text ELSE '0.00' END AS amount,
       acc.accumulated AS accumulated,
       CASE WHEN e.id IS NOT NULL THEN e.closing_value::text
            ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2)::text END AS closing
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN depreciation.depreciation_entries e
       ON e.asset_id = a.id AND e.basis = sqlc.arg(basis)
      AND e.period = sqlc.arg(period) AND e.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(de.depreciation_amount), 0)::text AS accumulated
  FROM depreciation.depreciation_entries de
  WHERE de.asset_id = a.id AND de.basis = sqlc.arg(basis)
    AND de.period <= sqlc.arg(period) AND de.deleted_at IS NULL
) acc ON true
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    e.id IS NOT NULL
    OR (
      a.capitalized AND a.status <> 'disposed'
      AND a.purchase_cost IS NOT NULL
      AND a.purchase_date IS NOT NULL
      AND (
        (sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.depreciation_method, c.default_depreciation_method) IS NOT NULL
         AND COALESCE(a.useful_life_months, c.default_useful_life_months) IS NOT NULL)
        OR (NOT sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) IS NOT NULL
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) <> 'non_susut')
      )
    )
  )
  AND (sqlc.narg(search)::text IS NULL
       OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_id)::uuid IS NULL OR a.office_id = sqlc.narg(office_id))
ORDER BY a.name, a.id
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ScheduleTotals :one
-- Filtered tfoot totals + row count (same FROM/WHERE as ScheduleRows, incl.
-- the search/category/office filters, no pagination).
SELECT
  COUNT(*) AS total,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.opening_value::numeric
                          ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2) END), 2), 0)::text AS opening,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.depreciation_amount::numeric ELSE 0 END), 2), 0)::text AS amount,
  COALESCE(round(SUM(acc.accumulated::numeric), 2), 0)::text AS accumulated,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.closing_value::numeric
                          ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2) END), 2), 0)::text AS closing
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN depreciation.depreciation_entries e
       ON e.asset_id = a.id AND e.basis = sqlc.arg(basis)
      AND e.period = sqlc.arg(period) AND e.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(de.depreciation_amount), 0)::text AS accumulated
  FROM depreciation.depreciation_entries de
  WHERE de.asset_id = a.id AND de.basis = sqlc.arg(basis)
    AND de.period <= sqlc.arg(period) AND de.deleted_at IS NULL
) acc ON true
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    e.id IS NOT NULL
    OR (
      a.capitalized AND a.status <> 'disposed'
      AND a.purchase_cost IS NOT NULL
      AND a.purchase_date IS NOT NULL
      AND (
        (sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.depreciation_method, c.default_depreciation_method) IS NOT NULL
         AND COALESCE(a.useful_life_months, c.default_useful_life_months) IS NOT NULL)
        OR (NOT sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) IS NOT NULL
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) <> 'non_susut')
      )
    )
  )
  AND (sqlc.narg(search)::text IS NULL
       OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(office_id)::uuid IS NULL OR a.office_id = sqlc.narg(office_id));

-- name: ScheduleKpi :one
-- Unfiltered KPI tiles (period + basis + scope only — table filters must never
-- shrink the tiles). Same FROM/WHERE as ScheduleTotals MINUS the search/
-- category/office filters.
SELECT
  COUNT(*) AS asset_count,
  COALESCE(round(SUM(a.purchase_cost::numeric), 2), 0)::text AS total_cost,
  COALESCE(round(SUM(acc.accumulated::numeric), 2), 0)::text AS total_accumulated,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.closing_value::numeric
                          ELSE round(a.purchase_cost::numeric - acc.accumulated::numeric, 2) END), 2), 0)::text AS total_book_value,
  COALESCE(round(SUM(CASE WHEN e.id IS NOT NULL THEN e.depreciation_amount::numeric ELSE 0 END), 2), 0)::text AS period_expense
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN depreciation.depreciation_entries e
       ON e.asset_id = a.id AND e.basis = sqlc.arg(basis)
      AND e.period = sqlc.arg(period) AND e.deleted_at IS NULL
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(de.depreciation_amount), 0)::text AS accumulated
  FROM depreciation.depreciation_entries de
  WHERE de.asset_id = a.id AND de.basis = sqlc.arg(basis)
    AND de.period <= sqlc.arg(period) AND de.deleted_at IS NULL
) acc ON true
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (
    e.id IS NOT NULL
    OR (
      a.capitalized AND a.status <> 'disposed'
      AND a.purchase_cost IS NOT NULL
      AND a.purchase_date IS NOT NULL
      AND (
        (sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.depreciation_method, c.default_depreciation_method) IS NOT NULL
         AND COALESCE(a.useful_life_months, c.default_useful_life_months) IS NOT NULL)
        OR (NOT sqlc.arg(is_commercial)::boolean
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) IS NOT NULL
         AND COALESCE(a.fiscal_group, c.default_fiscal_group) <> 'non_susut')
      )
    )
  );
