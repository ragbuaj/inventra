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

-- Data-scope for the 'assignments' module (mirror 'disposals' from 000021). Keeps
-- parity with every other module: without an explicit row a scoped role would fall
-- back to its '*' default. The roles that hold assignment.view/manage today already
-- default to global/office_subtree, so this is defense-in-depth for future delegation.
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'assignments', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
