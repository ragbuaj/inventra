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
