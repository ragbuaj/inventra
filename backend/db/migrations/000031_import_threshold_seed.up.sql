-- Migration 000031: Seed approval.approval_thresholds for request_type = 'asset_import'.
--
-- The bulk-import module submits an 'asset_import' approval request when a batch is
-- confirmed (see internal/import module), but approval.Submit requires a matching
-- approval_thresholds row for the request_type or it returns ErrNoThreshold. No tiers
-- existed for 'asset_import', so every batch-approval submission failed.
--
-- Mirror the existing 'asset_create' tiers (added in 000016_office_tier.up.sql) exactly:
-- a batch worth X should route through the same approval chain as a single asset worth X,
-- since a bulk import is conceptually N asset_create requests collapsed into one approval.
--
-- ON CONFLICT guards idempotency against the partial unique index
-- uq_apprthr_type_from_step on (request_type, amount_from, step_order) WHERE deleted_at IS NULL.
INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order) VALUES
  ('asset_import', 0,          10000000,  'office',  1),
  ('asset_import', 10000000,   100000000, 'office',  1),
  ('asset_import', 10000000,   100000000, 'wilayah', 2),
  ('asset_import', 100000000,  NULL,      'office',  1),
  ('asset_import', 100000000,  NULL,      'wilayah', 2),
  ('asset_import', 100000000,  NULL,      'pusat',   3)
ON CONFLICT (request_type, amount_from, step_order) WHERE deleted_at IS NULL DO NOTHING;
