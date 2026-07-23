-- Penomoran kode aset baru (spec 2026-07-23 legacy-parity, Fase 2).
-- Format asset_tag: {KODE_KANTOR}{KODE_KATEGORI}{TAHUN_BELI}{NNNNN} tanpa tanda '-'.
-- Sequence NNNNN kini PER-KANTOR (bukan per kantor+kategori+tahun), diturunkan dari
-- MAX(tag_seq)+1 sehingga: soft-delete menahan nomor (ikut MAX), hard-delete baris
-- teratas menurunkan MAX (nomor bisa dipakai lagi hanya untuk yang teratas).
-- Menggantikan tabel counter asset.asset_tag_counters.

ALTER TABLE asset.assets ADD COLUMN tag_seq int;
CREATE INDEX idx_assets_office_tagseq ON asset.assets (office_id, tag_seq);

-- Backfill tag_seq per kantor (termasuk baris soft-delete agar nomornya tertahan)
-- + re-tag semua aset eksisting ke format baru. Data pilot minimal & belum dipakai
-- nyata (label belum dicetak), jadi penomoran ulang aman.
WITH ranked AS (
  SELECT a.id, a.office_id, a.category_id, a.purchase_date, a.created_at,
         row_number() OVER (PARTITION BY a.office_id ORDER BY a.created_at, a.id) AS seq
  FROM asset.assets a
)
UPDATE asset.assets a
SET tag_seq   = r.seq,
    asset_tag = o.code || COALESCE(c.code, '') ||
                to_char(COALESCE(r.purchase_date, r.created_at::date), 'YYYY') ||
                lpad(r.seq::text, 5, '0')
FROM ranked r
JOIN masterdata.offices o    ON o.id = r.office_id
JOIN masterdata.categories c ON c.id = r.category_id
WHERE a.id = r.id;

-- tag_seq sengaja NULLABLE: jalur create auto/import selalu mengisinya (non-null),
-- tapi INSERT langsung (mis. seed/test/migrasi data) boleh NULL. COALESCE(MAX,0)
-- mengabaikan NULL, jadi jaminan "tak dipakai ulang" tetap berlaku untuk tag auto.

DROP TABLE asset.asset_tag_counters;
