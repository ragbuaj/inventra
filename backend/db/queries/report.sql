-- Reporting & Dashboard module — read-only aggregates.
-- Every query: deleted_at IS NULL + the standard scope clause
--   (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
-- plus an optional narg(office_filter) drill-down (validated against scope in the handler).
-- Money aggregates: COALESCE(SUM(x), 0)::text — never float.
-- Valuation rule: excluded_from_valuation is excluded from money sums, included in counts.

-- name: DashboardAssetKpis :one
SELECT
  count(*)::bigint AS total_assets,
  COALESCE(SUM(purchase_cost) FILTER (WHERE NOT excluded_from_valuation), 0)::text AS acquisition_value,
  COALESCE(SUM(book_value) FILTER (WHERE NOT excluded_from_valuation), 0)::text AS book_value,
  count(*) FILTER (WHERE excluded_from_valuation)::bigint AS excluded_count,
  count(*) FILTER (WHERE status = 'available')::bigint AS st_available,
  count(*) FILTER (WHERE status = 'assigned')::bigint AS st_assigned,
  count(*) FILTER (WHERE status = 'under_maintenance')::bigint AS st_under_maintenance,
  count(*) FILTER (WHERE status = 'in_transfer')::bigint AS st_in_transfer,
  count(*) FILTER (WHERE status = 'retired')::bigint AS st_retired,
  count(*) FILTER (WHERE status = 'disposed')::bigint AS st_disposed,
  count(*) FILTER (WHERE status = 'lost')::bigint AS st_lost,
  COALESCE(SUM(purchase_cost) FILTER (
    WHERE NOT excluded_from_valuation
      AND purchase_date BETWEEN sqlc.arg(period_from)::date AND sqlc.arg(period_to)::date), 0)::text AS acquired_in_period
FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR office_id = sqlc.narg(office_filter));

-- name: DashboardAssetsByCategory :many
SELECT c.name, count(*)::bigint AS cnt
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
GROUP BY c.name
ORDER BY cnt DESC, c.name
LIMIT 5;

-- name: DashboardAssetsByOffice :many
SELECT o.name, count(*)::bigint AS cnt
FROM asset.assets a
JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
GROUP BY o.name
ORDER BY cnt DESC, o.name
LIMIT 5;

-- name: DashboardAssetsByRoom :many
SELECT r.name, count(*)::bigint AS cnt
FROM asset.assets a
LEFT JOIN masterdata.rooms r ON r.id = a.room_id AND r.deleted_at IS NULL
WHERE a.deleted_at IS NULL AND a.office_id = sqlc.arg(office_id)
GROUP BY r.name
ORDER BY cnt DESC, r.name NULLS LAST
LIMIT 5;

-- name: DashboardOverdueCount :one
SELECT count(*)::bigint
FROM assignment.assignments ag
JOIN asset.assets a ON a.id = ag.asset_id AND a.deleted_at IS NULL
WHERE ag.deleted_at IS NULL AND ag.status = 'active'
  AND ag.due_date IS NOT NULL AND ag.due_date < sqlc.arg(today)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));

-- name: DashboardMaintenanceDueCount :one
SELECT count(*)::bigint
FROM maintenance.maintenance_schedules s
JOIN asset.assets a ON a.id = s.asset_id AND a.deleted_at IS NULL
WHERE s.deleted_at IS NULL AND s.is_active AND s.next_due_date <= sqlc.arg(window_end)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));

-- name: DashboardMaintenanceDueList :many
SELECT s.id, a.name AS asset_name, a.asset_tag, mc.name AS category_name, s.next_due_date
FROM maintenance.maintenance_schedules s
JOIN asset.assets a ON a.id = s.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = s.maintenance_category_id AND mc.deleted_at IS NULL
WHERE s.deleted_at IS NULL AND s.is_active AND s.next_due_date <= sqlc.arg(window_end)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
ORDER BY s.next_due_date ASC
LIMIT 3;

-- name: DashboardMaintenanceCost :one
SELECT
  COALESCE(SUM(r.cost) FILTER (WHERE r.completed_date BETWEEN sqlc.arg(cur_from)::date AND sqlc.arg(cur_to)::date), 0)::text AS current_cost,
  COALESCE(SUM(r.cost) FILTER (WHERE r.completed_date BETWEEN sqlc.arg(prev_from)::date AND sqlc.arg(prev_to)::date), 0)::text AS previous_cost
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));

-- name: DashboardDepreciationInPeriod :one
SELECT COALESCE(SUM(e.depreciation_amount), 0)::text
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.basis = 'commercial'
  AND e.period BETWEEN sqlc.arg(period_from)::date AND sqlc.arg(period_to)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter));

-- ══════════════════════════════════════════════════════════════════════════
-- Report builder — assets / depreciation / utilization / maintenance.
-- Every query shares the standard filter block on the assets alias `a`:
--   scope (all_scope OR office_id = ANY(office_ids))
--   + optional narg(office_filter) drill-down
--   + optional narg(category_id).
-- Money aggregates: COALESCE(SUM(x), 0)::text — never float.
-- Valuation rule (assets report): excluded_from_valuation is dropped from money
-- sums (Totals/KPIs/chart) but the rows still list every asset.
-- ══════════════════════════════════════════════════════════════════════════

-- name: ReportAssetRows :many
SELECT a.asset_tag, a.name, c.name AS category_name, a.status,
  COALESCE(a.purchase_cost, '0')::text AS purchase_cost,
  a.accumulated_depreciation::text AS accum_deprec,
  COALESCE(a.book_value, '0')::text AS book_value
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR a.status = sqlc.narg(status))
ORDER BY a.asset_tag
LIMIT sqlc.arg(lim);

-- name: ReportAssetTotals :one
SELECT count(*)::bigint AS row_count,
  COALESCE(SUM(a.purchase_cost) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_cost,
  COALESCE(SUM(a.accumulated_depreciation) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_accum,
  COALESCE(SUM(a.book_value) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_book
FROM asset.assets a
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR a.status = sqlc.narg(status));

-- book value per category (top 8)
-- name: ReportAssetChart :many
SELECT c.name, COALESCE(SUM(a.book_value) FILTER (WHERE NOT a.excluded_from_valuation), 0)::text AS total_book
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  AND (sqlc.narg(status)::shared.asset_status IS NULL OR a.status = sqlc.narg(status))
GROUP BY c.name
ORDER BY SUM(a.book_value) FILTER (WHERE NOT a.excluded_from_valuation) DESC NULLS LAST, c.name
LIMIT 8;

-- name: ReportDepreciationRows :many
SELECT to_char(e.period, 'YYYY-MM') AS period,
  COALESCE(SUM(e.opening_value), 0)::text AS opening,
  COALESCE(SUM(e.depreciation_amount), 0)::text AS amount,
  COALESCE(SUM(e.closing_value), 0)::text AS closing
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.basis = sqlc.arg(basis)
  AND e.period BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
GROUP BY e.period ORDER BY e.period;

-- name: ReportDepreciationKpis :one
SELECT
  COALESCE(SUM(e.depreciation_amount) FILTER (WHERE e.period BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date), 0)::text AS period_expense,
  COALESCE(SUM(e.depreciation_amount) FILTER (WHERE e.period <= sqlc.arg(date_to)::date), 0)::text AS accumulated
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.basis = sqlc.arg(basis)
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id));

-- sum of each asset's last closing_value <= date_to
-- name: ReportDepreciationRemaining :one
SELECT COALESCE(SUM(last.closing_value), 0)::text
FROM (
  SELECT DISTINCT ON (e.asset_id) e.closing_value
  FROM depreciation.depreciation_entries e
  JOIN asset.assets a ON a.id = e.asset_id AND a.deleted_at IS NULL
  WHERE e.deleted_at IS NULL AND e.basis = sqlc.arg(basis) AND e.period <= sqlc.arg(date_to)::date
    AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
    AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
    AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
  ORDER BY e.asset_id, e.period DESC
) AS last;

-- name: ReportUtilizationRows :many
SELECT a.name, a.asset_tag, c.name AS category_name,
  COALESCE(SUM(GREATEST(0,
    LEAST(COALESCE(ag.checkin_date::date, sqlc.arg(date_to)::date), sqlc.arg(date_to)::date)
    - GREATEST(ag.checkout_date::date, sqlc.arg(date_from)::date) + 1)), 0)::bigint AS days_loaned,
  count(ag.id)::bigint AS loan_count
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
LEFT JOIN assignment.assignments ag ON ag.asset_id = a.id AND ag.deleted_at IS NULL
  AND ag.checkout_date::date <= sqlc.arg(date_to)::date
  AND COALESCE(ag.checkin_date::date, sqlc.arg(date_to)::date) >= sqlc.arg(date_from)::date
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
GROUP BY a.id, a.name, a.asset_tag, c.name
HAVING count(ag.id) > 0
ORDER BY days_loaned DESC, a.asset_tag
LIMIT sqlc.arg(lim);

-- name: ReportUtilizationKpis :one
SELECT count(*)::bigint AS active_loans
FROM assignment.assignments ag
JOIN asset.assets a ON a.id = ag.asset_id AND a.deleted_at IS NULL
WHERE ag.deleted_at IS NULL AND ag.status = 'active'
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id));

-- name: ReportMaintenanceRows :many
SELECT a.name AS asset_name, c.name AS category_name, r.type,
  count(*)::bigint AS actions, COALESCE(SUM(r.cost), 0)::text AS total_cost
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND r.completed_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
GROUP BY a.id, a.name, c.name, r.type
ORDER BY SUM(r.cost) DESC NULLS LAST, a.name
LIMIT sqlc.arg(lim);

-- name: ReportMaintenanceKpis :one
SELECT COALESCE(SUM(r.cost), 0)::text AS total,
  COALESCE(SUM(r.cost) FILTER (WHERE r.type = 'preventive'), 0)::text AS preventive,
  COALESCE(SUM(r.cost) FILTER (WHERE r.type = 'corrective'), 0)::text AS corrective
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND r.completed_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id));

-- cost per category (top 8)
-- name: ReportMaintenanceChart :many
SELECT c.name, COALESCE(SUM(r.cost), 0)::text AS total
FROM maintenance.maintenance_records r
JOIN asset.assets a ON a.id = r.asset_id AND a.deleted_at IS NULL
JOIN masterdata.categories c ON c.id = a.category_id AND c.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'completed'
  AND r.completed_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(office_filter)::uuid IS NULL OR a.office_id = sqlc.narg(office_filter))
  AND (sqlc.narg(category_id)::uuid IS NULL OR a.category_id = sqlc.narg(category_id))
GROUP BY c.name
ORDER BY SUM(r.cost) DESC NULLS LAST, c.name
LIMIT 8;
