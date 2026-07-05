-- Impairment resume floor (fixes the double-depreciation / reverted-impairment
-- bugs). `book_value` is DERIVED state: refreshAssetSummary rewrites it to the
-- latest computed closing on EVERY compute, so it ratchets down and cannot be
-- used to detect "an impairment happened". `impaired_book_value` is a STABLE,
-- impairment-only column — written ONLY by an impairment write-down
-- (ApplyAssetImpairment) and read ONLY by the compute's commercial resumption
-- override — so an ordinary recompute never sees a spurious lower value.
-- Nullable: NULL means "never impaired". Spec:
-- docs/superpowers/specs/2026-07-05-depreciation-module-design.md.
ALTER TABLE asset.assets ADD COLUMN impaired_book_value numeric(18,2);
