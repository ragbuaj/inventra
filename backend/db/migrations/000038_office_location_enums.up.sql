-- Enums baru untuk fase legacy-parity (spec 2026-07-23):
--   office_ownership       — status kepemilikan kantor (dipakai migrasi kantor)
--   office_kind            — konvensional/syariah (dipakai migrasi kantor)
--   location_change_source — sumber perubahan lokasi aset (dipakai tabel history)
-- Didefinisikan lebih dulu di satu migrasi sesuai rencana fase; konsumennya menyusul.
CREATE TYPE shared.office_ownership       AS ENUM ('sewa', 'milik', 'hg_pakai', 'free');
CREATE TYPE shared.office_kind            AS ENUM ('konvensional', 'syariah');
CREATE TYPE shared.location_change_source AS ENUM ('registration', 'edit', 'transfer', 'migration');
