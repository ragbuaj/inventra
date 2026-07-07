-- Approval / maker-checker queries (approval schema).
-- See docs/DATABASE.md §4.5 and PRD §3.6 for schema context.

-- name: MatchThresholdSteps :many
SELECT * FROM approval.approval_thresholds
WHERE request_type = $1 AND is_active AND deleted_at IS NULL
  AND amount_from <= sqlc.arg(amount)
  AND (amount_to IS NULL OR sqlc.arg(amount) < amount_to)
ORDER BY step_order;

-- name: ListThresholds :many
SELECT * FROM approval.approval_thresholds WHERE deleted_at IS NULL
ORDER BY request_type, amount_from, step_order;

-- name: CreateThreshold :one
INSERT INTO approval.approval_thresholds
  (request_type, amount_from, amount_to, required_level, step_order, is_active)
VALUES (
  sqlc.arg(request_type),sqlc.arg(amount_from),sqlc.arg(amount_to),
  sqlc.arg(required_level),sqlc.arg(step_order),COALESCE(sqlc.arg(is_active)::boolean,true)
) RETURNING *;

-- name: UpdateThreshold :one
UPDATE approval.approval_thresholds SET
  amount_from=$2, amount_to=$3, required_level=$4, step_order=$5, is_active=$6
WHERE id=$1 AND deleted_at IS NULL RETURNING *;

-- name: SoftDeleteThreshold :execrows
UPDATE approval.approval_thresholds SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL;

-- name: CreateRequest :one
INSERT INTO approval.requests
  (type, office_id, amount, current_step, target_entity, target_id, payload, reason, requested_by_id)
VALUES (
  sqlc.arg(type),sqlc.arg(office_id),sqlc.arg(amount),1,
  sqlc.arg(target_entity),sqlc.arg(target_id),COALESCE(sqlc.arg(payload),'{}')::jsonb,
  sqlc.arg(reason),sqlc.arg(requested_by_id)
) RETURNING *;

-- name: GetRequest :one
SELECT * FROM approval.requests WHERE id=$1 AND deleted_at IS NULL;

-- name: GetRequestForUpdate :one
SELECT * FROM approval.requests WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ListRequests :many
SELECT * FROM approval.requests
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.request_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(type)::shared.request_type IS NULL OR type = sqlc.narg(type))
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountRequests :one
SELECT count(*) FROM approval.requests
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.request_status IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(type)::shared.request_type IS NULL OR type = sqlc.narg(type))
  AND (sqlc.narg(requested_by)::uuid IS NULL OR requested_by_id = sqlc.narg(requested_by));

-- name: SetRequestDecision :one
UPDATE approval.requests SET status=$2, decided_by_id=$3, decision_note=$4, decided_at=now()
WHERE id=$1 AND deleted_at IS NULL RETURNING *;

-- name: AdvanceRequestStep :one
UPDATE approval.requests SET current_step=current_step+1
WHERE id=$1 AND deleted_at IS NULL RETURNING *;

-- name: CancelRequest :one
UPDATE approval.requests SET status='cancelled'
WHERE id=$1 AND requested_by_id=$2 AND status='pending' AND deleted_at IS NULL RETURNING *;

-- name: CreateRequestApproval :one
INSERT INTO approval.request_approvals (request_id, step_order, required_level)
VALUES ($1,$2,$3) RETURNING *;

-- name: ListRequestApprovals :many
SELECT * FROM approval.request_approvals
WHERE request_id=$1 AND deleted_at IS NULL ORDER BY step_order;

-- name: DecideRequestApproval :one
UPDATE approval.request_approvals SET approver_id=$3, decision=$4, note=$5, decided_at=now()
WHERE request_id=$1 AND step_order=$2 AND deleted_at IS NULL RETURNING *;

-- name: ListInboxCandidates :many
SELECT * FROM approval.requests
WHERE deleted_at IS NULL AND status='pending'
ORDER BY created_at ASC;

-- Enriched read variants: request row + resolved maker/role/office names.
-- LEFT JOINs keep rows visible even when the user/office was soft-deleted.

-- name: GetRequestEnriched :one
SELECT sqlc.embed(r),
       u.name  AS requested_by_name,
       ro.name AS requested_by_role,
       o.name  AS office_name
FROM approval.requests r
LEFT JOIN identity.users u    ON u.id = r.requested_by_id AND u.deleted_at IS NULL
LEFT JOIN identity.roles ro   ON ro.id = u.role_id        AND ro.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = r.office_id       AND o.deleted_at IS NULL
WHERE r.id = $1 AND r.deleted_at IS NULL;

-- name: ListRequestsEnriched :many
SELECT sqlc.embed(r),
       u.name  AS requested_by_name,
       ro.name AS requested_by_role,
       o.name  AS office_name
FROM approval.requests r
LEFT JOIN identity.users u    ON u.id = r.requested_by_id AND u.deleted_at IS NULL
LEFT JOIN identity.roles ro   ON ro.id = u.role_id        AND ro.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = r.office_id       AND o.deleted_at IS NULL
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR r.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.request_status IS NULL OR r.status = sqlc.narg(status))
  AND (sqlc.narg(type)::shared.request_type IS NULL OR r.type = sqlc.narg(type))
  AND (sqlc.narg(requested_by)::uuid IS NULL OR r.requested_by_id = sqlc.narg(requested_by))
ORDER BY r.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ListInboxCandidatesEnriched :many
SELECT sqlc.embed(r),
       u.name  AS requested_by_name,
       ro.name AS requested_by_role,
       o.name  AS office_name
FROM approval.requests r
LEFT JOIN identity.users u    ON u.id = r.requested_by_id AND u.deleted_at IS NULL
LEFT JOIN identity.roles ro   ON ro.id = u.role_id        AND ro.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = r.office_id       AND o.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'pending'
ORDER BY r.created_at ASC;

-- name: ListRequestApprovalsEnriched :many
SELECT sqlc.embed(a), u.name AS approver_name
FROM approval.request_approvals a
LEFT JOIN identity.users u ON u.id = a.approver_id AND u.deleted_at IS NULL
WHERE a.request_id = $1 AND a.deleted_at IS NULL
ORDER BY a.step_order;
