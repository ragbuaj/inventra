# Asset Documents (BAST) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-asset document (BAST) management to the `asset` module — metadata CRUD plus an optional MinIO-backed file, scope-gated and audited.

**Architecture:** Documents are a sub-feature of the existing `internal/asset` package (like attachments & barcode), reusing `*asset.Service`, the `storage.Storage` interface, `resolveAssetInScope`, `contentDisposition`, and `handleErr`. The schema (`asset.asset_documents`) already exists (migration `000015`). Metadata is created via JSON; the file is attached/replaced via a separate multipart sub-resource and is optional.

**Tech Stack:** Go 1.25, Gin, pgx/v5, sqlc, MinIO (via `internal/storage`), testify + testcontainers (integration).

## Global Constraints

- Scope: standalone per-asset documents only. Accept `related_request_id` (optional); **do NOT** accept `related_transfer_id`/`related_disposal_id` (modules not built yet).
- Permissions: reuse `asset.view` (reads) / `asset.manage` (writes) — already passed into `asset.RegisterRoutes`.
- File whitelist & size: reuse attachment's `allowedMIME`/`extFor` (`image/jpeg`,`image/png`,`image/webp`,`application/pdf`) and `s.maxBytes` (`ATTACHMENT_MAX_BYTES`). No thumbnails.
- Object key format: `assets/{assetID}/documents/{docID}.{ext}`.
- Every read & write enforces caller office scope on the parent asset (`resolveAssetInScope`).
- Audit entity string: `"asset_documents"`. Audit create/update/delete/file-upload with office = asset's office.
- Never serialize `object_key` in responses (storage-internal); expose `has_file` (bool) instead.
- Do not hand-edit `backend/db/sqlc/` — change `db/queries/assets.sql` then `sqlc generate`.
- Verify gates: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./...`, Spectral lint — all green.

---

### Task 1: sqlc queries for `asset_documents`

**Files:**
- Modify: `backend/db/queries/assets.sql` (append after the existing attachment queries, before the `GetAssetByTag` block or at end)
- Generated (do not hand-edit): `backend/db/sqlc/assets.sql.go`, `backend/db/sqlc/querier.go`

**Interfaces:**
- Produces (generated): `sqlc.CreateAssetDocumentParams{AssetID uuid.UUID, DocType SharedAssetDocumentType, DocNo *string, DocDate pgtype.Date, Counterparty *string, RelatedRequestID *uuid.UUID, CreatedByID *uuid.UUID}`; `Queries.CreateAssetDocument`, `Queries.ListAssetDocuments(assetID uuid.UUID)`, `Queries.GetAssetDocument(id uuid.UUID)`, `sqlc.UpdateAssetDocumentParams{ID, DocType, DocNo, DocDate, Counterparty, RelatedRequestID}`, `Queries.UpdateAssetDocument`, `sqlc.SetAssetDocumentObjectKeyParams{ID uuid.UUID, ObjectKey *string}`, `Queries.SetAssetDocumentObjectKey`, `Queries.SoftDeleteAssetDocument(id uuid.UUID) (int64, error)`.

- [ ] **Step 1: Append the queries**

Append to `backend/db/queries/assets.sql`:

```sql
-- name: CreateAssetDocument :one
INSERT INTO asset.asset_documents (
  asset_id, doc_type, doc_no, doc_date, counterparty, related_request_id, created_by_id
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAssetDocuments :many
SELECT * FROM asset.asset_documents
WHERE asset_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetAssetDocument :one
SELECT * FROM asset.asset_documents WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateAssetDocument :one
UPDATE asset.asset_documents
SET doc_type = $2, doc_no = $3, doc_date = $4, counterparty = $5, related_request_id = $6
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetAssetDocumentObjectKey :one
UPDATE asset.asset_documents
SET object_key = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteAssetDocument :execrows
UPDATE asset.asset_documents SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
```

- [ ] **Step 2: Regenerate sqlc**

Run (from `backend/`): `sqlc generate`
Expected: exit 0, no errors; `git status` shows changes in `backend/db/sqlc/`.

- [ ] **Step 3: Verify it compiles**

Run (from `backend/`): `go build ./...`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add backend/db/queries/assets.sql backend/db/sqlc/
git commit -m "feat(asset): sqlc queries for asset_documents"
```

---

### Task 2: Document DTOs + serialization (unit-tested)

**Files:**
- Create: `backend/internal/asset/document_dto.go`
- Test: `backend/internal/asset/document_dto_test.go`

**Interfaces:**
- Consumes: `sqlc.AssetAssetDocument`, `sqlc.SharedAssetDocumentType`; existing `parseDate` and `dateStr` from `dto.go`; `common.ParseUUIDPtr`, `common.UUIDPtrStr`, `common.TsStr`.
- Produces: `DocumentCreateRequest`, `DocumentUpdateRequest` (binding structs); `(DocumentCreateRequest).toInput(assetID, createdBy uuid.UUID) (DocumentInput, error)`; `(DocumentUpdateRequest).toUpdateInput() (DocumentUpdateInput, error)`; `documentToMap(d sqlc.AssetAssetDocument) map[string]any`. (Types `DocumentInput`/`DocumentUpdateInput` are defined in Task 3's `document.go`; this task references them by name.)

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/asset/document_dto_test.go`:

```go
package asset

import (
	"testing"

	"github.com/google/uuid"
)

func TestDocumentCreateRequest_toInput_OK(t *testing.T) {
	no := "BAST-2026-001"
	cp := "PT Vendor"
	date := "2026-06-28"
	req := uuid.New().String()
	r := DocumentCreateRequest{
		DocType:          "bast_acquisition",
		DocNo:            &no,
		DocDate:          &date,
		Counterparty:     &cp,
		RelatedRequestID: &req,
	}
	assetID, createdBy := uuid.New(), uuid.New()
	in, err := r.toInput(assetID, createdBy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.AssetID != assetID || in.CreatedBy != createdBy {
		t.Fatal("asset/createdBy not propagated")
	}
	if string(in.DocType) != "bast_acquisition" {
		t.Fatalf("doc_type = %s", in.DocType)
	}
	if !in.DocDate.Valid || in.DocDate.Time.Format("2006-01-02") != "2026-06-28" {
		t.Fatal("doc_date not parsed")
	}
	if in.RelatedRequestID == nil {
		t.Fatal("related_request_id not parsed")
	}
}

func TestDocumentCreateRequest_toInput_BadDate(t *testing.T) {
	bad := "28-06-2026"
	r := DocumentCreateRequest{DocType: "invoice", DocDate: &bad}
	if _, err := r.toInput(uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error for bad date")
	}
}

func TestDocumentCreateRequest_toInput_NilOptionals(t *testing.T) {
	r := DocumentCreateRequest{DocType: "other"}
	in, err := r.toInput(uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.DocNo != nil || in.Counterparty != nil || in.RelatedRequestID != nil || in.DocDate.Valid {
		t.Fatal("optionals should be nil/invalid")
	}
}

func TestDocumentToMap_HidesObjectKeyExposesHasFile(t *testing.T) {
	key := "assets/x/documents/y.pdf"
	no := "BAST-1"
	d := sqlc.AssetAssetDocument{
		ID:        uuid.New(),
		AssetID:   uuid.New(),
		DocType:   "bast_transfer",
		DocNo:     &no,
		ObjectKey: &key,
	}
	m := documentToMap(d)
	if _, ok := m["object_key"]; ok {
		t.Fatal("object_key must not be serialized")
	}
	if m["has_file"] != true {
		t.Fatal("has_file should be true when object_key set")
	}
	if m["doc_type"] != "bast_transfer" || m["doc_no"] != &no {
		t.Fatalf("unexpected map: %v", m)
	}

	d2 := sqlc.AssetAssetDocument{ID: uuid.New(), AssetID: uuid.New(), DocType: "other"}
	if documentToMap(d2)["has_file"] != false {
		t.Fatal("has_file should be false when object_key nil")
	}
}
```

Note: `sqlc` is already imported by sibling files in package `asset`; add the import to this test file if `go vet` flags it (it references `sqlc.AssetAssetDocument`).

- [ ] **Step 2: Run tests to verify they fail**

Run (from `backend/`): `go test ./internal/asset/ -run TestDocument -v`
Expected: FAIL — `undefined: DocumentCreateRequest`, `undefined: documentToMap`.

- [ ] **Step 3: Write `document_dto.go`**

Create `backend/internal/asset/document_dto.go`:

```go
package asset

import (
	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// DocumentCreateRequest is the JSON body for creating an asset document (metadata only;
// the file is attached separately via the file sub-resource).
type DocumentCreateRequest struct {
	DocType          string  `json:"doc_type" binding:"required,oneof=bast_acquisition bast_transfer bast_disposal invoice contract other"`
	DocNo            *string `json:"doc_no"`
	DocDate          *string `json:"doc_date"`
	Counterparty     *string `json:"counterparty"`
	RelatedRequestID *string `json:"related_request_id" binding:"omitempty,uuid"`
}

// DocumentUpdateRequest is the JSON body for editing document metadata.
type DocumentUpdateRequest struct {
	DocType          string  `json:"doc_type" binding:"required,oneof=bast_acquisition bast_transfer bast_disposal invoice contract other"`
	DocNo            *string `json:"doc_no"`
	DocDate          *string `json:"doc_date"`
	Counterparty     *string `json:"counterparty"`
	RelatedRequestID *string `json:"related_request_id" binding:"omitempty,uuid"`
}

// toInput parses the create request into a DocumentInput (defined in document.go).
func (r DocumentCreateRequest) toInput(assetID, createdBy uuid.UUID) (DocumentInput, error) {
	date, err := parseDate(r.DocDate)
	if err != nil {
		return DocumentInput{}, err
	}
	reqID, err := common.ParseUUIDPtr(r.RelatedRequestID)
	if err != nil {
		return DocumentInput{}, err
	}
	return DocumentInput{
		AssetID:          assetID,
		DocType:          sqlc.SharedAssetDocumentType(r.DocType),
		DocNo:            r.DocNo,
		DocDate:          date,
		Counterparty:     r.Counterparty,
		RelatedRequestID: reqID,
		CreatedBy:        createdBy,
	}, nil
}

// toUpdateInput parses the update request into a DocumentUpdateInput (defined in document.go).
func (r DocumentUpdateRequest) toUpdateInput() (DocumentUpdateInput, error) {
	date, err := parseDate(r.DocDate)
	if err != nil {
		return DocumentUpdateInput{}, err
	}
	reqID, err := common.ParseUUIDPtr(r.RelatedRequestID)
	if err != nil {
		return DocumentUpdateInput{}, err
	}
	return DocumentUpdateInput{
		DocType:          sqlc.SharedAssetDocumentType(r.DocType),
		DocNo:            r.DocNo,
		DocDate:          date,
		Counterparty:     r.Counterparty,
		RelatedRequestID: reqID,
	}, nil
}

// documentToMap serializes a document for the API response. object_key is intentionally
// omitted (storage-internal); has_file is derived so callers can show a download affordance.
func documentToMap(d sqlc.AssetAssetDocument) map[string]any {
	return map[string]any{
		"id":                  d.ID.String(),
		"asset_id":            d.AssetID.String(),
		"doc_type":            string(d.DocType),
		"doc_no":              d.DocNo,
		"doc_date":            dateStr(d.DocDate),
		"counterparty":        d.Counterparty,
		"related_request_id":  common.UUIDPtrStr(d.RelatedRequestID),
		"related_transfer_id": common.UUIDPtrStr(d.RelatedTransferID),
		"related_disposal_id": common.UUIDPtrStr(d.RelatedDisposalID),
		"has_file":            d.ObjectKey != nil,
		"created_by_id":       common.UUIDPtrStr(d.CreatedByID),
		"created_at":          common.TsStr(d.CreatedAt),
		"updated_at":          common.TsStr(d.UpdatedAt),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run (from `backend/`): `go test ./internal/asset/ -run TestDocument -v`
Expected: PASS (compiles once Task 3 types exist; if `DocumentInput` is still undefined, proceed to Task 3 then re-run — these two files compile together). To keep this task self-contained, implement Task 3's type definitions block first if the compiler complains.

> **Note:** `DocumentInput` and `DocumentUpdateInput` are declared in `document.go` (Task 3). Because Go compiles the package as a unit, run `go test` for Tasks 2–3 together after both files exist. If executing strictly task-by-task, add the two struct definitions (from Task 3, Step 3) at the top of `document.go` as part of this task's compile.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/asset/document_dto.go backend/internal/asset/document_dto_test.go
git commit -m "feat(asset): document DTOs and serialization"
```

---

### Task 3: Document service layer (unit-tested)

**Files:**
- Create: `backend/internal/asset/document.go`
- Test: `backend/internal/asset/document_test.go`

**Interfaces:**
- Consumes: `*Service` (fields `q *sqlc.Queries`, `store storage.Storage`, `maxBytes int64`); `allowedMIME`, `extFor` (attachment.go); `mapDBError`, `ErrNotFound`, `ErrInvalidRef` (service.go); `ErrUnsupportedType`, `ErrTooLarge` (attachment.go); generated queries from Task 1.
- Produces: types `DocumentInput`, `DocumentUpdateInput`, `DocumentFileInput`; methods `CreateDocument`, `ListDocuments`, `GetDocument`, `UpdateDocument`, `DeleteDocument`, `AttachFile`, `OpenDocumentFile`.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/asset/document_test.go`:

```go
package asset

import (
	"context"
	"errors"
	"testing"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

func TestAttachFile_RejectsTypeAndSize(t *testing.T) {
	// q=nil intentional: validation must fire BEFORE any DB/storage call.
	s := NewService(nil, nil, storage.NewFake(), 10, "")
	doc := sqlc.AssetAssetDocument{ID: mustUUID(t), AssetID: mustUUID(t)}

	_, err := s.AttachFile(context.Background(), doc, DocumentFileInput{
		ContentType: "application/zip", Data: []byte("x"),
	})
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("want ErrUnsupportedType, got %v", err)
	}

	_, err = s.AttachFile(context.Background(), doc, DocumentFileInput{
		ContentType: "application/pdf", Data: make([]byte, 11),
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}

func TestAttachFile_RollbackOnDBError(t *testing.T) {
	// Put succeeds but the DB update fails (q=nil panics? no — use PutErr to stop before DB).
	f := storage.NewFake()
	f.PutErr = errors.New("boom")
	s := NewService(nil, nil, f, 1024, "")
	doc := sqlc.AssetAssetDocument{ID: mustUUID(t), AssetID: mustUUID(t)}

	_, err := s.AttachFile(context.Background(), doc, DocumentFileInput{
		ContentType: "application/pdf", Data: []byte("pdf"),
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("want put error, got %v", err)
	}
	if len(f.ObjsKeys()) != 0 {
		t.Fatalf("no object should remain, got %v", f.ObjsKeys())
	}
}

func TestOpenDocumentFile_NilObjectKey(t *testing.T) {
	s := NewService(nil, nil, storage.NewFake(), 1024, "")
	doc := sqlc.AssetAssetDocument{ID: mustUUID(t), AssetID: mustUUID(t)} // ObjectKey nil
	_, _, err := s.OpenDocumentFile(context.Background(), doc)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

Add a tiny helper at the bottom of the file (or reuse one if present in the package — check `dberr_test.go`/`state_test.go` first; if a `mustUUID` already exists, delete this and use it):

```go
func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}
```

…and add `"github.com/google/uuid"` to the test imports. If a UUID helper already exists in the package's test files, omit this and reuse it to avoid a redeclaration.

- [ ] **Step 2: Run tests to verify they fail**

Run (from `backend/`): `go test ./internal/asset/ -run 'TestAttachFile|TestOpenDocumentFile' -v`
Expected: FAIL — `undefined: DocumentFileInput`, `undefined: (*Service).AttachFile`.

- [ ] **Step 3: Write `document.go`**

Create `backend/internal/asset/document.go`:

```go
package asset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

// DocumentInput holds the metadata for creating an asset document.
type DocumentInput struct {
	AssetID          uuid.UUID
	DocType          sqlc.SharedAssetDocumentType
	DocNo            *string
	DocDate          pgtype.Date
	Counterparty     *string
	RelatedRequestID *uuid.UUID
	CreatedBy        uuid.UUID
}

// DocumentUpdateInput holds the editable metadata for an asset document.
type DocumentUpdateInput struct {
	DocType          sqlc.SharedAssetDocumentType
	DocNo            *string
	DocDate          pgtype.Date
	Counterparty     *string
	RelatedRequestID *uuid.UUID
}

// DocumentFileInput carries an uploaded document file.
type DocumentFileInput struct {
	ContentType string
	Data        []byte
}

// CreateDocument inserts a document metadata row (no file yet).
func (s *Service) CreateDocument(ctx context.Context, in DocumentInput) (sqlc.AssetAssetDocument, error) {
	cb := in.CreatedBy
	row, err := s.q.CreateAssetDocument(ctx, sqlc.CreateAssetDocumentParams{
		AssetID:          in.AssetID,
		DocType:          in.DocType,
		DocNo:            in.DocNo,
		DocDate:          in.DocDate,
		Counterparty:     in.Counterparty,
		RelatedRequestID: in.RelatedRequestID,
		CreatedByID:      &cb,
	})
	return row, mapDBError(err)
}

// ListDocuments returns all non-deleted documents for an asset (newest first).
func (s *Service) ListDocuments(ctx context.Context, assetID uuid.UUID) ([]sqlc.AssetAssetDocument, error) {
	rows, err := s.q.ListAssetDocuments(ctx, assetID)
	return rows, mapDBError(err)
}

// GetDocument returns a single document by ID, or ErrNotFound.
func (s *Service) GetDocument(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetDocument, error) {
	row, err := s.q.GetAssetDocument(ctx, id)
	return row, mapDBError(err)
}

// UpdateDocument applies metadata edits and returns before/after for audit diffing.
func (s *Service) UpdateDocument(ctx context.Context, id uuid.UUID, in DocumentUpdateInput) (before, after sqlc.AssetAssetDocument, err error) {
	before, err = s.q.GetAssetDocument(ctx, id)
	if err != nil {
		return before, before, mapDBError(err)
	}
	after, err = s.q.UpdateAssetDocument(ctx, sqlc.UpdateAssetDocumentParams{
		ID:               id,
		DocType:          in.DocType,
		DocNo:            in.DocNo,
		DocDate:          in.DocDate,
		Counterparty:     in.Counterparty,
		RelatedRequestID: in.RelatedRequestID,
	})
	return before, after, mapDBError(err)
}

// DeleteDocument soft-deletes a document and best-effort removes its stored file.
func (s *Service) DeleteDocument(ctx context.Context, id uuid.UUID) (sqlc.AssetAssetDocument, error) {
	doc, err := s.q.GetAssetDocument(ctx, id)
	if err != nil {
		return doc, mapDBError(err)
	}
	n, err := s.q.SoftDeleteAssetDocument(ctx, id)
	if err != nil {
		return doc, mapDBError(err)
	}
	if n == 0 {
		return doc, ErrNotFound
	}
	if doc.ObjectKey != nil {
		_ = s.store.Remove(ctx, *doc.ObjectKey)
	}
	return doc, nil
}

// AttachFile validates and stores the file, updates object_key, and best-effort removes
// any previously stored object. Validation fires before any storage/DB call.
func (s *Service) AttachFile(ctx context.Context, doc sqlc.AssetAssetDocument, in DocumentFileInput) (sqlc.AssetAssetDocument, error) {
	var zero sqlc.AssetAssetDocument
	if !allowedMIME(in.ContentType) {
		return zero, ErrUnsupportedType
	}
	if int64(len(in.Data)) > s.maxBytes {
		return zero, ErrTooLarge
	}

	newKey := fmt.Sprintf("assets/%s/documents/%s.%s", doc.AssetID, doc.ID, extFor(in.ContentType))
	if err := s.store.Put(ctx, newKey, bytes.NewReader(in.Data), int64(len(in.Data)), in.ContentType); err != nil {
		return zero, err
	}

	row, err := s.q.SetAssetDocumentObjectKey(ctx, sqlc.SetAssetDocumentObjectKeyParams{
		ID:        doc.ID,
		ObjectKey: &newKey,
	})
	if err != nil {
		_ = s.store.Remove(ctx, newKey) // rollback the just-uploaded object
		return zero, mapDBError(err)
	}

	// Remove the previous object only when the key actually changed (same ext => same key).
	if doc.ObjectKey != nil && *doc.ObjectKey != newKey {
		_ = s.store.Remove(ctx, *doc.ObjectKey)
	}
	return row, nil
}

// OpenDocumentFile returns a reader for the document's file, or ErrNotFound when the
// document has no file or the object is missing.
func (s *Service) OpenDocumentFile(ctx context.Context, doc sqlc.AssetAssetDocument) (io.ReadCloser, storage.ObjectInfo, error) {
	if doc.ObjectKey == nil {
		return nil, storage.ObjectInfo{}, ErrNotFound
	}
	rc, info, err := s.store.Get(ctx, *doc.ObjectKey)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return nil, storage.ObjectInfo{}, ErrNotFound
	}
	return rc, info, err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run (from `backend/`): `go test ./internal/asset/ -run 'TestAttachFile|TestOpenDocumentFile|TestDocument' -v`
Expected: PASS.

- [ ] **Step 5: Verify build + vet**

Run (from `backend/`): `go build ./... && go vet ./...`
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/asset/document.go backend/internal/asset/document_test.go
git commit -m "feat(asset): document service (CRUD + optional MinIO file)"
```

---

### Task 4: HTTP handlers + route wiring

**Files:**
- Create: `backend/internal/asset/document_handler.go`
- Modify: `backend/internal/asset/routes.go` (add the document group)
- Modify: `backend/internal/asset/attachment_handler.go:61-72` (extend `handleErr` to map `ErrInvalidRef` → 400)

**Interfaces:**
- Consumes: `*Handler` (fields `svc`, `aud`, `scoped`); `resolveAssetInScope`, `contentDisposition`, `handleErr` (attachment_handler.go); `documentToMap`, `DocumentCreateRequest`, `DocumentUpdateRequest` (Task 2); service methods (Task 3); `audit.Record`, `audit.ActionCreate/Update/Delete`, `audit.Diff`; `middleware.CtxUserID`.
- Produces: route handlers `createDocument`, `listDocuments`, `getDocument`, `updateDocument`, `deleteDocument`, `uploadDocumentFile`, `downloadDocumentFile`; helper `resolveDoc`, `docDownloadName`.

- [ ] **Step 1: Extend `handleErr` to map `ErrInvalidRef`**

In `backend/internal/asset/attachment_handler.go`, inside `handleErr`, add a case before `default`:

```go
	case errors.Is(err, ErrInvalidRef):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
```

(Attachments never return `ErrInvalidRef`, so this is additive and safe.)

- [ ] **Step 2: Write `document_handler.go`**

Create `backend/internal/asset/document_handler.go`:

```go
package asset

import (
	"errors"
	"io"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// resolveDoc loads the :docId document and verifies it belongs to assetID.
func (h *Handler) resolveDoc(c *gin.Context, assetID uuid.UUID) (sqlc.AssetAssetDocument, bool) {
	docID, err := uuid.Parse(c.Param("docId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return sqlc.AssetAssetDocument{}, false
	}
	doc, err := h.svc.GetDocument(c.Request.Context(), docID)
	if err != nil {
		h.handleErr(c, err)
		return sqlc.AssetAssetDocument{}, false
	}
	if doc.AssetID != assetID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return sqlc.AssetAssetDocument{}, false
	}
	return doc, true
}

// docDownloadName derives a safe download filename: doc_no (or doc_type) + the stored extension.
func docDownloadName(d sqlc.AssetAssetDocument) string {
	base := string(d.DocType)
	if d.DocNo != nil && *d.DocNo != "" {
		base = *d.DocNo
	}
	ext := ""
	if d.ObjectKey != nil {
		ext = path.Ext(*d.ObjectKey)
	}
	return base + ext
}

// createDocument handles POST /assets/:id/documents.
func (h *Handler) createDocument(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	var req DocumentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, _ := uuid.Parse(c.GetString(middleware.CtxUserID))
	in, err := req.toInput(assetID, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := h.svc.CreateDocument(c.Request.Context(), in)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionCreate, "asset_documents", row.ID, &oid,
		audit.Diff(nil, documentToMap(row)))
	c.JSON(http.StatusCreated, documentToMap(row))
}

// listDocuments handles GET /assets/:id/documents.
func (h *Handler) listDocuments(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListDocuments(c.Request.Context(), assetID)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, documentToMap(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": len(data)})
}

// getDocument handles GET /assets/:id/documents/:docId.
func (h *Handler) getDocument(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, documentToMap(doc))
}

// updateDocument handles PUT /assets/:id/documents/:docId.
func (h *Handler) updateDocument(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	var req DocumentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := req.toUpdateInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, after, err := h.svc.UpdateDocument(c.Request.Context(), doc.ID, in)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionUpdate, "asset_documents", after.ID, &oid,
		audit.Diff(documentToMap(before), documentToMap(after)))
	c.JSON(http.StatusOK, documentToMap(after))
}

// deleteDocument handles DELETE /assets/:id/documents/:docId.
func (h *Handler) deleteDocument(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	if _, err := h.svc.DeleteDocument(c.Request.Context(), doc.ID); err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionDelete, "asset_documents", doc.ID, &oid,
		audit.Diff(documentToMap(doc), nil))
	c.Status(http.StatusNoContent)
}

// uploadDocumentFile handles PUT /assets/:id/documents/:docId/file (multipart, field "file").
func (h *Handler) uploadDocumentFile(c *gin.Context) {
	assetID, officeID, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.maxBytes+1)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	before := documentToMap(doc)
	row, err := h.svc.AttachFile(c.Request.Context(), doc, DocumentFileInput{ContentType: contentType, Data: data})
	if err != nil {
		h.handleErr(c, err)
		return
	}
	oid := officeID
	audit.Record(c, h.aud, audit.ActionUpdate, "asset_documents", row.ID, &oid,
		audit.Diff(before, documentToMap(row)))
	c.JSON(http.StatusOK, documentToMap(row))
}

// downloadDocumentFile handles GET /assets/:id/documents/:docId/file.
func (h *Handler) downloadDocumentFile(c *gin.Context) {
	assetID, _, ok := h.resolveAssetInScope(c)
	if !ok {
		return
	}
	doc, ok := h.resolveDoc(c, assetID)
	if !ok {
		return
	}
	rc, info, err := h.svc.OpenDocumentFile(c.Request.Context(), doc)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	defer rc.Close()
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Content-Disposition", contentDisposition(docDownloadName(doc)))
	c.DataFromReader(http.StatusOK, info.Size, info.ContentType, rc, nil)
}
```

- [ ] **Step 3: Wire routes**

In `backend/internal/asset/routes.go`, add inside `RegisterRoutes` after the attachment group:

```go
	d := g.Group("/:id/documents")
	d.POST("", authMW, requireManage, h.createDocument)
	d.GET("", authMW, requireView, h.listDocuments)
	d.GET("/:docId", authMW, requireView, h.getDocument)
	d.PUT("/:docId", authMW, requireManage, h.updateDocument)
	d.DELETE("/:docId", authMW, requireManage, h.deleteDocument)
	d.PUT("/:docId/file", authMW, requireManage, h.uploadDocumentFile)
	d.GET("/:docId/file", authMW, requireView, h.downloadDocumentFile)
```

- [ ] **Step 4: Verify build + vet + unit tests**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./internal/asset/...`
Expected: exit 0, tests PASS.

> If Gin panics at router construction with a wildcard/param conflict, it is because `:aid` (attachments) and `:docId` (documents) sit under distinct static segments and should NOT conflict — re-check the group paths are exactly `/:id/attachments` and `/:id/documents`. A genuine conflict means a typo.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/asset/document_handler.go backend/internal/asset/routes.go backend/internal/asset/attachment_handler.go
git commit -m "feat(asset): document HTTP handlers + routes"
```

---

### Task 5: Integration tests (real Postgres + MinIO)

**Files:**
- Create: `backend/internal/asset/document_integration_test.go`

**Interfaces:**
- Consumes: `internal/testsupport` (Postgres/MinIO containers, migrate, seed), `internal/asset` exported handler/service constructors, `storage` MinIO impl. Mirror the structure of `attachment_integration_test.go` (same package `asset_test`, same router/seed helpers).

- [ ] **Step 1: Study the existing integration harness**

Read `backend/internal/asset/attachment_integration_test.go` in full and `backend/internal/testsupport/` to reuse: container setup, migration apply, asset/office/role seeding, JWT/context injection, and the multipart helper `attBuildMultipart`. Reuse these helpers — do not duplicate container bring-up.

- [ ] **Step 2: Write the integration tests**

Create `backend/internal/asset/document_integration_test.go` with build tag `//go:build integration`, package `asset_test`. Implement these cases (reuse the seed + router helpers already used by the attachment integration test; reuse `attBuildMultipart`, `attMakePDF`, `attMakePNG`):

```go
//go:build integration

package asset_test

// Cases (use require/assert; mirror attachment_integration_test.go setup):
//
// 1. Create_MetadataOnly: POST /assets/:id/documents {doc_type:"bast_acquisition",
//    doc_no:"BAST-1"} -> 201, body has_file=false, no object_key key present.
// 2. AttachAndDownload_RoundTrip: create doc; PUT .../file (PDF) -> 200 has_file=true;
//    GET .../file -> 200, bytes byte-identical to uploaded PDF, headers include
//    X-Content-Type-Options: nosniff and Content-Disposition.
// 3. ReplaceFile_RemovesOld: attach PNG then attach PDF; assert the MinIO object for the
//    old key is gone and the new key exists (query storage via the same MinIO client, or
//    assert GET returns the new content-type). Old/new keys differ by extension.
// 4. ListAndGet: create two docs -> GET list returns total=2 newest-first; GET one by id ok.
// 5. WrongAssetDoc_404: create doc under asset A; GET /assets/{B}/documents/{docA} -> 404.
// 6. Scope_Forbidden: caller whose office scope excludes the asset's office gets 403 on
//    POST (create), GET (list), and GET .../file.
// 7. Delete_RemovesObject: attach file, DELETE doc -> 204; row soft-deleted (GET -> 404)
//    and MinIO object removed (no orphan).
// 8. InvalidRelatedRequest_400: POST with related_request_id = random UUID (no such
//    request) -> 400 (FK violation mapped to ErrInvalidRef).
// 9. UploadOversize_413 and UploadDisallowedType_415: PUT .../file with >maxBytes body
//    -> 413; with a .zip part -> 415.
// 10. DownloadNoFile_404: create metadata-only doc, GET .../file -> 404.
//
// Each case constructs the gin router via the same NewRouter/seed path used in
// attachment_integration_test.go and authenticates as a seeded user with asset.manage.
```

Write each case as a concrete `t.Run(...)` with real assertions following the patterns in `attachment_integration_test.go`. Do not leave the comment block as the implementation — it is the checklist; replace it with executable tests.

- [ ] **Step 3: Run integration tests**

Run (from `backend/`): `go test -tags=integration ./internal/asset/ -run TestDocument -v`
Expected: PASS (Docker must be available for testcontainers).

- [ ] **Step 4: Run the full integration suite (shared-signature safety gate)**

Run (from `backend/`): `go test -tags=integration ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/asset/document_integration_test.go
git commit -m "test(asset): integration coverage for asset documents"
```

---

### Task 6: OpenAPI spec + PROGRESS + final verification

**Files:**
- Modify: `backend/api/openapi.yaml` (add the document paths + schemas, mirroring the attachment endpoints)
- Modify: `docs/PROGRESS.md`

**Interfaces:** none (docs).

- [ ] **Step 1: Add OpenAPI paths + schemas**

In `backend/api/openapi.yaml`, locate the existing `/assets/{id}/attachments` paths and add analogous entries:
- `/assets/{id}/documents` — `get` (list) + `post` (create, JSON `DocumentCreateRequest`).
- `/assets/{id}/documents/{docId}` — `get`, `put` (`DocumentUpdateRequest`), `delete`.
- `/assets/{id}/documents/{docId}/file` — `put` (multipart `file`), `get` (binary download).

Add response schema `AssetDocument` with: `id`, `asset_id`, `doc_type` (enum: `bast_acquisition`,`bast_transfer`,`bast_disposal`,`invoice`,`contract`,`other`), `doc_no`, `doc_date`, `counterparty`, `related_request_id`, `related_transfer_id`, `related_disposal_id`, `has_file` (boolean), `created_by_id`, `created_at`, `updated_at`. Add request bodies `DocumentCreateRequest`/`DocumentUpdateRequest` (`doc_type` required; others nullable). Reuse the existing security scheme + error responses (`401`/`403`/`404`) used by the attachment paths.

- [ ] **Step 2: Lint the spec**

Run (from repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors.

- [ ] **Step 3: Update PROGRESS.md**

In `docs/PROGRESS.md`:
- Under **Bank-FAM** remaining list, change `- [ ] **Asset documents (BAST)**` to `- [x]` with a note: `metadata CRUD + optional MinIO file; scope-gated + audited; integration tests. **Done — (2026-06-28).**`
- Under the **Backend — Feature modules** list, flip the matching `Asset documents (BAST)` line if present.
- Refresh the **"▶ Next session — start here"** block: remove "Asset documents (BAST)" from the next-priorities and point at the remaining priorities (wire frontend Asset/Approval screens after ADR-0007 refactor, or asset transfer/mutasi).

- [ ] **Step 4: Full verification gate**

Run (from `backend/`):
```
go build ./...
go vet ./...
go test ./...
go test -tags=integration ./...
```
Then (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: all exit 0 / PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/api/openapi.yaml docs/PROGRESS.md
git commit -m "docs(asset): openapi + progress for asset documents"
```

---

## Self-Review

**Spec coverage:**
- bagian 3 file placement → Tasks 2–4 (document_dto.go / document.go / document_handler.go + routes.go). ✓
- bagian 4 endpoints (7) → Task 4 routes + handlers. ✓
- bagian 5 validation (doc_type oneof, date parse, related_request FK, file MIME/size, nosniff/CSP/disposition, NULL→404) → Tasks 2 (DTO), 3 (service validateFile/Open), 4 (download headers). ✓
- bagian 6 audit/rollback/sentinels → Task 3 (rollback, sentinels) + Task 4 (audit.Record). ✓
- bagian 7 queries (6) → Task 1. ✓
- bagian 8 tests (unit + integration list) → Tasks 2, 3 (unit), 5 (integration; all 10 cases mapped). ✓
- bagian 9 OpenAPI + PROGRESS + verification → Task 6. ✓

**Placeholder scan:** No "TBD"/"implement later". Task 5 uses a checklist comment but Step 2 explicitly requires replacing it with executable `t.Run` tests — flagged, not a hidden placeholder.

**Type consistency:** `DocumentInput`/`DocumentUpdateInput`/`DocumentFileInput` defined in Task 3, referenced by Task 2 (`toInput`/`toUpdateInput`) and Task 4 — names + fields match. `documentToMap` signature consistent across Tasks 2/4. Service method names (`CreateDocument`,`ListDocuments`,`GetDocument`,`UpdateDocument`,`DeleteDocument`,`AttachFile`,`OpenDocumentFile`) identical in Tasks 3 and 4. sqlc params (`CreateAssetDocumentParams`,`UpdateAssetDocumentParams`,`SetAssetDocumentObjectKeyParams`) consistent Task 1 ↔ Task 3.

**Cross-package compile note:** Tasks 2 and 3 share package `asset` and must compile together; Task 2 Step 4 and Task 3 Step 1 both note this. Acceptable for task-by-task execution as long as Task 3's type block lands with (or before) Task 2's `go test`.
