DELETE FROM identity.data_scope_policies WHERE module = 'depreciation';
DELETE FROM identity.role_permissions WHERE permission_key IN ('depreciation.view', 'depreciation.manage');
DELETE FROM identity.app_settings WHERE key = 'depreciation.accumulated_gl_account';
DROP INDEX IF EXISTS depreciation.idx_depr_basis_period;
DROP TABLE IF EXISTS depreciation.depreciation_periods;
DROP TYPE IF EXISTS shared.depreciation_period_status;
