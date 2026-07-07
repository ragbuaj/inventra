-- Assignment module seed: assignment.view permission + approval band for peminjaman.
-- assignment.manage is already seeded (000005) for superadmin + manager.

-- Permissions: assignment.view (read).
-- Superadmin, Kepala Kanwil, Kepala Unit, Manager get both manage and view.
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('assignment.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Peminjaman is not value-tiered: a single office-level approval step.
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order)
VALUES ('assignment', 0, NULL, 'office', 1)
ON CONFLICT DO NOTHING;
