-- Reverse migration 000016: remove office_tier column + approval/asset seeds.

DELETE FROM identity.field_permissions WHERE entity = 'assets';
DELETE FROM identity.data_scope_policies WHERE module IN ('assets', 'requests');
DELETE FROM identity.role_permissions WHERE permission_key IN
  ('request.create', 'request.decide', 'approval.config.manage', 'asset.view', 'asset.manage');
DELETE FROM approval.approval_thresholds
  WHERE request_type IN ('asset_create', 'asset_disposal', 'valuation_exclusion');
ALTER TABLE masterdata.office_types DROP COLUMN tier;
