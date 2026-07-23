-- Kolom paritas aset (spec 2026-07-23 legacy-parity, Fase 1):
--   capacity          — spesifikasi bebas (mis. "2 PK"); 1 row = 1 aset
--   lease_date        — tanggal sewa (aset sewa)
--   installation_date — tanggal instalasi
--   warranty_start    — awal garansi (warranty_expiry sudah ada)
--   floor_id          — lokasi bisa berhenti di lantai (tanpa ruangan)
--   pic_employee_id   — penanggung jawab (PIC), berbeda dari pemegang
ALTER TABLE asset.assets
  ADD COLUMN capacity          text,
  ADD COLUMN lease_date        date,
  ADD COLUMN installation_date date,
  ADD COLUMN warranty_start    date,
  ADD COLUMN floor_id          uuid REFERENCES masterdata.floors (id),
  ADD COLUMN pic_employee_id   uuid REFERENCES masterdata.employees (id);
CREATE INDEX idx_assets_floor_id ON asset.assets (floor_id);
CREATE INDEX idx_assets_pic      ON asset.assets (pic_employee_id);

-- Lokasi boleh berhenti di lantai: tangible wajib floor_id ATAU room_id.
-- (Intangible tetap bebas keduanya.) Aset tangible eksisting selalu punya room_id
-- (constraint lama mewajibkannya), jadi tak ada baris yang melanggar constraint baru.
ALTER TABLE asset.assets DROP CONSTRAINT chk_assets_tangible_room;
ALTER TABLE asset.assets ADD CONSTRAINT chk_assets_tangible_location
  CHECK (asset_class = 'intangible' OR floor_id IS NOT NULL OR room_id IS NOT NULL);
