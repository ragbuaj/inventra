# Asset Documents (BAST) — Design

| | |
|---|---|
| **Tanggal** | 2026-06-28 |
| **Modul** | `internal/asset` (sub-fitur dokumen) |
| **Schema** | `asset.asset_documents` (migrasi `000015_fam_tables`, sudah ada) |
| **Status** | Disetujui — siap implementasi |

## 1. Konteks & tujuan

PRD bagian 3.10 (FR-10.1–10.3): transaksi perolehan/mutasi/penghapusan punya **nomor BAST** dan
dokumen pendukung di MinIO; entitas `asset_documents` menautkan dokumen ke aset & transaksi
terkait (jenis, nomor, tanggal, pihak, berkas), dan **mengikuti hak akses & scope aset** dengan
perubahan tercatat di audit trail.

Tabel `asset.asset_documents` **sudah dibuat** (migrasi `000015`):

```
id, asset_id (FK assets ON DELETE CASCADE), doc_type (shared.asset_document_type:
bast_acquisition/bast_transfer/bast_disposal/invoice/contract/other), doc_no, doc_date,
counterparty, object_key (file MinIO, nullable), related_request_id (FK approval.requests),
related_transfer_id (FK transfer.asset_transfers), related_disposal_id (FK disposal.disposals),
created_by_id (FK users), created_at/updated_at/deleted_at
```

Interface `internal/storage` (MinIO) dan modul **attachment** (`internal/asset/attachment*.go`)
sudah ada dan menjadi analog terdekat — desain ini meniru polanya, dengan perbedaan: dokumen
**metadata-first** (file pendukung **opsional**) dan punya metadata + tautan transaksi.

## 2. Lingkup (increment ini)

**CRUD dokumen mandiri per-aset, sekarang.** Modul `transfer` & `disposal` belum dibangun,
jadi tautan `related_transfer_id`/`related_disposal_id` **ditunda** sampai modul itu ada.
Increment ini menerima `asset_id` + metadata + file opsional + `related_request_id` opsional.

Keputusan minor (mengikuti pola repo):
- **Izin**: pakai ulang `asset.view` / `asset.manage` + scope per-aset (sama seperti attachment).
- **Whitelist file**: sama dengan attachment (`pdf/jpeg/png/webp`, ≤ `ATTACHMENT_MAX_BYTES`).
- **Tanpa thumbnail** (dokumen = bukti, bukan galeri).
- **Tanpa endpoint detach-file** (ganti berkas & hapus dokumen sudah mencakup kebutuhan).

## 3. Penempatan file

Sub-fitur modul `asset` (seperti attachment & barcode) — **bukan** package baru. File baru di
`internal/asset/`:

- `document.go` — metode service di atas `*Service` yang sudah ada: `CreateDocument`,
  `ListDocuments`, `GetDocument`, `UpdateDocument`, `DeleteDocument`, `AttachFile`,
  `OpenDocumentFile`.
- `document_handler.go` — handler HTTP; pakai ulang `resolveAssetInScope`, `contentDisposition`,
  `handleErr`.
- `document_dto.go` — request DTO (`binding` tags) + `documentToMap`.
- Query di-append ke `db/queries/assets.sql`, lalu `sqlc generate`.
- Route ditambah di `internal/asset/routes.go`.

## 4. Endpoint

Semua nested di `/assets/:id`, dengan `authMW` + scope per-aset (office aset ∈ scope caller).

| Method | Path | Izin | Fungsi |
|---|---|---|---|
| POST | `/assets/:id/documents` | manage | Buat dokumen (JSON metadata, tanpa file) → 201 |
| GET | `/assets/:id/documents` | view | List dokumen aset → `{data, total}` |
| GET | `/assets/:id/documents/:docId` | view | Detail satu dokumen |
| PUT | `/assets/:id/documents/:docId` | manage | Edit metadata |
| DELETE | `/assets/:id/documents/:docId` | manage | Soft-delete + hapus objek MinIO (best-effort) → 204 |
| PUT | `/assets/:id/documents/:docId/file` | manage | Unggah/ganti berkas (multipart `file`) |
| GET | `/assets/:id/documents/:docId/file` | view | Proxy unduh berkas |

`:docId` diverifikasi milik aset `:id` (mirip attachment) — kalau tidak → 404.

## 5. Validasi & perilaku data

- `doc_type` **wajib**: `binding:"required,oneof=bast_acquisition bast_transfer bast_disposal invoice contract other"` → 400 bila invalid.
- `doc_no`, `counterparty` — opsional string.
- `doc_date` — opsional string `YYYY-MM-DD`, diparse ke `pgtype.Date`; format salah → 400.
- `related_request_id` — opsional UUID; FK invalid (`23503`) → `ErrInvalidRef` (400).
- **File**:
  - MIME whitelist + batas ukuran sama dengan attachment (`allowedMIME`/`extFor`, `s.maxBytes`).
  - Object key: `assets/{assetID}/documents/{docID}.{ext}`.
  - Ganti berkas → simpan objek baru, update `object_key`, hapus objek lama (best-effort).
  - Tabel **tak punya** kolom filename/mime/size → nama unduhan diturunkan dari `doc_no`
    (di-sanitasi) atau `doc_type`, ekstensi dari content-type objek; content-type dari metadata
    objek MinIO (`storage.ObjectInfo`).
  - Unduh: header `X-Content-Type-Options: nosniff` + `Content-Security-Policy: sandbox` +
    `Content-Disposition` aman (pakai ulang `contentDisposition`).
  - GET `/file` saat `object_key` NULL → 404 (`ErrNotFound`).

## 6. Lintas-cutting

- **Audit**: `audit.Record` pada entity `"asset_documents"` untuk create/update/delete/upload-file,
  `office_id` = office aset, dengan before/after diff (`audit.Diff`).
- **Rollback**: bila DB gagal setelah `store.Put`, hapus objek yang baru diunggah (pola attachment).
- **Sentinel error** pakai ulang: `ErrNotFound`, `ErrInvalidRef`, `ErrUnsupportedType`, `ErrTooLarge`.
- **Validasi sebelum I/O**: cek MIME/ukuran sebelum panggil storage/DB (pola `UploadAttachment`).

## 7. Query (sqlc) yang ditambahkan ke `assets.sql`

- `CreateAssetDocument :one` — INSERT metadata (object_key NULL awalnya).
- `ListAssetDocuments :many` — `WHERE asset_id = $1 AND deleted_at IS NULL` urut `created_at DESC`.
- `GetAssetDocument :one` — `WHERE id = $1 AND deleted_at IS NULL`.
- `UpdateAssetDocument :one` — update metadata (doc_type/doc_no/doc_date/counterparty/related_request_id).
- `SetAssetDocumentObjectKey :one` — set `object_key` saat attach/replace file.
- `SoftDeleteAssetDocument :execrows` — `SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`.

## 8. Pengujian (proaktif & luas)

- **Unit** (`document_test.go`, `document_dto_test.go`): validasi DTO (doc_type oneof, parse
  tanggal valid/invalid, related_request opsional), `documentToMap` (serialisasi semua field +
  flag `has_file`), MIME/ukuran lewat fake store (metadata-only sukses tanpa I/O; tipe terlarang &
  oversize ditolak sebelum DB), download saat object_key NULL → ErrNotFound.
- **Integration** (`document_integration_test.go`, `//go:build integration`, MinIO+Postgres
  testcontainer):
  - Buat dokumen metadata-only (tanpa file) → tersimpan, `has_file=false`.
  - Lampirkan file → round-trip unduh byte-identik; `has_file=true`.
  - Ganti berkas → objek lama hilang dari MinIO, objek baru ada.
  - List & get; `:docId` aset lain → 404.
  - **Enforce scope**: caller di luar office aset → 403 pada read & write.
  - Delete → row soft-deleted + objek hilang (tak ada orphan).
  - `related_request_id` invalid → 400.
  - Oversize & tipe terlarang ditolak.
  - Rollback DB → tak ada orphan di MinIO.

## 9. Sinkronisasi & "selesai"

- `backend/api/openapi.yaml` — tambah paths + schema; lolos Spectral.
- `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./...` hijau.
- `docs/PROGRESS.md` — centang **"Asset documents (BAST)"** + refresh blok "Next session".
