ALTER TABLE asset.assets DROP CONSTRAINT chk_assets_tangible_location;
ALTER TABLE asset.assets ADD CONSTRAINT chk_assets_tangible_room
  CHECK (asset_class = 'intangible' OR room_id IS NOT NULL);

DROP INDEX IF EXISTS asset.idx_assets_pic;
DROP INDEX IF EXISTS asset.idx_assets_floor_id;

ALTER TABLE asset.assets
  DROP COLUMN pic_employee_id,
  DROP COLUMN floor_id,
  DROP COLUMN warranty_start,
  DROP COLUMN installation_date,
  DROP COLUMN lease_date,
  DROP COLUMN capacity;
