-- Authorization queries: office subtree (scoping) and field permissions.

-- name: GetOfficeSubtree :many
-- Returns an office plus all of its descendants (Pusat -> Wilayah -> Cabang -> Outlet).
WITH RECURSIVE subtree AS (
  SELECT o.id FROM masterdata.offices o WHERE o.id = $1 AND o.deleted_at IS NULL
  UNION ALL
  SELECT o.id
  FROM masterdata.offices o
  JOIN subtree s ON o.parent_id = s.id
  WHERE o.deleted_at IS NULL
)
SELECT id FROM subtree;

-- name: ListFieldPermissionsByRole :many
SELECT entity, field, can_view, can_edit
FROM identity.field_permissions
WHERE role_id = $1 AND deleted_at IS NULL;

-- name: ListUsersWithPermission :many
-- The inverse of the request-time permission check: given a permission key, who
-- holds it? Needed to fan notifications out to concrete users, because every
-- other authorization path here answers only "may THIS caller act?".
-- Callers must still apply scope and SoD by running the existing eligibility
-- predicate over each candidate -- deliberately not reimplemented in SQL, or the
-- rules would drift from approval.eligibleToDecide.
SELECT u.id, u.role_id, u.office_id
FROM identity.users u
JOIN identity.role_permissions rp
  ON rp.role_id = u.role_id AND rp.deleted_at IS NULL
WHERE rp.permission_key = @permission_key
  AND u.status = 'active'
  AND u.deleted_at IS NULL;
