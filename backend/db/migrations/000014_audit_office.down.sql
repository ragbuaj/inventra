DROP INDEX IF EXISTS audit.idx_audit_office;
ALTER TABLE audit.audit_logs DROP COLUMN IF EXISTS office_id;
