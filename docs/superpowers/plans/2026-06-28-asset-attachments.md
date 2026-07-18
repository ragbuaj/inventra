# Asset Attachments (MinIO) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users attach photos/documents to assets, stored in MinIO and served only through backend-authorized proxy endpoints, with image thumbnails.

**Architecture:** A `internal/storage` package abstracts MinIO behind a `Storage` interface (real MinIO impl + in-memory fake for tests). The existing `internal/asset` module gains attachment service methods, multipart upload + proxied download/thumbnail handlers, and `/assets/:id/attachments` routes. Every attachment access is scope-checked against the owning asset's office (same authz as asset read/update).

**Tech Stack:** Go 1.25, Gin, pgx/v5, sqlc, MinIO (`github.com/minio/minio-go/v7`), image processing (`github.com/disintegration/imaging` + `golang.org/x/image/webp`), testify + testcontainers-go.

## Global Constraints

- Go module path `github.com/ragbuaj/inventra`; backend commands run from `backend/`.
- Backend-proxied access only — NO presigned URLs. Every download/thumbnail checks caller office scope on the owning asset before streaming.
- MIME whitelist: `image/jpeg`, `image/png`, `image/webp`, `application/pdf`. Disallowed → 415. Max size from env `ATTACHMENT_MAX_BYTES` (default `5*1024*1024`). Over → 413.
- `kind` derived from MIME: `image/*` → `photo`, else → `document`. Client never sends `kind`.
- Images: generate a JPEG thumbnail; store the ORIGINAL unmodified. PDFs/documents: no thumbnail.
- Object key: `assets/<asset_id>/<uuid>.<ext>`; thumbnail `assets/<asset_id>/<uuid>_thumb.jpg`.
- API responses NEVER expose `object_key`/`thumbnail_key`; expose `has_thumbnail` bool instead.
- Authz: upload/delete = `asset.manage` + office scope; list/download/thumbnail = `asset.view` + office scope. Scope via `common.ScopedDeps.CallerOfficeScope(c, "assets")` + `common.InScope(all, ids, asset.OfficeID)`.
- Soft-delete everywhere; reads filter `deleted_at IS NULL`. Never hand-edit `db/sqlc/`.
- Don't break existing asset tests. Default `go test ./...` stays unit-only; integration behind `//go:build integration`.
- Conventional Commits: `feat(storage):`, `feat(asset):`, `feat(db):`, `docs(api):`. No Claude/AI co-author trailers.
- Reference spec: `docs/superpowers/specs/2026-06-28-asset-attachments-design.md`.

---

## File Structure

- `backend/go.mod` / `go.sum` — add minio-go, disintegration/imaging, golang.org/x/image.
- `backend/internal/storage/storage.go` — `Storage` interface + `ObjectInfo` + `ErrObjectNotFound`.
- `backend/internal/storage/minio.go` — `MinIOStorage` (real impl) + `NewMinIOStorage`.
- `backend/internal/storage/fake.go` — `Fake` in-memory `Storage` for tests (in package, non-test file so other packages' tests can import it).
- `backend/internal/config/config.go` (modify) — `AttachmentMaxBytes`.
- `backend/.env.example` (modify) — `ATTACHMENT_MAX_BYTES`.
- `backend/db/queries/assets.sql` (modify) + `db/sqlc/*` (generated) — attachment queries.
- `backend/internal/asset/thumbnail.go` — `makeThumbnail([]byte) ([]byte, error)`.
- `backend/internal/asset/attachment.go` — attachment service methods + sentinels (`ErrUnsupportedType`, `ErrTooLarge`).
- `backend/internal/asset/attachment_handler.go` — attachment handlers.
- `backend/internal/asset/service.go` (modify) — Service gains `storage`, `maxBytes`; `NewService` signature.
- `backend/internal/asset/dto.go` (modify) — `attachmentToMap`.
- `backend/internal/asset/routes.go` (modify) — attachment routes.
- `backend/internal/server/router.go` (modify) + `cmd/api/main.go` (modify) — Deps.Storage + construction + EnsureBucket + NewService call.
- `backend/internal/testsupport/minio.go` — `NewMinIO(t)` container helper.
- `backend/internal/asset/attachment_*_test.go` (unit) + `attachment_integration_test.go` (`//go:build integration`).
- `backend/api/openapi.yaml` (modify); `docs/PROGRESS.md` (modify).

---

## Task 1: Storage package (interface + fake + MinIO impl) + deps

**Files:**
- Create: `backend/internal/storage/storage.go`, `backend/internal/storage/fake.go`, `backend/internal/storage/minio.go`
- Modify: `backend/go.mod`, `backend/go.sum`
- Test: `backend/internal/storage/fake_test.go`

**Interfaces:**
- Produces: `type Storage interface { EnsureBucket(ctx) error; Put(ctx, key string, r io.Reader, size int64, contentType string) error; Get(ctx, key string) (io.ReadCloser, ObjectInfo, error); Remove(ctx, key string) error }`; `type ObjectInfo struct { ContentType string; Size int64 }`; `var ErrObjectNotFound`; `func NewFake() *Fake`; `func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStorage, error)`.

- [ ] **Step 1: Add dependencies**

Run:
```bash
cd backend
go get github.com/minio/minio-go/v7@latest
go get github.com/disintegration/imaging@latest
go get golang.org/x/image@latest
```
Expected: go.mod/go.sum updated, no errors.

- [ ] **Step 2: Write `storage.go`**

```go
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrObjectNotFound is returned by Get/Remove when the key does not exist.
var ErrObjectNotFound = errors.New("object not found")

// ObjectInfo carries the minimal metadata needed to serve an object.
type ObjectInfo struct {
	ContentType string
	Size        int64
}

// Storage is an S3-compatible object store abstraction.
type Storage interface {
	EnsureBucket(ctx context.Context) error
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Remove(ctx context.Context, key string) error
}
```

- [ ] **Step 3: Write `fake.go` (in-memory, used by other packages' unit tests)**

```go
package storage

import (
	"bytes"
	"context"
	"io"
	"sync"
)

type fakeObj struct {
	data        []byte
	contentType string
}

// Fake is an in-memory Storage for tests.
type Fake struct {
	mu   sync.Mutex
	objs map[string]fakeObj
	// PutErr, when set, makes the next Put fail (to test rollback paths).
	PutErr error
}

func NewFake() *Fake { return &Fake{objs: map[string]fakeObj{}} }

func (f *Fake) EnsureBucket(context.Context) error { return nil }

func (f *Fake) Put(_ context.Context, key string, r io.Reader, _ int64, ct string) error {
	if f.PutErr != nil { return f.PutErr }
	b, err := io.ReadAll(r)
	if err != nil { return err }
	f.mu.Lock(); defer f.mu.Unlock()
	f.objs[key] = fakeObj{data: b, contentType: ct}
	return nil
}

func (f *Fake) Get(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	o, ok := f.objs[key]
	if !ok { return nil, ObjectInfo{}, ErrObjectNotFound }
	return io.NopCloser(bytes.NewReader(o.data)), ObjectInfo{ContentType: o.contentType, Size: int64(len(o.data))}, nil
}

func (f *Fake) Remove(_ context.Context, key string) error {
	f.mu.Lock(); defer f.mu.Unlock()
	delete(f.objs, key)
	return nil
}

// Has reports whether a key exists (test helper).
func (f *Fake) Has(key string) bool {
	f.mu.Lock(); defer f.mu.Unlock()
	_, ok := f.objs[key]; return ok
}
```

- [ ] **Step 4: Write the failing fake test**

```go
package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestFakeRoundTrip(t *testing.T) {
	f := NewFake(); ctx := context.Background()
	if err := f.Put(ctx, "k", bytes.NewReader([]byte("hi")), 2, "text/plain"); err != nil { t.Fatal(err) }
	rc, info, err := f.Get(ctx, "k")
	if err != nil { t.Fatal(err) }
	b, _ := io.ReadAll(rc); rc.Close()
	if string(b) != "hi" || info.ContentType != "text/plain" || info.Size != 2 { t.Fatalf("got %q %+v", b, info) }
	if !f.Has("k") { t.Fatal("Has should be true") }
	if err := f.Remove(ctx, "k"); err != nil { t.Fatal(err) }
	if _, _, err := f.Get(ctx, "k"); !errors.Is(err, ErrObjectNotFound) { t.Fatalf("want ErrObjectNotFound, got %v", err) }
}
```

- [ ] **Step 5: Run the test**

Run: `cd backend && go test ./internal/storage/ -run TestFakeRoundTrip -v`
Expected: PASS.

- [ ] **Step 6: Write `minio.go` (real impl)**

```go
package storage

import (
	"context"
	"errors"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	client *minio.Client
	bucket string
}

func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStorage, error) {
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil { return nil, err }
	return &MinIOStorage{client: c, bucket: bucket}, nil
}

func (s *MinIOStorage) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil { return err }
	if ok { return nil }
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

func (s *MinIOStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *MinIOStorage) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil { return nil, ObjectInfo{}, err }
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		var resp minio.ErrorResponse
		if errors.As(err, &resp) && resp.Code == "NoSuchKey" { return nil, ObjectInfo{}, ErrObjectNotFound }
		return nil, ObjectInfo{}, err
	}
	return obj, ObjectInfo{ContentType: info.ContentType, Size: info.Size}, nil
}

func (s *MinIOStorage) Remove(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
```

> Verify the exact minio-go v7 symbols after `go get` (`minio.New`, `minio.Options`, `credentials.NewStaticV4`, `GetObjectOptions`, `obj.Stat()`, `minio.ErrorResponse.Code`). Adjust the NoSuchKey detection if the installed version differs; keep the `ErrObjectNotFound` mapping behavior.

- [ ] **Step 7: Build + commit**

Run: `cd backend && go build ./... && go test ./internal/storage/`
```bash
git add backend/go.mod backend/go.sum backend/internal/storage/
git commit -m "feat(storage): MinIO storage abstraction + in-memory fake"
```

## Task 2: Config field + wiring (Deps.Storage, main, NewService signature)

**Files:**
- Modify: `backend/internal/config/config.go`, `backend/.env.example`, `backend/cmd/api/main.go`, `backend/internal/server/router.go`, `backend/internal/asset/service.go`

**Interfaces:**
- Consumes: `storage.Storage`, `storage.NewMinIOStorage` (Task 1).
- Produces: `Config.AttachmentMaxBytes int64`; `Deps.Storage storage.Storage`; new `asset.NewService(q *sqlc.Queries, pool *pgxpool.Pool, store storage.Storage, maxBytes int64) *Service` (Service gains `store storage.Storage`, `maxBytes int64` fields).

- [ ] **Step 1: Add config field**

In `internal/config/config.go`, add to the `Config` struct (near the MinIO block):
```go
	AttachmentMaxBytes int64
```
In `Load()`, add:
```go
		AttachmentMaxBytes: int64(getEnvInt("ATTACHMENT_MAX_BYTES", 5*1024*1024)),
```
> Confirm `getEnvInt` returns `int`; cast to int64 as shown. If a `getEnvInt64` exists, use it.
In `.env.example`, under the MinIO block, add:
```
ATTACHMENT_MAX_BYTES=5242880
```

- [ ] **Step 2: Update Service struct + NewService**

In `internal/asset/service.go`, change the struct + constructor:
```go
type Service struct {
	q        *sqlc.Queries
	pool     *pgxpool.Pool
	store    storage.Storage
	maxBytes int64
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, store storage.Storage, maxBytes int64) *Service {
	return &Service{q: q, pool: pool, store: store, maxBytes: maxBytes}
}
```
Add the import `"github.com/ragbuaj/inventra/internal/storage"`.

- [ ] **Step 3: Add Storage to Deps + construct in main + update router call**

In `internal/server/router.go`, add to `Deps`:
```go
	Storage storage.Storage
```
(import `"github.com/ragbuaj/inventra/internal/storage"`). Update the asset wiring line:
```go
assetSvc := asset.NewService(queries, d.Pool, d.Storage, d.Cfg.AttachmentMaxBytes)
```
In `cmd/api/main.go`, after the Redis client is built and before `server.NewRouter`, construct storage:
```go
store, err := storage.NewMinIOStorage(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
if err != nil {
	logger.Error("minio init failed", "err", err)
	os.Exit(1)
}
if err := store.EnsureBucket(context.Background()); err != nil {
	logger.Error("minio bucket ensure failed", "err", err)
	os.Exit(1)
}
```
Add `store` to the Deps literal: `server.NewRouter(server.Deps{Cfg: cfg, Pool: pool, Redis: rdb, Log: logger, Limiter: limiter, Storage: store})`. Add imports `"context"` and `"github.com/ragbuaj/inventra/internal/storage"` if missing (match the file's existing error-handling/logging style for the two failure exits).

- [ ] **Step 4: Build + vet**

Run: `cd backend && go build ./... && go vet ./...`
Expected: clean. (Service has new fields, unused for now — that's fine.)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config/config.go backend/.env.example backend/cmd/api/main.go backend/internal/server/router.go backend/internal/asset/service.go
git commit -m "feat(asset): wire MinIO storage + ATTACHMENT_MAX_BYTES into asset service"
```

## Task 3: Attachment queries

**Files:**
- Modify: `backend/db/queries/assets.sql`; generated `backend/db/sqlc/*`

**Interfaces:**
- Produces: `CreateAttachment`, `ListAttachments`, `GetAttachment`, `SoftDeleteAttachment` sqlc methods.

- [ ] **Step 1: Append queries to `db/queries/assets.sql`**

```sql
-- name: CreateAttachment :one
INSERT INTO asset.asset_attachments (
  asset_id, kind, object_key, thumbnail_key, original_filename, size_bytes, mime_type, created_by_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *;

-- name: ListAttachments :many
SELECT * FROM asset.asset_attachments
WHERE asset_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetAttachment :one
SELECT * FROM asset.asset_attachments WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteAttachment :execrows
UPDATE asset.asset_attachments SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
```

- [ ] **Step 2: Generate + build**

Run: `cd backend && sqlc generate && go build ./...`
Expected: `db/sqlc/assets.sql.go` defines the four methods; `CreateAttachmentParams` has `AssetID, Kind sqlc.SharedAttachmentKind, ObjectKey, ThumbnailKey *string, OriginalFilename, SizeBytes int64, MimeType, CreatedByID *uuid.UUID`. No errors.

- [ ] **Step 3: Commit**

```bash
git add backend/db/queries/assets.sql backend/db/sqlc/
git commit -m "feat(db): asset attachment queries"
```

## Task 4: Thumbnail helper

**Files:**
- Create: `backend/internal/asset/thumbnail.go`, `backend/internal/asset/thumbnail_test.go`

**Interfaces:**
- Produces: `func makeThumbnail(data []byte) ([]byte, error)` — decodes jpeg/png/webp, fits within 300x300, returns JPEG bytes. Returns an error on undecodable input.

- [ ] **Step 1: Write the failing test**

```go
package asset

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func makeTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ { for y := 0; y < h; y++ { img.Set(x, y, color.RGBA{uint8(x % 256), 0, 0, 255}) } }
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestMakeThumbnail_ResizesImage(t *testing.T) {
	src := makeTestPNG(800, 600)
	out, err := makeThumbnail(src)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil { t.Fatalf("thumbnail not decodable: %v", err) }
	if format != "jpeg" { t.Fatalf("want jpeg, got %s", format) }
	b := img.Bounds()
	if b.Dx() > 300 || b.Dy() > 300 { t.Fatalf("thumbnail too large: %dx%d", b.Dx(), b.Dy()) }
}

func TestMakeThumbnail_RejectsGarbage(t *testing.T) {
	if _, err := makeThumbnail([]byte("not an image")); err == nil {
		t.Fatal("expected error for non-image input")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run TestMakeThumbnail`
Expected: FAIL (`undefined: makeThumbnail`).

- [ ] **Step 3: Implement `thumbnail.go`**

```go
package asset

import (
	"bytes"
	"image"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register webp decoder
)

const thumbMaxDim = 300

// makeThumbnail decodes an image (jpeg/png/webp) and returns a JPEG thumbnail
// fitted within thumbMaxDim x thumbMaxDim. Returns an error for undecodable input.
func makeThumbnail(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	thumb := imaging.Fit(img, thumbMaxDim, thumbMaxDim, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, imaging.JPEG, imaging.JPEGQuality(80)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

> `image.Decode` needs registered decoders. `imaging` imports jpeg/png; the blank `golang.org/x/image/webp` import registers webp. Confirm `image.Decode` resolves jpeg/png via imaging's transitive registration; if not, add `_ "image/jpeg"` and `_ "image/png"` blank imports.

- [ ] **Step 4: Run to verify pass**

Run: `cd backend && go test ./internal/asset/ -run TestMakeThumbnail -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/asset/thumbnail.go backend/internal/asset/thumbnail_test.go
git commit -m "feat(asset): image thumbnail helper (jpeg/png/webp -> jpeg)"
```

## Task 5: Attachment service (validation, upload+rollback, list/get/open/delete)

**Files:**
- Create: `backend/internal/asset/attachment.go`, `backend/internal/asset/attachment_test.go`

**Interfaces:**
- Consumes: `Service` fields `store`/`maxBytes`/`q`/`pool` (Task 2), `makeThumbnail` (Task 4), `mapDBError` (existing), sqlc attachment queries (Task 3), `storage.ErrObjectNotFound` (Task 1).
- Produces: sentinels `ErrUnsupportedType`, `ErrTooLarge`; `type UploadInput struct { AssetID uuid.UUID; Filename, ContentType string; Data []byte; CreatedBy uuid.UUID }`; methods `UploadAttachment(ctx, in UploadInput) (sqlc.AssetAssetAttachment, error)`, `ListAttachments(ctx, assetID uuid.UUID) ([]sqlc.AssetAssetAttachment, error)`, `GetAttachment(ctx, id uuid.UUID) (sqlc.AssetAssetAttachment, error)`, `OpenAttachment(ctx, att sqlc.AssetAssetAttachment, thumb bool) (io.ReadCloser, storage.ObjectInfo, error)`, `DeleteAttachment(ctx, id uuid.UUID) (sqlc.AssetAssetAttachment, error)`; helpers `kindFor(mime) sqlc.SharedAttachmentKind`, `extFor(mime) string`, `allowedMIME(mime) bool`.

- [ ] **Step 1: Write the failing test (validation + kind/ext + upload rollback, using the fake Storage)**

```go
package asset

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

func TestAllowedMIMEAndKind(t *testing.T) {
	for _, m := range []string{"image/jpeg", "image/png", "image/webp", "application/pdf"} {
		if !allowedMIME(m) { t.Errorf("%s should be allowed", m) }
	}
	if allowedMIME("application/zip") { t.Error("zip should be rejected") }
	if kindFor("image/png") != sqlc.SharedAttachmentKindPhoto { t.Error("image -> photo") }
	if kindFor("application/pdf") != sqlc.SharedAttachmentKindDocument { t.Error("pdf -> document") }
	if extFor("image/jpeg") != "jpg" || extFor("application/pdf") != "pdf" { t.Error("ext mapping") }
}

func TestUploadAttachment_RejectsTypeAndSize(t *testing.T) {
	s := NewService(nil, nil, storage.NewFake(), 10)
	ctx := context.Background()
	_, err := s.UploadAttachment(ctx, UploadInput{AssetID: uuid.New(), ContentType: "application/zip", Data: []byte("x")})
	if !errors.Is(err, ErrUnsupportedType) { t.Fatalf("want ErrUnsupportedType, got %v", err) }
	_, err = s.UploadAttachment(ctx, UploadInput{AssetID: uuid.New(), ContentType: "application/pdf", Data: make([]byte, 11)})
	if !errors.Is(err, ErrTooLarge) { t.Fatalf("want ErrTooLarge, got %v", err) }
}
```

> Note: `UploadAttachment` must validate MIME and size BEFORE any DB/storage call so these two cases work with `q == nil`.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run 'TestAllowedMIME|TestUploadAttachment_Rejects'`
Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement `attachment.go`**

```go
package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

var (
	ErrUnsupportedType = errors.New("unsupported attachment type")
	ErrTooLarge        = errors.New("attachment exceeds size limit")
)

var mimeExt = map[string]string{
	"image/jpeg":      "jpg",
	"image/png":       "png",
	"image/webp":      "webp",
	"application/pdf": "pdf",
}

func allowedMIME(m string) bool { _, ok := mimeExt[m]; return ok }
func extFor(m string) string    { return mimeExt[m] }

func kindFor(m string) sqlc.SharedAttachmentKind {
	if strings.HasPrefix(m, "image/") {
		return sqlc.SharedAttachmentKindPhoto
	}
	return sqlc.SharedAttachmentKindDocument
}

type UploadInput struct {
	AssetID     uuid.UUID
	Filename    string
	ContentType string
	Data        []byte
	CreatedBy   uuid.UUID
}

func (s *Service) UploadAttachment(ctx context.Context, in UploadInput) (sqlc.AssetAssetAttachment, error) {
	var zero sqlc.AssetAssetAttachment
	if !allowedMIME(in.ContentType) {
		return zero, ErrUnsupportedType
	}
	if int64(len(in.Data)) > s.maxBytes {
		return zero, ErrTooLarge
	}
	kind := kindFor(in.ContentType)
	id := uuid.New()
	objectKey := fmt.Sprintf("assets/%s/%s.%s", in.AssetID, id, extFor(in.ContentType))

	// thumbnail for images (original stored intact)
	var thumbKey *string
	if kind == sqlc.SharedAttachmentKindPhoto {
		thumb, err := makeThumbnail(in.Data)
		if err != nil {
			return zero, ErrUnsupportedType // declared image but undecodable
		}
		tk := fmt.Sprintf("assets/%s/%s_thumb.jpg", in.AssetID, id)
		if err := s.store.Put(ctx, tk, bytesReader(thumb), int64(len(thumb)), "image/jpeg"); err != nil {
			return zero, err
		}
		thumbKey = &tk
	}

	if err := s.store.Put(ctx, objectKey, bytesReader(in.Data), int64(len(in.Data)), in.ContentType); err != nil {
		if thumbKey != nil { _ = s.store.Remove(ctx, *thumbKey) }
		return zero, err
	}

	createdBy := in.CreatedBy
	row, err := s.q.CreateAttachment(ctx, sqlc.CreateAttachmentParams{
		AssetID:          in.AssetID,
		Kind:             kind,
		ObjectKey:        objectKey,
		ThumbnailKey:     thumbKey,
		OriginalFilename: in.Filename,
		SizeBytes:        int64(len(in.Data)),
		MimeType:         in.ContentType,
		CreatedByID:      &createdBy,
	})
	if err != nil {
		// rollback objects so storage doesn't accumulate orphans
		_ = s.store.Remove(ctx, objectKey)
		if thumbKey != nil { _ = s.store.Remove(ctx, *thumbKey) }
		return zero, mapDBError(err)
	}
	return row, nil
}

func (s *Service) ListAttachments(ctx context.Context, assetID uuid.UUID) ([]sqlc.AssetAssetAttachment, error) {
	rows, err := s.q.ListAttachments(ctx, assetID)
	return rows, mapDBError(err)
}

func (s *Service) GetAttachment(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetAttachment, error) {
	row, err := s.q.GetAttachment(ctx, id)
	return row, mapDBError(err)
}

func (s *Service) OpenAttachment(ctx context.Context, att sqlc.AssetAssetAttachment, thumb bool) (io.ReadCloser, storage.ObjectInfo, error) {
	key := att.ObjectKey
	if thumb {
		if att.ThumbnailKey == nil {
			return nil, storage.ObjectInfo{}, ErrNotFound
		}
		key = *att.ThumbnailKey
	}
	rc, info, err := s.store.Get(ctx, key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return nil, storage.ObjectInfo{}, ErrNotFound
	}
	return rc, info, err
}

func (s *Service) DeleteAttachment(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetAttachment, error) {
	att, err := s.q.GetAttachment(ctx, id)
	if err != nil {
		return att, mapDBError(err)
	}
	n, err := s.q.SoftDeleteAttachment(ctx, id)
	if err != nil {
		return att, mapDBError(err)
	}
	if n == 0 {
		return att, ErrNotFound
	}
	// best-effort object removal
	_ = s.store.Remove(ctx, att.ObjectKey)
	if att.ThumbnailKey != nil { _ = s.store.Remove(ctx, *att.ThumbnailKey) }
	return att, nil
}
```

Add a tiny helper at the bottom of `attachment.go`:
```go
func bytesReader(b []byte) io.Reader { return strings.NewReader(string(b)) }
```
> `ErrNotFound` already exists in the asset package (service.go). Reuse it. If you prefer, replace `bytesReader` with `bytes.NewReader` and import `bytes` (cleaner — do that instead of the string round-trip).

- [ ] **Step 4: Run tests + build**

Run: `cd backend && go test ./internal/asset/ -run 'TestAllowedMIME|TestUploadAttachment_Rejects' -v && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 5: Add an upload-rollback unit test (fake Put failure)**

Append to `attachment_test.go`:
```go
func TestUploadAttachment_RollbackOnDBError(t *testing.T) {
	// q is nil → CreateAttachment will panic/err; instead simulate by making the fake Put succeed
	// then asserting object removal requires a DB. This case is covered by integration; here we
	// assert the storage-only path: a Put failure returns the error and writes nothing.
	f := storage.NewFake()
	f.PutErr = errors.New("boom")
	s := NewService(nil, nil, f, 1024)
	_, err := s.UploadAttachment(context.Background(), UploadInput{AssetID: uuid.New(), ContentType: "application/pdf", Data: []byte("pdf")})
	if err == nil || err.Error() != "boom" { t.Fatalf("want put error, got %v", err) }
	if len(f.objsKeys()) != 0 { t.Fatalf("no object should remain") }
}
```
Add a test helper to `fake.go`:
```go
func (f *Fake) objsKeys() []string {
	f.mu.Lock(); defer f.mu.Unlock()
	ks := make([]string, 0, len(f.objs))
	for k := range f.objs { ks = append(ks, k) }
	return ks
}
```
> The DB-insert rollback path (object removed after a CreateAttachment failure) genuinely needs a DB and is covered in the integration task; this unit test covers the storage-failure path only.

- [ ] **Step 6: Run + commit**

Run: `cd backend && go test ./internal/asset/ -run 'Attachment|AllowedMIME' && go build ./...`
```bash
git add backend/internal/asset/attachment.go backend/internal/asset/attachment_test.go backend/internal/storage/fake.go
git commit -m "feat(asset): attachment service (validate, upload, thumbnail, list/get/delete)"
```

## Task 6: Attachment DTO

**Files:**
- Modify: `backend/internal/asset/dto.go`
- Test: `backend/internal/asset/dto_test.go` (append)

**Interfaces:**
- Produces: `func attachmentToMap(a sqlc.AssetAssetAttachment) map[string]any`.

- [ ] **Step 1: Write the failing test**

```go
func TestAttachmentToMap_HidesKeysExposesHasThumbnail(t *testing.T) {
	tk := "assets/x/y_thumb.jpg"
	m := attachmentToMap(sqlc.AssetAssetAttachment{
		ID: uuid.New(), AssetID: uuid.New(), Kind: sqlc.SharedAttachmentKindPhoto,
		ObjectKey: "assets/x/y.jpg", ThumbnailKey: &tk, OriginalFilename: "photo.jpg",
		SizeBytes: 123, MimeType: "image/jpeg",
	})
	if _, ok := m["object_key"]; ok { t.Error("object_key must not be exposed") }
	if _, ok := m["thumbnail_key"]; ok { t.Error("thumbnail_key must not be exposed") }
	if m["has_thumbnail"] != true { t.Error("has_thumbnail should be true") }
	if m["original_filename"] != "photo.jpg" || m["mime_type"] != "image/jpeg" { t.Error("metadata missing") }
	noThumb := attachmentToMap(sqlc.AssetAssetAttachment{ID: uuid.New(), MimeType: "application/pdf", Kind: sqlc.SharedAttachmentKindDocument})
	if noThumb["has_thumbnail"] != false { t.Error("has_thumbnail should be false for nil thumbnail_key") }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run TestAttachmentToMap`
Expected: FAIL.

- [ ] **Step 3: Implement `attachmentToMap` in `dto.go`**

```go
func attachmentToMap(a sqlc.AssetAssetAttachment) map[string]any {
	return map[string]any{
		"id":                a.ID.String(),
		"asset_id":          a.AssetID.String(),
		"kind":              string(a.Kind),
		"original_filename": a.OriginalFilename,
		"size_bytes":        a.SizeBytes,
		"mime_type":         a.MimeType,
		"has_thumbnail":     a.ThumbnailKey != nil,
		"created_at":        common.TsStr(a.CreatedAt),
	}
}
```
> `common` is already imported in dto.go. Confirm `common.TsStr` signature matches the existing usage.

- [ ] **Step 4: Run + commit**

Run: `cd backend && go test ./internal/asset/ -run TestAttachmentToMap -v`
```bash
git add backend/internal/asset/dto.go backend/internal/asset/dto_test.go
git commit -m "feat(asset): attachment DTO (hides storage keys, exposes has_thumbnail)"
```

## Task 7: Attachment handlers + routes

**Files:**
- Create: `backend/internal/asset/attachment_handler.go`
- Modify: `backend/internal/asset/routes.go`

**Interfaces:**
- Consumes: `Handler` struct (svc, fieldSvc, scoped, aud — existing), service methods (Task 5), `attachmentToMap` (Task 6), `common.InScope`/`WriteError`/`ErrForbidden`, `audit.Record`/`Diff`, `middleware.CtxUserID`.
- Produces: handler methods `uploadAttachment`, `listAttachments`, `downloadAttachment`, `downloadThumbnail`, `deleteAttachment`; updated `RegisterRoutes` (same signature) mounting `/assets/:id/attachments`.

- [ ] **Step 1: Implement `attachment_handler.go`**

```go
package asset

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// resolveAssetInScope loads the asset for :id and enforces the caller's "assets" office scope.
// Returns the asset and true if access is allowed; otherwise writes the error response and returns false.
func (h *Handler) resolveAssetInScope(c *gin.Context) (assetID uuid.UUID, officeID uuid.UUID, ok bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return }
	a, err := h.svc.Get(c.Request.Context(), id)
	if err != nil { h.handleErr(c, err); return }
	all, ids, err := h.scoped.CallerOfficeScope(c, "assets")
	if err != nil { common.WriteError(c, err); return }
	if !common.InScope(all, ids, a.OfficeID) { common.WriteError(c, common.ErrForbidden); return }
	return a.ID, a.OfficeID, true
}

func (h *Handler) handleErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnsupportedType):
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
	case errors.Is(err, ErrTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		common.WriteError(c, err)
	}
}

func (h *Handler) uploadAttachment(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok { return }
	// cap the request body to maxBytes+1 to detect oversize without buffering unbounded data
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.maxBytes+1)
	fileHeader, err := c.FormFile("file")
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field"}); return }
	f, err := fileHeader.Open()
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"}); return }
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"}); return
	}
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" { contentType = http.DetectContentType(data) }
	uid, _ := uuid.Parse(c.GetString(middleware.CtxUserID))
	row, err := h.svc.UploadAttachment(c.Request.Context(), UploadInput{
		AssetID: assetID, Filename: fileHeader.Filename, ContentType: contentType, Data: data, CreatedBy: uid,
	})
	if err != nil { h.handleErr(c, err); return }
	oid := officeID
	audit.Record(c, h.aud, audit.ActionCreate, "asset_attachments", row.ID, &oid, audit.Diff(nil, attachmentToMap(row)))
	c.JSON(http.StatusCreated, attachmentToMap(row))
}

func (h *Handler) listAttachments(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok { return }
	rows, err := h.svc.ListAttachments(c.Request.Context(), assetID)
	if err != nil { h.handleErr(c, err); return }
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows { data = append(data, attachmentToMap(r)) }
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

func (h *Handler) streamAttachment(c *gin.Context, thumb bool) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok { return }
	aid, err := uuid.Parse(c.Param("aid"))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid attachment id"}); return }
	att, err := h.svc.GetAttachment(c.Request.Context(), aid)
	if err != nil { h.handleErr(c, err); return }
	if att.AssetID != assetID { c.JSON(http.StatusNotFound, gin.H{"error": "not found"}); return }
	rc, info, err := h.svc.OpenAttachment(c.Request.Context(), att, thumb)
	if err != nil { h.handleErr(c, err); return }
	defer rc.Close()
	ct := info.ContentType
	if thumb { ct = "image/jpeg" } else if att.MimeType != "" { ct = att.MimeType }
	c.Header("Content-Disposition", "inline; filename=\""+att.OriginalFilename+"\"")
	c.DataFromReader(http.StatusOK, info.Size, ct, rc, nil)
}

func (h *Handler) downloadAttachment(c *gin.Context) { h.streamAttachment(c, false) }
func (h *Handler) downloadThumbnail(c *gin.Context)  { h.streamAttachment(c, true) }

func (h *Handler) deleteAttachment(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok { return }
	aid, err := uuid.Parse(c.Param("aid"))
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid attachment id"}); return }
	att, err := h.svc.GetAttachment(c.Request.Context(), aid)
	if err != nil { h.handleErr(c, err); return }
	if att.AssetID != assetID { c.JSON(http.StatusNotFound, gin.H{"error": "not found"}); return }
	if _, err := h.svc.DeleteAttachment(c.Request.Context(), aid); err != nil { h.handleErr(c, err); return }
	oid := officeID
	audit.Record(c, h.aud, audit.ActionDelete, "asset_attachments", aid, &oid, audit.Diff(attachmentToMap(att), nil))
	c.Status(http.StatusNoContent)
}
```

> Reconcile every cross-package symbol against the existing asset handler: `h.svc.Get`, `h.scoped.CallerOfficeScope`, `common.InScope/WriteError/ErrForbidden`, `audit.Record(c, svc, action, entityType, entityID, *officeID, changes)`, `audit.Diff`, `audit.ActionCreate/ActionDelete`, `middleware.CtxUserID`, and `c.DataFromReader`. `h.svc.maxBytes` is a same-package field access (allowed). If `audit.ActionDelete` doesn't exist, use the constant the audit package actually defines for deletes.

- [ ] **Step 2: Extend `routes.go`**

```go
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW, requireView, requireManage gin.HandlerFunc) {
	g := rg.Group("/assets")
	g.GET("", authMW, requireView, h.list)
	g.GET("/:id", authMW, requireView, h.get)
	g.PUT("/:id", authMW, requireManage, h.update)

	a := g.Group("/:id/attachments")
	a.POST("", authMW, requireManage, h.uploadAttachment)
	a.GET("", authMW, requireView, h.listAttachments)
	a.GET("/:aid/content", authMW, requireView, h.downloadAttachment)
	a.GET("/:aid/thumbnail", authMW, requireView, h.downloadThumbnail)
	a.DELETE("/:aid", authMW, requireManage, h.deleteAttachment)
}
```

> Gin route conflict note: `/assets/:id` and `/assets/:id/attachments/...` share the `:id` param — that's fine. But Gin may reject mixing `:id` wildcard with other path shapes at the same level only if there's a conflicting static segment; this nesting is standard and works. If Gin panics about wildcard conflicts at startup, restructure to a single `g.Group("/:id")` shared by get/update/attachments. Verify the server starts (Task 10 integration boots the router).

- [ ] **Step 3: Build + vet + existing tests**

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/asset/`
Expected: clean (no DB needed for unit tests; handlers compile).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/asset/attachment_handler.go backend/internal/asset/routes.go
git commit -m "feat(asset): attachment endpoints (upload/list/download/thumbnail/delete)"
```

## Task 8: OpenAPI sync

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add paths + schema**

Add under the existing `/api/v1` paths (mirror the existing asset path style, security `bearerJWT`, reuse `Forbidden`/`Unauthorized`/`BadRequest` response components):
- `POST /assets/{id}/attachments` — `requestBody` multipart/form-data with a `file` binary property; responses 201 `Attachment`, 400, 403, 413, 415.
- `GET /assets/{id}/attachments` — 200 `{data:[Attachment], total}`, 403.
- `GET /assets/{id}/attachments/{aid}/content` — 200 binary (`application/octet-stream` + image/pdf), 403, 404.
- `GET /assets/{id}/attachments/{aid}/thumbnail` — 200 `image/jpeg`, 404.
- `DELETE /assets/{id}/attachments/{aid}` — 204, 403, 404.

Add schema `Attachment`: `{id, asset_id, kind (enum photo/document), original_filename, size_bytes (integer), mime_type, has_thumbnail (boolean), created_at}`. Do NOT include object_key/thumbnail_key.

- [ ] **Step 2: Lint**

Run: `cd /d/portfolio-project/asset-management && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(api): OpenAPI for asset attachment endpoints"
```

## Task 9: testsupport MinIO helper

**Files:**
- Create: `backend/internal/testsupport/minio.go`

**Interfaces:**
- Produces: `func NewMinIO(t *testing.T) (*storage.MinIOStorage, string)` — starts a MinIO testcontainer, returns a ready `MinIOStorage` (bucket ensured) + the endpoint. Cleaned up via `t.Cleanup`.

- [ ] **Step 1: Implement `minio.go`**

```go
package testsupport

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ragbuaj/inventra/internal/storage"
)

// NewMinIO starts a MinIO container and returns a ready MinIOStorage (bucket created).
func NewMinIO(t *testing.T) *storage.MinIOStorage {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env:          map[string]string{"MINIO_ROOT_USER": "minioadmin", "MINIO_ROOT_PASSWORD": "minioadmin123"},
		Cmd:          []string{"server", "/data"},
		WaitingFor:   wait.ForListeningPort("9000/tcp"),
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx); require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "9000"); require.NoError(t, err)
	endpoint := host + ":" + port.Port()

	store, err := storage.NewMinIOStorage(endpoint, "minioadmin", "minioadmin123", "inventra-test", false)
	require.NoError(t, err)
	require.NoError(t, store.EnsureBucket(ctx))
	return store
}
```

> Mirror the existing `testsupport/postgres.go` container style (same testcontainers-go version, wait strategy import path). If `wait.ForListeningPort` needs a readiness HTTP probe instead, use `wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp")`.

- [ ] **Step 2: Build (test build tag)**

Run: `cd backend && go build ./internal/testsupport/ && go vet ./internal/testsupport/`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/testsupport/minio.go
git commit -m "test: testsupport MinIO container helper"
```

## Task 10: Integration tests

**Files:**
- Create: `backend/internal/asset/attachment_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes: `testsupport.NewPostgres`, `testsupport.NewMinIO`, the asset Service/Handler, `RegisterRoutes`, httptest.

- [ ] **Step 1: Write the integration suite (real Postgres + MinIO, via httptest router)**

Cover, with REAL assertions (no hollow checks):
- Seed an office + category + an asset (via direct SQL or the seed helpers). Build a gin router: `asset.NewService(queries, pool, minioStore, 5MB)` → `asset.NewHandler(...)` → `asset.RegisterRoutes` with a stub auth middleware that sets `CtxUserID`/`CtxRoleID` and a permission middleware that allows, plus a `ScopedDeps` resolving the seeded role's scope.
- **Image upload round-trip**: POST multipart PNG → 201; response has `has_thumbnail=true`, no `object_key`. `GET /attachments` lists it. `GET /:aid/content` returns the exact uploaded bytes with the image Content-Type. `GET /:aid/thumbnail` returns a decodable JPEG. `DELETE /:aid` → 204; afterwards `GET /:aid/content` → 404 and the row is soft-deleted (list empty).
- **PDF upload**: → 201 `has_thumbnail=false`; `GET /:aid/thumbnail` → 404.
- **Oversize**: a service with `maxBytes` small → POST larger file → 413.
- **Disallowed type**: POST `application/zip` (or a .zip filename with that content-type) → 415.
- **Scope enforcement**: a caller whose scope excludes the asset's office → 403 on upload/list/download/delete.
- **DB-insert rollback** (real DB): force a DB failure for CreateAttachment (e.g. upload targeting a non-existent asset_id that violates the FK) → expect error AND assert the object was removed from MinIO (use the store directly to confirm the key is absent).

Use `httptest.NewRecorder()` + `http.NewRequest` with a `multipart.Writer` body for uploads.

- [ ] **Step 2: Run the integration suite**

Run: `cd backend && go test -tags=integration ./internal/asset/ -run Attachment -v`
Expected: PASS (Docker up). Also confirm `go build -tags=integration ./...` and the non-tag `go test ./internal/asset/` still pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/asset/attachment_integration_test.go
git commit -m "test: integration coverage for asset attachments (MinIO round-trip + scope)"
```

## Task 11: PROGRESS.md + final verification gate

**Files:**
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Run the full gate**

Run:
```bash
cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./internal/asset/
cd /d/portfolio-project/asset-management && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
```
Expected: all green. If anything fails, STOP and report — do not edit PROGRESS to claim success.

- [ ] **Step 2: Update `docs/PROGRESS.md`**

Tick **Asset attachments (MinIO)** under "Backend — Feature modules" with a one-line note (Storage interface + MinIO impl; proxied upload/list/download/thumbnail/delete; MIME whitelist + size limit; image thumbnails, original preserved; scope-gated; integration tests). Note that BAST/asset_documents can now reuse this. Refresh the "▶ Next session — start here" block to point at the next real step (e.g. Barcode/QR, or wiring frontend Asset screens after ADR-0007). Note the PR number when merged.

- [ ] **Step 3: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): asset attachments (MinIO) landed"
```

---

## Self-Review notes (spec coverage)

- Spec bagian 1 storage package → Task 1. bagian 2 config+wiring → Task 2. bagian 3 service/handlers/routes → Tasks 5/6/7. bagian 4 queries → Task 3. bagian 5 DTO → Task 6. Thumbnail → Task 4. Authz scope-gating → Task 7 (`resolveAssetInScope`). Error handling (415/413/404/403) → Task 7 `handleErr`. Testing (unit fake + integration MinIO) → Tasks 1/4/5/6 (unit) + 9/10 (integration). OpenAPI → Task 8. Gates+PROGRESS → Task 11.
- `kind` derivation, MIME whitelist, key format, has_thumbnail, no-key-exposure all have concrete tasks/tests.
- Upload rollback: storage-failure path unit-tested (Task 5); DB-insert-failure rollback integration-tested (Task 10).
- Type consistency: `UploadInput`, `Storage`, `ObjectInfo`, `attachmentToMap`, `makeThumbnail`, `NewService(q,pool,store,maxBytes)` are defined once and referenced consistently across tasks.
