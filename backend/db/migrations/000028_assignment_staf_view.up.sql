-- Fix: grant Staf the `assignment.view` permission it needs to list its own
-- active assignments — required by the Maintenance module's "Laporan
-- Kerusakan" asset picker (`GET /assignments?status=active`), which a Staf
-- caller could not use at all (403) despite already having an 'office'-level
-- data-scope row for the 'assignments' module (seeded in 000026 "for future
-- delegation"). Found while building the Maintenance module e2e (task 13):
-- a real Staf user reporting damage on an asset they hold had no way to
-- select that asset, because the picker's underlying list call was gated
-- behind a permission Staf never received.
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, 'assignment.view'
FROM identity.roles r
WHERE r.deleted_at IS NULL
  AND r.name = 'Staf'
ON CONFLICT DO NOTHING;
