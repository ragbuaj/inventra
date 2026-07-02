DELETE FROM identity.data_scope_policies WHERE module = 'disposals';
DELETE FROM identity.role_permissions WHERE permission_key IN ('disposal.manage', 'disposal.view');
