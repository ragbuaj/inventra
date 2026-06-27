-- Reverse migration 000016: remove office_tier column + approval/asset seeds.

DELETE FROM identity.field_permissions WHERE entity = 'assets';
DELETE FROM identity.data_scope_policies WHERE module IN ('assets', 'requests');
-- Remove only the permission rows that 000016 actually inserted.
-- request.create is NOT deleted here: all target roles (superadmin, kepala_kanwil,
-- kepala_unit, manager, staf) already had it from 000005_seed_identity.up.sql, so
-- ON CONFLICT DO NOTHING in the up-migration inserted zero new rows for that key.
DELETE FROM identity.role_permissions WHERE permission_key IN
  ('request.decide', 'approval.config.manage', 'asset.view', 'asset.manage');
DELETE FROM approval.approval_thresholds
  WHERE request_type IN ('asset_create', 'asset_disposal', 'valuation_exclusion');
ALTER TABLE masterdata.office_types DROP COLUMN tier;
