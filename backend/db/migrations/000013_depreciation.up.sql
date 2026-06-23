-- Depreciation read model — `depreciation` schema. See docs/DATABASE.md §4.4 and PRD §3.5.
-- One row per asset per monthly period (period = first day of month).

CREATE SCHEMA IF NOT EXISTS depreciation;

CREATE TABLE depreciation.depreciation_entries (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id            uuid NOT NULL REFERENCES asset.assets (id),
  period              date NOT NULL,
  opening_value       numeric(18,2) NOT NULL,
  depreciation_amount numeric(18,2) NOT NULL,
  closing_value       numeric(18,2) NOT NULL,
  method              shared.depreciation_method NOT NULL,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz
);
CREATE UNIQUE INDEX uq_depr_asset_period ON depreciation.depreciation_entries (asset_id, period) WHERE deleted_at IS NULL;
CREATE INDEX idx_depr_asset_id ON depreciation.depreciation_entries (asset_id);
CREATE TRIGGER trg_depr_set_updated BEFORE UPDATE ON depreciation.depreciation_entries
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
