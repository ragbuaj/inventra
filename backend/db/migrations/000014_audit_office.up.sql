-- Add the entity's office to each audit row so the audit view can be office-scoped
-- (reuses the same office-subtree data-scope model as the rest of the app).
-- Nullable: global actions (master data with no office, e.g. categories/reference) carry none.
-- No FK to masterdata.offices on purpose: audit is append-only and must outlive office deletion.
ALTER TABLE audit.audit_logs ADD COLUMN office_id uuid;
CREATE INDEX idx_audit_office ON audit.audit_logs (office_id);
