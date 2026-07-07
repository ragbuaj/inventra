DELETE FROM identity.data_scope_policies WHERE module = 'stockopname';
DELETE FROM identity.role_permissions WHERE permission_key IN ('stockopname.view', 'stockopname.manage');
DROP INDEX IF EXISTS stockopname.idx_opnitem_followup;
ALTER TABLE stockopname.stock_opname_items DROP COLUMN IF EXISTS followup_request_id;
