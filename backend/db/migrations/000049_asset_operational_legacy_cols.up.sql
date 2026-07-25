-- Kolom tambahan pada asset.assets:
--   is_operational_asset - klasifikasi aset operasional vs non-operasional (BUKAN
--                          status fungsional; lihat kolom status). Default true.
--   spk_number           - nomor Surat Perintah Kerja (dokumen berbeda dari po_number/PO).
--   legacy_asset_code    - kode aset dari sistem lama; arsip migrasi, TIDAK ditampilkan di Inventra.
--   legacy_barcode       - barcode dari sistem lama; arsip migrasi, TIDAK ditampilkan di Inventra.
ALTER TABLE asset.assets
  ADD COLUMN is_operational_asset boolean NOT NULL DEFAULT true,
  ADD COLUMN spk_number           text,
  ADD COLUMN legacy_asset_code    text,
  ADD COLUMN legacy_barcode       text;

COMMENT ON COLUMN asset.assets.is_operational_asset IS 'Klasifikasi aset operasional vs non-operasional (bukan status fungsional).';
COMMENT ON COLUMN asset.assets.spk_number IS 'Nomor Surat Perintah Kerja; berbeda dari po_number (Purchase Order).';
COMMENT ON COLUMN asset.assets.legacy_asset_code IS 'Kode aset dari sistem lama; arsip migrasi, tidak ditampilkan di Inventra.';
COMMENT ON COLUMN asset.assets.legacy_barcode IS 'Barcode dari sistem lama; arsip migrasi, tidak ditampilkan di Inventra.';
