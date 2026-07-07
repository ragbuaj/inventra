-- name: CheckoutAssignment :one
INSERT INTO assignment.assignments (
  asset_id, employee_id, assigned_by_id, checkout_date, due_date, condition_out, notes, status
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(employee_id), sqlc.arg(assigned_by_id),
  sqlc.arg(checkout_date), sqlc.narg(due_date), sqlc.narg(condition_out), sqlc.narg(notes), 'active'
)
RETURNING *;

-- name: CheckinAssignment :one
UPDATE assignment.assignments
SET status = 'returned', checkin_date = sqlc.arg(checkin_date), condition_in = sqlc.narg(condition_in)
WHERE id = sqlc.arg(id) AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: GetAssignmentScoped :one
-- Plain (unenriched) row, scoped by the asset's office. Used by Checkin to load +
-- validate state before the update.
SELECT asg.* FROM assignment.assignments asg
JOIN asset.assets a ON a.id = asg.asset_id AND a.deleted_at IS NULL
WHERE asg.id = sqlc.arg(id) AND asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: GetAssignmentEnriched :one
SELECT sqlc.embed(asg),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       a.office_id AS office_id,
       e.name      AS employee_name,
       u.name      AS assigned_by_name,
       o.name      AS office_name
FROM assignment.assignments asg
LEFT JOIN asset.assets a         ON a.id = asg.asset_id       AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id    AND e.deleted_at IS NULL
LEFT JOIN identity.users u       ON u.id = asg.assigned_by_id AND u.deleted_at IS NULL
LEFT JOIN masterdata.offices o   ON o.id = a.office_id        AND o.deleted_at IS NULL
WHERE asg.id = sqlc.arg(id) AND asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));

-- name: ListAssignmentsEnriched :many
SELECT sqlc.embed(asg),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       a.office_id AS office_id,
       e.name      AS employee_name,
       u.name      AS assigned_by_name,
       o.name      AS office_name
FROM assignment.assignments asg
JOIN asset.assets a              ON a.id = asg.asset_id       AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id    AND e.deleted_at IS NULL
LEFT JOIN identity.users u       ON u.id = asg.assigned_by_id AND u.deleted_at IS NULL
LEFT JOIN masterdata.offices o   ON o.id = a.office_id        AND o.deleted_at IS NULL
WHERE asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.assignment_status IS NULL OR asg.status = sqlc.narg(status))
  AND (sqlc.narg(employee_id)::uuid IS NULL OR asg.employee_id = sqlc.narg(employee_id))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR e.name ILIKE '%' || sqlc.narg(search) || '%')
ORDER BY asg.checkout_date DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAssignments :one
SELECT count(*)
FROM assignment.assignments asg
JOIN asset.assets a              ON a.id = asg.asset_id    AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id AND e.deleted_at IS NULL
WHERE asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.assignment_status IS NULL OR asg.status = sqlc.narg(status))
  AND (sqlc.narg(employee_id)::uuid IS NULL OR asg.employee_id = sqlc.narg(employee_id))
  AND (sqlc.narg(search)::text IS NULL OR a.name ILIKE '%' || sqlc.narg(search) || '%'
       OR a.asset_tag ILIKE '%' || sqlc.narg(search) || '%'
       OR e.name ILIKE '%' || sqlc.narg(search) || '%');

-- name: ListAssignmentsByAssetEnriched :many
SELECT sqlc.embed(asg),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       a.office_id AS office_id,
       e.name      AS employee_name,
       u.name      AS assigned_by_name,
       o.name      AS office_name
FROM assignment.assignments asg
JOIN asset.assets a              ON a.id = asg.asset_id       AND a.deleted_at IS NULL
LEFT JOIN masterdata.employees e ON e.id = asg.employee_id    AND e.deleted_at IS NULL
LEFT JOIN identity.users u       ON u.id = asg.assigned_by_id AND u.deleted_at IS NULL
LEFT JOIN masterdata.offices o   ON o.id = a.office_id        AND o.deleted_at IS NULL
WHERE asg.asset_id = sqlc.arg(asset_id) AND asg.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY asg.checkout_date DESC;
