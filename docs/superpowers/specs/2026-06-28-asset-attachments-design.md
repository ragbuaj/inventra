# Asset Attachments (MinIO) — Backend Design

Date: 2026-06-28
Status: Approved (decisions confirmed with user)

## Goal

Let users attach files (photos + documents) to assets, stored in MinIO (S3-compatible), with all
access **mediated and authorized by the backend** (PRD FR-2.6). The `asset.asset_attachments` table,
the `shared.attachment_kind` enum, and the MinIO config already exist (migration `000008`, config
`internal/config`); MinIO runs in docker-compose. This is a greenfield Go build: add a storage client,
attachment endpoints on the existing asset module, validation, and thumbnail generation for images.

## Scope

**In scope:**
- `internal/storage/` package: a `Storage` interface + a MinIO implementation (`minio-go/v7`).
- Attachment CRUD on the asset module: upload (multipart, proxied), list, download (proxied stream),
  thumbnail (proxied stream), delete.
- Validation: MIME whitelist + max-size (env-config). `kind` derived from MIME.
- Thumbnail generation for images (original stored intact); PDFs/documents stored as-is.
- sqlc queries for attachments, OpenAPI sync, unit + integration tests, wiring.

**Out of scope (do NOT build here):**
- BAST/asset_documents (separate module; this feature enables it later).
- Original-image compression/re-encode (deliberately NOT done — originals preserved per user choice).
- Presigned URLs (rejected in favor of backend-proxied access for authz consistency).
- Async/background thumbnailing (done synchronously on upload — no job queue exists yet).
- Barcode/QR, transfer, disposal accounting, etc.

## Decisions (confirmed)

1. **Backend-proxied access** (not presigned URLs): upload is multipart → backend validates → streams to
   MinIO; download/thumbnail are streamed back through the backend after a per-request scope check on
   the owning asset. Every byte served is authorized — consistent with the bank data-scope model and
   the IDOR hardening already applied to the asset module.
2. **Thumbnail only; original preserved.** For images, generate a JPEG thumbnail for preview but store
   the original unmodified (asset photos are evidence; detail matters). Deliberate, user-approved
   deviation from PRD's literal "kompresi". PDFs/documents have no thumbnail.
3. **`kind` derived from MIME**: `image/*` → `photo`, otherwise → `document`. The client does not send
   `kind`.
4. **MIME whitelist + size limit**: accept `image/jpeg`, `image/png`, `image/webp`, `application/pdf`;
   reject others (415); max size from env `ATTACHMENT_MAX_BYTES` (default 5 MB), over → 413.
5. **Integration tests use a real MinIO testcontainer** for the storage round-trip (in addition to
   fake-Storage unit tests for service logic).

## Architecture

### 1. `internal/storage/` package

```go
type ObjectInfo struct {
    ContentType   string
    Size          int64
}

type Storage interface {
    EnsureBucket(ctx context.Context) error
    Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
    Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
    Remove(ctx context.Context, key string) error
}
```

- `minio.go`: `type MinIOStorage struct { client *minio.Client; bucket string }` implementing `Storage`
  via `github.com/minio/minio-go/v7`. `NewMinIOStorage(cfg)` builds the client from
  `MinIOEndpoint/AccessKey/SecretKey/Bucket/UseSSL`. `EnsureBucket` calls `BucketExists`→`MakeBucket`.
- The interface lives in `storage.go` so the asset service depends on the interface, not the MinIO
  client — unit tests inject an in-memory fake (`map[string][]byte`).

### 2. Config + wiring

- `internal/config`: add `AttachmentMaxBytes int64` (env `ATTACHMENT_MAX_BYTES`, default `5*1024*1024`).
  Document in `.env.example`. (MinIO fields already exist.)
- `cmd/api/main.go`: construct `storage.NewMinIOStorage(cfg)`, call `EnsureBucket` at startup (fail fast
  on error), pass into `server.Deps`.
- `internal/server/router.go`: add `Storage storage.Storage` to `Deps`; `asset.NewService` gains
  `storage storage.Storage` + `maxBytes int64` parameters; update the wiring block.

### 3. Asset module — attachment layer

New file `internal/asset/attachment.go` (service methods) keeps `service.go` focused; handler methods
in `attachment_handler.go` (or appended to `handler.go` if small); routes extended in `routes.go`.

**Service methods** (Gin-free; return domain structs + sentinel errors):
- `UploadAttachment(ctx, in UploadInput) (sqlc.AssetAssetAttachment, error)` — `in` carries
  `AssetID, OfficeID(for record), Filename, ContentType, Size, Data []byte (or io.Reader), CreatedBy`.
  Validates MIME + size (`ErrUnsupportedType`/`ErrTooLarge`), derives `kind`, generates object key,
  `storage.Put` the original; if image, decode→resize→encode JPEG thumbnail and `Put` the thumbnail
  key; insert the metadata row (`CreateAttachment`). If the DB insert fails after Put, best-effort
  `Remove` the just-written object(s).
- `ListAttachments(ctx, assetID) ([]sqlc.AssetAssetAttachment, error)`.
- `GetAttachment(ctx, attachmentID) (sqlc.AssetAssetAttachment, error)` (for scope check + key lookup).
- `OpenAttachment(ctx, att, thumb bool) (io.ReadCloser, ObjectInfo, error)` — `storage.Get` the
  original or thumbnail key (thumb on a non-image → `ErrNotFound`).
- `DeleteAttachment(ctx, attachmentID) (sqlc.AssetAssetAttachment, error)` — soft-delete the row, then
  best-effort `storage.Remove` of object + thumbnail.

**Object key**: `assets/<asset_id>/<uuid>.<ext>`; thumbnail `assets/<asset_id>/<uuid>_thumb.jpg`.
`<ext>` from the MIME (jpg/png/webp/pdf).

**Thumbnail**: `github.com/disintegration/imaging` (decode jpeg/png; `golang.org/x/image/webp` registers
webp decode) → `imaging.Fit(img, 300, 300, ...)` → encode JPEG (quality ~80). On decode failure of a
declared-image, return `ErrUnsupportedType` (corrupt/mislabeled) rather than storing a thumbnailless
photo.

**Handlers** (each: resolve asset, scope-check `InScope(all, ids, asset.OfficeID)` for module "assets",
else 403):
- `uploadAttachment` — parse multipart (`c.Request.FormFile("file")`); enforce `MaxBytes` via
  `http.MaxBytesReader` / `c.Request.ContentLength` guard; detect content-type from the header and/or
  `http.DetectContentType` and require it to match the whitelist; call service; audit.Record(create);
  return 201 metadata.
- `listAttachments` — 200 `{data:[Attachment], total}`.
- `downloadAttachment` / `downloadThumbnail` — scope-check, `OpenAttachment`, set
  `Content-Type` + `Content-Disposition` (inline; filename for original), `io.Copy` to the response.
- `deleteAttachment` — scope-check, service delete, audit.Record(delete), 204.

**Routes** (`/assets/:id/attachments`, after authMW):
```
POST   ""                      requireManage   uploadAttachment
GET    ""                      requireView     listAttachments
GET    "/:aid/content"         requireView     downloadAttachment
GET    "/:aid/thumbnail"       requireView     downloadThumbnail
DELETE "/:aid"                 requireManage   deleteAttachment
```
(`:id` is the asset id; `:aid` the attachment id. The handler verifies the attachment belongs to the
asset.)

### 4. Queries (`db/queries/assets.sql` → sqlc generate)

- `CreateAttachment` (asset_id, kind, object_key, thumbnail_key, original_filename, size_bytes,
  mime_type, created_by_id) RETURNING *.
- `ListAttachments` — by asset_id, `deleted_at IS NULL`, order by created_at.
- `GetAttachment` — by id, `deleted_at IS NULL`.
- `SoftDeleteAttachment` — set deleted_at; `:execrows` (0 → ErrNotFound).

### 5. DTO

`attachmentToMap`/response: `{id, asset_id, kind, original_filename, size_bytes, mime_type,
has_thumbnail (thumbnail_key != nil), created_at}`. Never expose raw `object_key`/`thumbnail_key`
(internal storage paths) in API responses — access is only via the proxied content/thumbnail routes.

## Authorization

- Upload/delete: `asset.manage` + office scope of the asset. List/download/thumbnail: `asset.view` +
  office scope. Scope resolved via `common.ScopedDeps.CallerOfficeScope(c, "assets")` and
  `common.InScope(all, ids, asset.OfficeID)` (same pattern as the asset read/update handlers). The
  attachment's asset is the authorization anchor — fetch it first, scope-check, then act.
- No field-permission masking on attachment metadata (not sensitive cost data).

## Error handling

- `ErrUnsupportedType` → 415; `ErrTooLarge` → 413; asset/attachment not found → 404; out-of-scope →
  403; mismatched asset↔attachment → 404. DB errors via the existing `mapDBError`. Storage `Get`
  not-found → 404. A failed thumbnail decode of a declared image → 415 (reject the upload; do not store
  a partial record).
- Upload atomicity: write object(s) first, then the DB row; on DB failure best-effort remove the
  object(s) so storage doesn't accumulate orphans. (A rare orphan on a crash between Put and insert is
  acceptable; a later GC sweep can reconcile — out of scope.)

## Testing

**Unit** (fake in-memory `Storage`, no container):
- MIME whitelist (each allowed type accepted; a disallowed type → ErrUnsupportedType).
- Size limit boundary (== max ok; > max → ErrTooLarge).
- `kind` derivation (image/* → photo; pdf → document).
- Object-key format + extension mapping.
- Thumbnail: a valid small PNG/JPEG → thumbnail object written + `thumbnail_key` set; a PDF → no
  thumbnail, `thumbnail_key` nil; a corrupt "image/png" → ErrUnsupportedType.
- Upload rollback: fake Storage Put ok but injected DB-insert failure → object removed (assert the fake
  no longer holds the key).
- `attachmentToMap` omits object_key/thumbnail_key; `has_thumbnail` reflects presence.

**Integration** (`//go:build integration`, real Postgres + **MinIO testcontainer** via a new
`testsupport.NewMinIO(t)` helper):
- Round-trip: upload an image via the HTTP handler (httptest) → 201; object + thumbnail exist in MinIO;
  list returns it; `GET /content` streams the exact bytes back with the right Content-Type; `GET
  /thumbnail` streams a JPEG; `DELETE` → 204 and the objects are gone + row soft-deleted.
- PDF upload → no thumbnail; `GET /thumbnail` → 404.
- Scope enforcement: a caller out of scope for the asset's office → 403 on upload/list/download/delete.
- Oversize upload → 413; disallowed type → 415.

## Verification gates

`go build ./...` · `go vet ./...` · `go test ./...` · `go test -tags=integration ./internal/asset/`
(Docker/MinIO up) · Spectral lint of `backend/api/openapi.yaml` · update `docs/PROGRESS.md`.

## Open items (flagged, non-blocking)

- Storage GC for orphaned objects (crash between Put and DB insert) — deferred; rare and reconcilable.
- A future `asset_documents`/BAST module will reuse this `Storage` interface and the attachment
  patterns.
