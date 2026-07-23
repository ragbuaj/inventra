-- History aset (spec 2026-07-23 legacy-parity, Fase 3):
--   asset_location_history — riwayat lokasi (kantor/lantai/ruangan) sepanjang waktu
--   asset_pic_history      — riwayat PIC (satu PIC aktif per aset)
-- Pemegang: TIDAK ada tabel baru — pakai assignment.assignments (modul Assignment).

CREATE TABLE asset.asset_location_history (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id    uuid NOT NULL REFERENCES asset.assets (id) ON DELETE CASCADE,
  office_id   uuid NOT NULL REFERENCES masterdata.offices (id),
  floor_id    uuid REFERENCES masterdata.floors (id),
  room_id     uuid REFERENCES masterdata.rooms (id),
  source      shared.location_change_source NOT NULL DEFAULT 'edit',
  moved_at    timestamptz NOT NULL DEFAULT now(),
  moved_by_id uuid REFERENCES identity.users (id),
  transfer_id uuid REFERENCES transfer.asset_transfers (id),
  note        text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE INDEX idx_asset_loc_hist_asset ON asset.asset_location_history (asset_id, moved_at DESC);
CREATE TRIGGER trg_asset_loc_hist_set_updated BEFORE UPDATE ON asset.asset_location_history
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE asset.asset_pic_history (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id        uuid NOT NULL REFERENCES asset.assets (id) ON DELETE CASCADE,
  pic_employee_id uuid NOT NULL REFERENCES masterdata.employees (id),
  assigned_at     timestamptz NOT NULL DEFAULT now(),
  released_at     timestamptz,
  assigned_by_id  uuid REFERENCES identity.users (id),
  note            text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
-- Satu PIC aktif (belum released) per aset.
CREATE UNIQUE INDEX uq_asset_pic_active ON asset.asset_pic_history (asset_id)
  WHERE released_at IS NULL AND deleted_at IS NULL;
CREATE INDEX idx_asset_pic_hist_asset ON asset.asset_pic_history (asset_id, assigned_at DESC);
CREATE TRIGGER trg_asset_pic_hist_set_updated BEFORE UPDATE ON asset.asset_pic_history
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Backfill satu baris lokasi awal per aset eksisting.
INSERT INTO asset.asset_location_history (asset_id, office_id, floor_id, room_id, source, moved_at)
SELECT a.id, a.office_id, a.floor_id, a.room_id, 'migration', a.created_at
FROM asset.assets a WHERE a.deleted_at IS NULL;

-- Backfill PIC aktif untuk aset yang sudah punya pic_employee_id.
INSERT INTO asset.asset_pic_history (asset_id, pic_employee_id, assigned_at)
SELECT a.id, a.pic_employee_id, a.created_at
FROM asset.assets a WHERE a.deleted_at IS NULL AND a.pic_employee_id IS NOT NULL;
