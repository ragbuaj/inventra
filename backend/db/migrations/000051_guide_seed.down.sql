-- Revert 000051: buang sembilan modul bawaan beserta lampiran yang sempat
-- ditambahkan padanya.
--
-- Penghapusan lampiran diperlukan karena FK guide_attachments.module_id
-- menghalangi penghapusan modul. Ini memang destruktif: lampiran yang diunggah
-- pengelola ke modul bawaan ikut hilang. Objek di MinIO sengaja tidak disentuh —
-- lihat bagian "Cara membatalkan" pada rencana.

DELETE FROM guide.guide_attachments a
USING guide.guide_modules m
WHERE a.module_id = m.id
  AND m.slug IN (
    'login-security',
    'dashboard',
    'asset-catalog',
    'loan-assignment',
    'request-approval',
    'maintenance',
    'transfer-opname-disposal',
    'report-depreciation',
    'master-data-settings'
  );

DELETE FROM guide.guide_modules
WHERE slug IN (
  'login-security',
  'dashboard',
  'asset-catalog',
  'loan-assignment',
  'request-approval',
  'maintenance',
  'transfer-opname-disposal',
  'report-depreciation',
  'master-data-settings'
);
