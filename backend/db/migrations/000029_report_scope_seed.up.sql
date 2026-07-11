-- Reporting & Dashboard module: data-scope seed only.
-- Permissions report.view / report.export were already seeded in 000005
-- (report.view: all roles; report.export: all except Staf).

-- Data-scope for the 'report' module (mirror 'maintenance', 000027).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'report', (CASE
    WHEN r.name = 'Superadmin'                    THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit') THEN 'office_subtree'
    WHEN r.name = 'Manager'                       THEN 'office'
    ELSE 'own'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
