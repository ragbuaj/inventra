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
