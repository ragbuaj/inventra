-- Rapikan kebijakan masking kolom finansial aset agar KONSISTEN satu tier.
--
-- Latar: 000016 menyembunyikan purchase_cost/book_value/accumulated_depreciation
-- dari role non-privileged, tetapi memberi Manager akses purchase_cost + book_value
-- SAJA — accumulated_depreciation tetap masked untuk Manager. Itu tidak konsisten
-- dan maskingnya "bocor": secara akuntansi
--   accumulated_depreciation = purchase_cost - book_value - impairment_loss,
-- jadi Manager yang sudah melihat purchase_cost + book_value bisa menurunkan
-- accumulated sendiri. Menyembunyikan accumulated dari Manager tak menambah
-- kerahasiaan nyata, hanya membuat kebijakan tak seragam.
--
-- Keputusan: tiga kolom finansial dijadikan SATU tier — role yang boleh melihat
-- salah satunya boleh melihat ketiganya. Untuk role bawaan, itu berarti Manager
-- kini juga melihat accumulated_depreciation (setara Superadmin untuk ketiga
-- kolom). Role non-privileged (Kepala Unit/Kanwil, Staf) tetap ter-mask penuh.
UPDATE identity.field_permissions
SET can_view = true
WHERE entity = 'assets'
  AND field = 'accumulated_depreciation'
  AND deleted_at IS NULL
  AND role_id IN (SELECT id FROM identity.roles WHERE name = 'Manager' AND deleted_at IS NULL);
