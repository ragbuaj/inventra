-- Nomor urut kode aset harus terikat pada kantor PENERBIT tag, bukan kantor
-- tempat aset berada saat ini.
--
-- Bug yang diperbaiki: sejak 000040 nomor urut diturunkan dari
-- MAX(tag_seq) WHERE office_id = <kantor>. Mutasi (transfer) memindahkan aset ke
-- kantor lain dan membawa serta tag_seq-nya, sehingga MAX kantor asal TURUN dan
-- pembuatan aset berikutnya MENERBITKAN ULANG nomor itu. Bila kategori & tahun
-- kebetulan sama, asset_tag yang dihasilkan identik dengan tag aset yang sudah
-- dimutasi -> pelanggaran unique (23505). Selain itu ini melanggar aturan spec
-- "kode aset lama tidak boleh dipakai ulang".
--
-- tag_office_id diisi saat aset dibuat dan TIDAK PERNAH diubah oleh mutasi, jadi
-- deret nomor tiap kantor selalu maju.
ALTER TABLE asset.assets
  ADD COLUMN tag_office_id uuid REFERENCES masterdata.offices (id);

-- Backfill: untuk data lama, kantor saat ini adalah perkiraan terbaik yang ada
-- (aset yang belum pernah dimutasi memang masih di kantor penerbitnya).
UPDATE asset.assets SET tag_office_id = office_id WHERE tag_office_id IS NULL;

-- Menopang GetMaxTagSeqForOffice (MAX per kantor penerbit).
CREATE INDEX idx_assets_tag_office_seq ON asset.assets (tag_office_id, tag_seq);
