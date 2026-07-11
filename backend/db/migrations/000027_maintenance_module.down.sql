DELETE FROM approval.approval_thresholds WHERE request_type = 'maintenance';
DELETE FROM identity.data_scope_policies WHERE module = 'maintenance';
DELETE FROM identity.role_permissions WHERE permission_key = 'maintenance.view';
DROP INDEX IF EXISTS stockopname.idx_opnitem_followup_record;
ALTER TABLE stockopname.stock_opname_items DROP COLUMN IF EXISTS followup_record_id;
DROP INDEX IF EXISTS maintenance.idx_mrec_schedule_id;
ALTER TABLE maintenance.maintenance_records DROP COLUMN IF EXISTS schedule_id;
