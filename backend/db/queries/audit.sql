-- Audit log: append-only writes + an office-scoped, filterable read model.
-- all_scope bypasses the office filter (global scope); otherwise only rows whose
-- office_id is in office_ids are returned. NULL-office (global) rows are visible
-- only to all-scope callers.

-- name: InsertAuditLog :one
INSERT INTO audit.audit_logs (actor_id, entity_type, entity_id, action, changes, ip, office_id)
VALUES (
  sqlc.narg(actor_id),
  sqlc.arg(entity_type),
  sqlc.arg(entity_id),
  sqlc.arg(action),
  sqlc.narg(changes),
  sqlc.narg(ip),
  sqlc.narg(office_id)
)
RETURNING *;

-- name: ListAuditLogs :many
SELECT
  a.*,
  u.name  AS actor_name,
  u.email AS actor_email,
  ro.name AS actor_role,
  o.name  AS office_name
FROM audit.audit_logs a
LEFT JOIN identity.users u ON u.id = a.actor_id
LEFT JOIN identity.roles ro ON ro.id = u.role_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id
WHERE (sqlc.arg(all_scope)::bool OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR a.actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(entity_type)::text IS NULL OR a.entity_type = sqlc.narg(entity_type))
  AND (sqlc.narg(action)::shared.audit_action IS NULL OR a.action = sqlc.narg(action))
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR a.created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR a.created_at <= sqlc.narg(to_ts))
  AND (
    sqlc.arg(search)::text = ''
    OR a.entity_type ILIKE '%' || sqlc.arg(search) || '%'
    OR a.entity_id::text ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY a.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAuditLogs :one
SELECT count(*)
FROM audit.audit_logs a
WHERE (sqlc.arg(all_scope)::bool OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR a.actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(entity_type)::text IS NULL OR a.entity_type = sqlc.narg(entity_type))
  AND (sqlc.narg(action)::shared.audit_action IS NULL OR a.action = sqlc.narg(action))
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR a.created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR a.created_at <= sqlc.narg(to_ts))
  AND (
    sqlc.arg(search)::text = ''
    OR a.entity_type ILIKE '%' || sqlc.arg(search) || '%'
    OR a.entity_id::text ILIKE '%' || sqlc.arg(search) || '%'
  );
