-- Konten Panduan Penggunaan. Panduan bersifat global: tidak ada penyaringan
-- office scope di mana pun berkas ini, dan itu memang disengaja (lihat
-- docs/superpowers/plans/2026-08-08-guide-media-cms.md).

-- Satu kueri untuk pembaca dan pengelola. @include_draft = false menyisakan
-- modul terbit saja; handler hanya menyalakannya untuk pemanggil yang memegang
-- guide.manage, sehingga draf tidak pernah bocor lewat parameter.
-- name: ListGuideModules :many
SELECT * FROM guide.guide_modules
WHERE deleted_at IS NULL
  AND (@include_draft::boolean OR status = 'published')
ORDER BY sort_order, created_at;

-- name: GetGuideModule :one
SELECT * FROM guide.guide_modules
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateGuideModule :one
INSERT INTO guide.guide_modules
  (slug, icon, sort_order, status, published_at, title_id, title_en, body_id, body_en, steps, created_by, updated_by)
VALUES (
  @slug, @icon, @sort_order, @status,
  CASE WHEN @status::shared.guide_status = 'published' THEN now() END,
  @title_id, @title_en, @body_id, @body_en, @steps, @actor, @actor
)
RETURNING *;

-- published_at menandai penerbitan PERTAMA dan tidak diperbarui saat modul yang
-- sudah terbit disunting lagi; menariknya kembali ke draf mengosongkannya, jadi
-- penerbitan berikutnya mencatat waktu barunya sendiri.
-- name: UpdateGuideModule :one
UPDATE guide.guide_modules SET
  slug         = @slug,
  icon         = @icon,
  sort_order   = @sort_order,
  status       = @status,
  published_at = CASE
                   WHEN @status::shared.guide_status = 'published'
                     THEN COALESCE(published_at, now())
                   ELSE NULL
                 END,
  title_id     = @title_id,
  title_en     = @title_en,
  body_id      = @body_id,
  body_en      = @body_en,
  steps        = @steps,
  updated_by   = @actor
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- Lampiran ikut ditandai terhapus dalam transaksi yang sama oleh service, bukan
-- oleh cascade: baris lampiran perlu tetap ada supaya objek MinIO-nya masih bisa
-- ditelusuri kalau penghapusan objek gagal.
-- name: SoftDeleteGuideModule :one
UPDATE guide.guide_modules SET deleted_at = now(), updated_by = @actor
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteGuideModuleAttachments :many
UPDATE guide.guide_attachments SET deleted_at = now()
WHERE module_id = @module_id AND deleted_at IS NULL
RETURNING *;

-- Dipakai daftar modul: satu kueri untuk seluruh modul yang tampil, bukan satu
-- kueri per modul.
-- name: ListGuideAttachmentsByModules :many
SELECT * FROM guide.guide_attachments
WHERE module_id = ANY(@module_ids::uuid[]) AND deleted_at IS NULL
ORDER BY module_id, sort_order, created_at;

-- name: GetGuideAttachment :one
SELECT * FROM guide.guide_attachments
WHERE id = $1 AND deleted_at IS NULL;

-- Dipakai endpoint penyajian berkas. Status modul induk ikut disaring DI DALAM
-- kueri, bukan diperiksa pada hasilnya: lampiran milik modul draf tidak
-- dikembalikan sama sekali kecuali @include_draft menyala, dan handler hanya
-- menyalakannya untuk pemanggil yang memegang guide.manage. Tanpa baris,
-- pemanggil menerima 404 yang sama persis dengan id yang tidak ada — keberadaan
-- draf pun tidak terbocorkan.
-- name: GetGuideAttachmentForRead :one
SELECT a.* FROM guide.guide_attachments a
JOIN guide.guide_modules m ON m.id = a.module_id
WHERE a.id = @id
  AND a.deleted_at IS NULL
  AND m.deleted_at IS NULL
  AND (@include_draft::boolean OR m.status = 'published');

-- Menegakkan batas jumlah lampiran per modul. Dihitung di dalam transaksi yang
-- sama dengan penyisipan supaya dua permintaan bersamaan tidak sama-sama lolos.
-- name: CountGuideAttachments :one
SELECT count(*) FROM guide.guide_attachments
WHERE module_id = $1 AND deleted_at IS NULL;

-- name: CreateGuideAttachment :one
INSERT INTO guide.guide_attachments
  (module_id, kind, title_id, title_en, sort_order, youtube_id, object_key, original_filename, mime_type, size_bytes, created_by)
VALUES (
  @module_id, @kind, @title_id, @title_en, @sort_order,
  @youtube_id, @object_key, @original_filename, @mime_type, @size_bytes, @actor
)
RETURNING *;

-- Hanya judul dan urutan. Berkas maupun tautan tidak bisa diganti di tempat:
-- untuk itu lampirannya dihapus lalu dibuat ulang, sehingga tidak pernah ada
-- baris yang menunjuk objek MinIO yang sudah bukan miliknya.
-- name: UpdateGuideAttachment :one
UPDATE guide.guide_attachments SET
  title_id   = @title_id,
  title_en   = @title_en,
  sort_order = @sort_order
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteGuideAttachment :one
UPDATE guide.guide_attachments SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING *;
