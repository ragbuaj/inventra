-- Traceability: link a variance item to the approval request generated from it.
ALTER TABLE stockopname.stock_opname_items
  ADD COLUMN followup_request_id uuid REFERENCES approval.requests (id);
CREATE INDEX idx_opnitem_followup ON stockopname.stock_opname_items (followup_request_id);

-- Permissions: stockopname.manage (create/count/reconcile/close/follow-up) + stockopname.view (read).
-- Operational roles get both; Staf gets neither (PRD §2.1 "Kelola stock opname").
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('stockopname.manage'), ('stockopname.view')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name IN ('Superadmin', 'Manager', 'Kepala Kanwil', 'Kepala Unit')
ON CONFLICT DO NOTHING;

-- Data-scope for the 'stockopname' module (mirror 000023 depreciation pattern).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'stockopname', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
