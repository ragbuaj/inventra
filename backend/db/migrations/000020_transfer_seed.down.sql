DELETE FROM identity.data_scope_policies WHERE module = 'transfers';
DELETE FROM identity.role_permissions WHERE permission_key IN ('transfer.manage', 'transfer.view');
DELETE FROM approval.approval_thresholds WHERE request_type = 'asset_transfer';
