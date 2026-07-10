-- Maintenance module: schema links + seed. Tables exist since 000012.

-- Explicit record → schedule link: completing a linked record updates the
-- schedule's last_done/next_due (no implicit asset+category guessing).
ALTER TABLE maintenance.maintenance_records
  ADD COLUMN schedule_id uuid REFERENCES maintenance.maintenance_schedules (id);
CREATE INDEX idx_mrec_schedule_id ON maintenance.maintenance_records (schedule_id);

-- Traceability + idempotency for the stock-opname 'damaged' follow-up
-- (mirrors followup_request_id from 000025).
ALTER TABLE stockopname.stock_opname_items
  ADD COLUMN followup_record_id uuid REFERENCES maintenance.maintenance_records (id);
CREATE INDEX idx_opnitem_followup_record ON stockopname.stock_opname_items (followup_record_id);

-- Permissions: maintenance.view (read). maintenance.manage is already seeded
-- (000005) for Superadmin + Manager. Kepala get view (assignment.view precedent).
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('maintenance.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Data-scope for the 'maintenance' module (mirror 'assignments', 000026).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'maintenance', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- Damage report is not value-tiered: a single office-level approval step.
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order)
VALUES ('maintenance', 0, NULL, 'office', 1)
ON CONFLICT DO NOTHING;
