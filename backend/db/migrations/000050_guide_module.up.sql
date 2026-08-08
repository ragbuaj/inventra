-- Guide module: konten Panduan Penggunaan berbasis data + lampiran media.
-- Lihat docs/superpowers/specs/2026-08-08-guide-media-cms-design.md dan
-- docs/superpowers/plans/2026-08-08-guide-media-cms.md.
--
-- Isi panduan pindah dari berkas i18n frontend ke tabel ini supaya bisa diubah
-- tanpa rilis kode. Seed sembilan modul yang ada hari ini menyusul di 000051:
-- DDL dan konten sengaja dipisah supaya konten bisa di-seed ulang atau
-- dibatalkan tanpa menyentuh struktur.

CREATE SCHEMA IF NOT EXISTS guide;

CREATE TYPE shared.guide_status          AS ENUM ('draft', 'published');
CREATE TYPE shared.guide_attachment_kind AS ENUM ('video', 'document');

-- Satu modul = satu kartu di halaman /guide.
--
-- Dua bahasa disimpan sebagai kolom berpasangan pada baris yang sama, bukan
-- tabel terjemahan: *_id wajib, *_en opsional. Server mengirim keduanya dan
-- klien yang memilih beserta fallback-nya, sehingga pergantian locale tidak
-- menuntut permintaan ulang.
CREATE TABLE guide.guide_modules (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  -- Kunci stabil untuk seed idempoten dan penautan dalam. Tidak dipakai user.
  slug         text NOT NULL,
  icon         text NOT NULL,
  sort_order   integer NOT NULL DEFAULT 0,
  status       shared.guide_status NOT NULL DEFAULT 'draft',
  published_at timestamptz,
  title_id     text NOT NULL,
  title_en     text,
  body_id      text,
  body_en      text,
  -- Daftar langkah bernomor: [{"text_id": "...", "text_en": "..."}].
  -- jsonb dan bukan tabel anak karena langkah selalu diambil bersama modulnya,
  -- disunting sebagai satu daftar utuh, dan tidak pernah dikueri satuan. Bentuk
  -- isinya divalidasi di Go; CHECK di sini hanya menjaga tipe terluarnya.
  steps        jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_by   uuid REFERENCES identity.users (id),
  updated_by   uuid REFERENCES identity.users (id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT guide_modules_steps_is_array CHECK (jsonb_typeof(steps) = 'array')
);

CREATE UNIQUE INDEX uq_guide_modules_slug
  ON guide.guide_modules (slug)
  WHERE deleted_at IS NULL;

-- Kueri pembaca: modul terbit, urut tampil.
CREATE INDEX idx_guide_modules_reader
  ON guide.guide_modules (status, sort_order)
  WHERE deleted_at IS NULL;

CREATE TRIGGER trg_guide_modules_set_updated BEFORE UPDATE ON guide.guide_modules
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Lampiran modul: tautan video YouTube atau berkas PDF di MinIO.
CREATE TABLE guide.guide_attachments (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  module_id         uuid NOT NULL REFERENCES guide.guide_modules (id),
  kind              shared.guide_attachment_kind NOT NULL,
  title_id          text NOT NULL,
  title_en          text,
  sort_order        integer NOT NULL DEFAULT 0,
  -- kind = 'video': hanya identitas video yang disimpan, bukan URL mentah dari
  -- input. Sematan dibangun server ke youtube-nocookie.com dari identitas ini.
  youtube_id        text,
  -- kind = 'document': kunci objek dibangkitkan sistem. Nama berkas dari
  -- pengunggah hanya metadata tampilan dan tidak pernah masuk ke path objek.
  object_key        text,
  original_filename text,
  mime_type         text,
  size_bytes        bigint,
  created_by        uuid REFERENCES identity.users (id),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  -- Memaksakan bentuk per jenis: satu lampiran tidak bisa sekaligus video dan
  -- dokumen, dan tidak bisa kosong keduanya.
  CONSTRAINT guide_attachments_shape CHECK (
       (kind = 'video'    AND youtube_id IS NOT NULL AND object_key IS NULL)
    OR (kind = 'document' AND object_key IS NOT NULL AND youtube_id IS NULL)
  )
);

CREATE INDEX idx_guide_attachments_module
  ON guide.guide_attachments (module_id, sort_order)
  WHERE deleted_at IS NULL;

CREATE TRIGGER trg_guide_attachments_set_updated BEFORE UPDATE ON guide.guide_attachments
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Permission tulis untuk konten panduan. Di-seed hanya ke Superadmin; peran
-- lain diberi lewat menu RBAC tanpa perlu migration baru.
--
-- Sengaja TIDAK ada baris identity.data_scope_policies: panduan bersifat global
-- untuk seluruh organisasi dan tidak melewati CallerOfficeScope.
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('guide.manage')) AS p(key)
WHERE r.deleted_at IS NULL
  AND r.name = 'Superadmin'
ON CONFLICT DO NOTHING;
