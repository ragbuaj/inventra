-- name: CreateMaintSchedule :one
INSERT INTO maintenance.maintenance_schedules (
  asset_id, maintenance_category_id, interval_months, next_due_date
) VALUES (
  sqlc.arg(asset_id), sqlc.narg(maintenance_category_id), sqlc.arg(interval_months), sqlc.arg(next_due_date)
)
RETURNING *;

-- name: GetMaintScheduleScoped :one
SELECT ms.* FROM maintenance.maintenance_schedules ms
JOIN asset.assets a ON a.id = ms.asset_id AND a.deleted_at IS NULL
WHERE ms.id = sqlc.arg(id) AND ms.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListMaintSchedulesEnriched :many
SELECT sqlc.embed(ms),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name
FROM maintenance.maintenance_schedules ms
JOIN asset.assets a ON a.id = ms.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = ms.maintenance_category_id AND mc.deleted_at IS NULL
WHERE ms.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(is_active)::boolean IS NULL OR ms.is_active = sqlc.narg(is_active))
ORDER BY ms.next_due_date ASC NULLS LAST
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountMaintSchedules :one
SELECT count(*)
FROM maintenance.maintenance_schedules ms
JOIN asset.assets a ON a.id = ms.asset_id AND a.deleted_at IS NULL
WHERE ms.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(is_active)::boolean IS NULL OR ms.is_active = sqlc.narg(is_active));

-- name: UpdateMaintSchedule :one
UPDATE maintenance.maintenance_schedules
SET maintenance_category_id = COALESCE(sqlc.narg(maintenance_category_id), maintenance_category_id),
    interval_months         = COALESCE(sqlc.narg(interval_months), interval_months),
    is_active               = COALESCE(sqlc.narg(is_active), is_active),
    next_due_date           = COALESCE(sqlc.narg(next_due_date), next_due_date)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMaintSchedule :execrows
UPDATE maintenance.maintenance_schedules
SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: TouchMaintScheduleDone :one
UPDATE maintenance.maintenance_schedules
SET last_done_date = sqlc.arg(last_done_date), next_due_date = sqlc.arg(next_due_date)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: CreateMaintRecord :one
INSERT INTO maintenance.maintenance_records (
  asset_id, schedule_id, maintenance_category_id, problem_category_id,
  type, status, scheduled_date, completed_date, cost, vendor_id,
  performed_by, description, reported_by_id
) VALUES (
  sqlc.arg(asset_id), sqlc.narg(schedule_id), sqlc.narg(maintenance_category_id), sqlc.narg(problem_category_id),
  sqlc.arg(type), sqlc.arg(status), sqlc.narg(scheduled_date), sqlc.narg(completed_date), sqlc.narg(cost), sqlc.narg(vendor_id),
  sqlc.narg(performed_by), sqlc.arg(description), sqlc.narg(reported_by_id)
)
RETURNING *;

-- name: GetMaintRecordScoped :one
SELECT mr.* FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
WHERE mr.id = sqlc.arg(id) AND mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: GetMaintRecordEnriched :one
SELECT sqlc.embed(mr),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name,
       pc.name     AS problem_name,
       v.name      AS vendor_name,
       u.name      AS reported_by_name
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = mr.maintenance_category_id AND mc.deleted_at IS NULL
LEFT JOIN masterdata.problem_categories pc ON pc.id = mr.problem_category_id AND pc.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
LEFT JOIN identity.users u ON u.id = mr.reported_by_id AND u.deleted_at IS NULL
WHERE mr.id = sqlc.arg(id) AND mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListMaintRecordsEnriched :many
SELECT sqlc.embed(mr),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name,
       pc.name     AS problem_name,
       v.name      AS vendor_name,
       u.name      AS reported_by_name
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = mr.maintenance_category_id AND mc.deleted_at IS NULL
LEFT JOIN masterdata.problem_categories pc ON pc.id = mr.problem_category_id AND pc.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
LEFT JOIN identity.users u ON u.id = mr.reported_by_id AND u.deleted_at IS NULL
WHERE mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.maintenance_status IS NULL OR mr.status = sqlc.narg(status))
  AND (sqlc.narg(mtype)::shared.maintenance_type IS NULL OR mr.type = sqlc.narg(mtype))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR v.name ILIKE '%' || sqlc.narg(search) || '%')
ORDER BY COALESCE(mr.scheduled_date, mr.created_at::date) DESC, mr.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountMaintRecords :one
SELECT count(*)
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
WHERE mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.maintenance_status IS NULL OR mr.status = sqlc.narg(status))
  AND (sqlc.narg(mtype)::shared.maintenance_type IS NULL OR mr.type = sqlc.narg(mtype))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR v.name ILIKE '%' || sqlc.narg(search) || '%');

-- name: UpdateMaintRecord :one
UPDATE maintenance.maintenance_records
SET status                  = COALESCE(sqlc.narg(status), status),
    maintenance_category_id = COALESCE(sqlc.narg(maintenance_category_id), maintenance_category_id),
    scheduled_date          = COALESCE(sqlc.narg(scheduled_date), scheduled_date),
    completed_date          = COALESCE(sqlc.narg(completed_date), completed_date),
    cost                    = COALESCE(sqlc.narg(cost), cost),
    vendor_id               = COALESCE(sqlc.narg(vendor_id), vendor_id),
    performed_by            = COALESCE(sqlc.narg(performed_by), performed_by),
    description             = COALESCE(sqlc.narg(description), description)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: ListMaintRecordsByAssetEnriched :many
SELECT sqlc.embed(mr),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       mc.name     AS category_name,
       pc.name     AS problem_name,
       v.name      AS vendor_name,
       u.name      AS reported_by_name
FROM maintenance.maintenance_records mr
JOIN asset.assets a ON a.id = mr.asset_id AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
LEFT JOIN masterdata.maintenance_categories mc ON mc.id = mr.maintenance_category_id AND mc.deleted_at IS NULL
LEFT JOIN masterdata.problem_categories pc ON pc.id = mr.problem_category_id AND pc.deleted_at IS NULL
LEFT JOIN masterdata.vendors v ON v.id = mr.vendor_id AND v.deleted_at IS NULL
LEFT JOIN identity.users u ON u.id = mr.reported_by_id AND u.deleted_at IS NULL
WHERE mr.asset_id = sqlc.arg(asset_id) AND mr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY mr.created_at DESC;

-- name: CountActiveMaintRecordsByAsset :one
-- Active = scheduled or in_progress. exclude_id lets the caller ignore the row
-- it is about to transition (release check).
SELECT count(*)
FROM maintenance.maintenance_records
WHERE asset_id = sqlc.arg(asset_id) AND deleted_at IS NULL
  AND status IN ('scheduled', 'in_progress')
  AND (sqlc.narg(exclude_id)::uuid IS NULL OR id <> sqlc.narg(exclude_id));

-- name: ListMaintAttentionAssets :many
-- Assets flagged under_maintenance (e.g. by assignment check-in) with no active
-- maintenance record — the "Perlu Tindak Lanjut" queue.
SELECT a.id, a.asset_tag, a.name, a.office_id, o.name AS office_name
FROM asset.assets a
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
WHERE a.deleted_at IS NULL AND a.status = 'under_maintenance'
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND NOT EXISTS (
    SELECT 1 FROM maintenance.maintenance_records mr
    WHERE mr.asset_id = a.id AND mr.deleted_at IS NULL
      AND mr.status IN ('scheduled', 'in_progress')
  )
ORDER BY a.updated_at DESC
LIMIT 100;

-- name: CountPendingMaintRequests :one
-- Duplicate-guard: pending maintenance request for the same asset by the same maker.
SELECT count(*)
FROM approval.requests
WHERE type = 'maintenance' AND status = 'pending' AND deleted_at IS NULL
  AND target_id = sqlc.arg(asset_id) AND requested_by_id = sqlc.arg(requested_by_id);

-- name: ListSchedulesDueBetween :many
-- Unscoped, unlimited sweep for the notification due-reminder job. Distinct from
-- DashboardMaintenanceDueList, which is scope-filtered and hardcodes LIMIT 3 for
-- the dashboard card, so it cannot drive a sweep.
SELECT sqlc.embed(ms),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       a.office_id AS office_id
FROM maintenance.maintenance_schedules ms
JOIN asset.assets a ON a.id = ms.asset_id AND a.deleted_at IS NULL
WHERE ms.deleted_at IS NULL
  AND ms.is_active
  AND ms.next_due_date IS NOT NULL
  AND ms.next_due_date <= sqlc.arg(due_before)::date
ORDER BY ms.next_due_date ASC;
