-- Seed label boilerplate rows for asset labels (company name + disclaimer).
-- Idempotent: skips insert if a non-deleted row with the same key already exists.
INSERT INTO identity.app_settings (key, value, value_type, description)
SELECT 'label.company_name',
       'PT Bank Tabungan Negara (Persero) Tbk',
       'string',
       'Company name printed on asset labels'
WHERE NOT EXISTS (
  SELECT 1 FROM identity.app_settings
  WHERE key = 'label.company_name' AND deleted_at IS NULL
);

INSERT INTO identity.app_settings (key, value, value_type, description)
SELECT 'label.disclaimer',
       'Tidak Untuk Diperjualbelikan & Apabila Dipindah posisi untuk disampaikan ke Pengelola Gedung',
       'string',
       'Disclaimer text printed on asset labels'
WHERE NOT EXISTS (
  SELECT 1 FROM identity.app_settings
  WHERE key = 'label.disclaimer' AND deleted_at IS NULL
);
