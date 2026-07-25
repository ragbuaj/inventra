ALTER TABLE asset.assets
  DROP COLUMN IF EXISTS legacy_barcode,
  DROP COLUMN IF EXISTS legacy_asset_code,
  DROP COLUMN IF EXISTS spk_number,
  DROP COLUMN IF EXISTS is_operational_asset;
