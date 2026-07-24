ALTER TABLE asset.assets DROP CONSTRAINT chk_assets_tangible_location;
-- NOT VALID: rollback is lossy. Setelah fitur dipakai, bisa ada aset tangible
-- floor-only (room_id NULL, floor_id terisi) yang MELANGGAR constraint lama.
-- Memvalidasi ulang terhadap baris eksisting akan menggagalkan rollback; NOT VALID
-- memasang aturan untuk baris BARU tanpa memvalidasi yang lama. Normalisasi/tolak
-- baris floor-only secara manual bila validasi penuh diperlukan.
ALTER TABLE asset.assets ADD CONSTRAINT chk_assets_tangible_room
  CHECK (asset_class = 'intangible' OR room_id IS NOT NULL) NOT VALID;

DROP INDEX IF EXISTS asset.idx_assets_pic;
DROP INDEX IF EXISTS asset.idx_assets_floor_id;

ALTER TABLE asset.assets
  DROP COLUMN pic_employee_id,
  DROP COLUMN floor_id,
  DROP COLUMN warranty_start,
  DROP COLUMN installation_date,
  DROP COLUMN lease_date,
  DROP COLUMN capacity;
