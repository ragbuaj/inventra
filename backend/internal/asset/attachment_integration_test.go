//go:build integration

package asset_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/storage"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── image / file helpers ────────────────────────────────────────────────────

// attMakePNG returns a minimal valid 1×1 red PNG encoded as bytes.
func attMakePNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// attMakePDF returns a minimal valid-enough PDF byte slice.
func attMakePDF() []byte {
	return []byte("%PDF-1.4\n%%EOF\n")
}

// attBuildMultipart creates a multipart/form-data body with a single "file" part.
// When partCT is empty the Content-Type part header is omitted.
func attBuildMultipart(t *testing.T, filename, partCT string, data []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	if partCT != "" {
		h.Set("Content-Type", partCT)
	}
	fw, err := w.CreatePart(h)
	require.NoError(t, err)
	_, err = io.Copy(fw, bytes.NewReader(data))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

// ─── harness ─────────────────────────────────────────────────────────────────

// attHarness holds all wired-up resources for an attachment HTTP integration test.
type attHarness struct {
	router   *gin.Engine
	svc      *asset.Service
	store    *storage.MinIOStorage
	assetID  uuid.UUID
	officeID uuid.UUID
	userID   uuid.UUID
	roleID   uuid.UUID
}

// attNewHarness boots a throwaway Postgres + MinIO testcontainer, seeds one office
// + category + asset, wires a gin router with stub auth (no JWT), and returns the
// harness.  maxBytes controls the service's upload size limit.
//
// The seeded caller has the migration-seeded "Superadmin" role (global scope),
// so all endpoints are in-scope by default.
func attNewHarness(t *testing.T, maxBytes int64) *attHarness {
	t.Helper()
	ctx := context.Background()

	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	minioStore := testsupport.NewMinIO(t)

	// Clean mutable tables; leave migration-seeded roles & scope policies intact.
	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)

	officeID := seedOfficeWithType(t, pool, "AttachOfficeType", "ATCH01")
	catID := seedCategory(t, pool, "ATC")
	assetID := seedAssetDirect(t, pool, "ATCH01-ATC-2026-00001", "Attachment Test Asset", catID, officeID)

	// Superadmin has global scope — migration 000005 seeds both the role and its policy.
	roleID := lookupRole(t, pool, "Superadmin")

	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"att-caller", "att-caller@test.local", roleID, officeID).Scan(&userID))

	svc := asset.NewService(q, pool, minioStore, maxBytes)
	fieldSvc := authz.NewFieldService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	audSvc := audit.NewService(q)
	h := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := router.Group("/api/v1")

	// Stub auth: injects CtxUserID / CtxRoleID without any JWT verification.
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	passThrough := func(c *gin.Context) { c.Next() }

	asset.RegisterRoutes(rg, h, stubAuth, passThrough, passThrough)

	return &attHarness{
		router:   router,
		svc:      svc,
		store:    minioStore,
		assetID:  assetID,
		officeID: officeID,
		userID:   userID,
		roleID:   roleID,
	}
}

// do fires a single HTTP request against the harness router and returns the recorder.
func (h *attHarness) do(t *testing.T, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// attPath returns the base attachments path for the harness asset.
func (h *attHarness) attPath() string {
	return "/api/v1/assets/" + h.assetID.String() + "/attachments"
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestAttachment_ImageRoundTrip is the full lifecycle test:
//
//  1. POST a PNG → 201, has_thumbnail=true, no object_key exposed.
//  2. GET /attachments → 200, list contains exactly the new item.
//  3. GET /:aid/content → exact uploaded bytes, image/png Content-Type,
//     X-Content-Type-Options:nosniff and Content-Security-Policy:sandbox present.
//  4. GET /:aid/thumbnail → 200, body decodes as JPEG.
//  5. DELETE /:aid → 204.
//  6. GET /:aid/content → 404; GET /attachments → 200, list empty.
func TestAttachment_ImageRoundTrip(t *testing.T) {
	h := attNewHarness(t, 5<<20)
	pngData := attMakePNG()
	base := h.attPath()

	// ── 1. Upload PNG ──────────────────────────────────────────────────────────
	body, ct := attBuildMultipart(t, "photo.png", "image/png", pngData)
	w := h.do(t, http.MethodPost, base, body, ct)
	require.Equal(t, http.StatusCreated, w.Code, "upload PNG → 201; body: %s", w.Body.String())

	var uploadResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uploadResp))
	assert.Equal(t, true, uploadResp["has_thumbnail"], "image upload must set has_thumbnail=true")
	assert.NotContains(t, uploadResp, "object_key", "object_key must not appear in response")
	assert.NotContains(t, uploadResp, "thumbnail_key", "thumbnail_key must not appear in response")
	attachmentID, ok := uploadResp["id"].(string)
	require.True(t, ok, "response must contain string id")

	// ── 2. List ────────────────────────────────────────────────────────────────
	w = h.do(t, http.MethodGet, base, nil, "")
	require.Equal(t, http.StatusOK, w.Code, "list → 200")
	var listResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	items, _ := listResp["data"].([]any)
	assert.Len(t, items, 1, "list must contain exactly one attachment")
	assert.Equal(t, float64(1), listResp["total"])

	// ── 3. Content download ────────────────────────────────────────────────────
	contentPath := base + "/" + attachmentID + "/content"
	w = h.do(t, http.MethodGet, contentPath, nil, "")
	require.Equal(t, http.StatusOK, w.Code, "content download → 200")
	assert.Equal(t, pngData, w.Body.Bytes(), "downloaded bytes must match uploaded bytes exactly")
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"),
		"Content-Type must be image/png")
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"),
		"must set X-Content-Type-Options: nosniff")
	assert.Equal(t, "sandbox", w.Header().Get("Content-Security-Policy"),
		"must set Content-Security-Policy: sandbox")

	// ── 4. Thumbnail ───────────────────────────────────────────────────────────
	thumbPath := base + "/" + attachmentID + "/thumbnail"
	w = h.do(t, http.MethodGet, thumbPath, nil, "")
	require.Equal(t, http.StatusOK, w.Code, "thumbnail → 200")
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"),
		"thumbnail Content-Type must be image/jpeg")
	_, _, decodeErr := image.Decode(bytes.NewReader(w.Body.Bytes()))
	assert.NoError(t, decodeErr, "thumbnail body must decode as a valid image")

	// ── 5. Delete ──────────────────────────────────────────────────────────────
	deletePath := base + "/" + attachmentID
	w = h.do(t, http.MethodDelete, deletePath, nil, "")
	require.Equal(t, http.StatusNoContent, w.Code, "delete → 204")

	// ── 6. Post-delete: content → 404, list → empty ───────────────────────────
	w = h.do(t, http.MethodGet, contentPath, nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code, "content after delete → 404")

	w = h.do(t, http.MethodGet, base, nil, "")
	require.Equal(t, http.StatusOK, w.Code)
	var listAfter map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listAfter))
	itemsAfter, _ := listAfter["data"].([]any)
	assert.Len(t, itemsAfter, 0, "list after delete must be empty (soft-deleted row excluded)")
	assert.Equal(t, float64(0), listAfter["total"])
}

// TestAttachment_PDFUpload verifies that a PDF upload:
//   - returns 201 with has_thumbnail=false
//   - thumbnail endpoint returns 404
func TestAttachment_PDFUpload(t *testing.T) {
	h := attNewHarness(t, 5<<20)
	pdfData := attMakePDF()
	base := h.attPath()

	body, ct := attBuildMultipart(t, "document.pdf", "application/pdf", pdfData)
	w := h.do(t, http.MethodPost, base, body, ct)
	require.Equal(t, http.StatusCreated, w.Code, "PDF upload → 201; body: %s", w.Body.String())

	var uploadResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uploadResp))
	assert.Equal(t, false, uploadResp["has_thumbnail"], "PDF upload must set has_thumbnail=false")
	assert.Equal(t, "application/pdf", uploadResp["mime_type"])

	attachmentID, ok := uploadResp["id"].(string)
	require.True(t, ok, "response must contain string id")

	// Thumbnail for a PDF → 404.
	thumbPath := base + "/" + attachmentID + "/thumbnail"
	w = h.do(t, http.MethodGet, thumbPath, nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code, "PDF thumbnail → 404")
}

// TestAttachment_Oversize verifies that uploading a file whose total multipart body
// exceeds maxBytes+1 returns 413. The handler sets MaxBytesReader on the request
// body; when the multipart body exceeds the limit, FormFile fails with
// *http.MaxBytesError, which the handler maps to 413.
//
// We use a raw 500-byte payload (content-type image/png, not a real PNG — validation
// happens after the size check) and set maxBytes=200 so that the total multipart body
// (~500 + ~223 overhead = ~723 bytes) exceeds maxBytes+1=201.
func TestAttachment_Oversize(t *testing.T) {
	// 200-byte limit. A 500-byte file gives a total body of ~723 bytes, well above 201.
	const maxBytes = 200

	// Raw data that exceeds maxBytes. Content-type declared as image/png so the
	// MIME check would pass if we ever got that far, but we won't (size rejected first).
	largeData := bytes.Repeat([]byte{0xff, 0xd8, 0xff, 0xe0}, 125) // 500 bytes, fake JPEG prefix
	require.Greater(t, len(largeData), maxBytes,
		"test pre-condition: data (%d B) must exceed maxBytes (%d)", len(largeData), maxBytes)

	h := attNewHarness(t, maxBytes)
	base := h.attPath()

	body, ct := attBuildMultipart(t, "big.png", "image/png", largeData)
	w := h.do(t, http.MethodPost, base, body, ct)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code,
		"oversize upload → 413; body: %s", w.Body.String())
}

// TestAttachment_DisallowedType verifies that uploading an unsupported MIME type
// (application/zip) returns 415.
func TestAttachment_DisallowedType(t *testing.T) {
	h := attNewHarness(t, 5<<20)
	zipData := []byte("PK\x03\x04fake zip content")
	base := h.attPath()

	body, ct := attBuildMultipart(t, "archive.zip", "application/zip", zipData)
	w := h.do(t, http.MethodPost, base, body, ct)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code,
		"disallowed type → 415; body: %s", w.Body.String())
}

// TestAttachment_ScopeEnforcement verifies that a caller whose data scope (office-level
// restricted to officeB) excludes the asset's office (officeA) gets 403 on all
// attachment verbs: upload, list, content download, thumbnail, and delete.
func TestAttachment_ScopeEnforcement(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	minioStore := testsupport.NewMinIO(t)

	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)

	// Seed two unrelated offices.
	officeA := seedOfficeWithType(t, pool, "ScopeOfficeType", "SCPA")
	var officeTypeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT office_type_id FROM masterdata.offices WHERE id = $1`, officeA).Scan(&officeTypeID))
	var officeB uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Office B', 'SCPB') RETURNING id`, officeTypeID).Scan(&officeB))

	// Asset lives in officeA.
	catID := seedCategory(t, pool, "SPC")
	assetID := seedAssetDirect(t, pool, "SCPA-SPC-2026-00001", "Scoped Asset", catID, officeA)

	// Restricted role: office-level scope (only the caller's own office).
	restrictedRoleID := testsupport.SeedRole(t, pool, "ScopeEnfRole")
	testsupport.SeedScopePolicy(t, pool, restrictedRoleID, "*", sqlc.SharedScopeLevelOffice)

	// Caller is placed in officeB — so their scope is {officeB}, excluding officeA.
	var excludedUserID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"excluded-caller", "excluded-caller@test.local", restrictedRoleID, officeB).Scan(&excludedUserID))

	svc := asset.NewService(q, pool, minioStore, 5<<20)
	fieldSvc := authz.NewFieldService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	audSvc := audit.NewService(q)
	h := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := router.Group("/api/v1")

	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, excludedUserID.String())
		c.Set(middleware.CtxRoleID, restrictedRoleID.String())
		c.Next()
	}
	passThrough := func(c *gin.Context) { c.Next() }
	asset.RegisterRoutes(rg, h, stubAuth, passThrough, passThrough)

	do := func(method, path string, body io.Reader, ct string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, body)
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	base := "/api/v1/assets/" + assetID.String() + "/attachments"
	fakeAID := uuid.New().String()

	// Upload → 403.
	uploadBody, uploadCT := attBuildMultipart(t, "photo.png", "image/png", attMakePNG())
	w := do(http.MethodPost, base, uploadBody, uploadCT)
	assert.Equal(t, http.StatusForbidden, w.Code,
		"upload from out-of-scope caller → 403; body: %s", w.Body.String())

	// List → 403.
	w = do(http.MethodGet, base, nil, "")
	assert.Equal(t, http.StatusForbidden, w.Code,
		"list from out-of-scope caller → 403")

	// Content download → 403.
	w = do(http.MethodGet, base+"/"+fakeAID+"/content", nil, "")
	assert.Equal(t, http.StatusForbidden, w.Code,
		"content download from out-of-scope caller → 403")

	// Thumbnail → 403.
	w = do(http.MethodGet, base+"/"+fakeAID+"/thumbnail", nil, "")
	assert.Equal(t, http.StatusForbidden, w.Code,
		"thumbnail from out-of-scope caller → 403")

	// Delete → 403.
	w = do(http.MethodDelete, base+"/"+fakeAID, nil, "")
	assert.Equal(t, http.StatusForbidden, w.Code,
		"delete from out-of-scope caller → 403")
}

// TestAttachment_DBRollback drives UploadAttachment with a non-existent asset_id
// (causing a FK violation on CreateAttachment) and verifies that the best-effort
// rollback leaves NO orphaned objects in storage.
//
// It uses a storage.Fake (in-memory) paired with a real Postgres testcontainer so
// that the FK violation is authentic while ObjsKeys() lets us directly assert that
// every object the service uploaded was removed by the rollback path.
//
// Two sub-cases:
//  1. PDF (no thumbnail) — one object Put, one Remove expected.
//  2. PNG (image, thumbnail generated) — two objects Put (original + thumbnail),
//     both must be removed.
func TestAttachment_DBRollback(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)

	nonExistentAssetID := uuid.New()
	callerID := uuid.New()

	// ── sub-case 1: PDF — document, no thumbnail ──────────────────────────────
	t.Run("pdf_no_orphan", func(t *testing.T) {
		fake := storage.NewFake()
		svc := asset.NewService(q, pool, fake, 5<<20)

		_, uploadErr := svc.UploadAttachment(ctx, asset.UploadInput{
			AssetID:     nonExistentAssetID,
			Filename:    "x.pdf",
			ContentType: "application/pdf",
			Data:        []byte("%PDF-1.4 test"),
			CreatedBy:   callerID,
		})

		// Must return an error (FK violation → ErrInvalidRef).
		require.Error(t, uploadErr, "upload to non-existent asset_id must fail")
		assert.ErrorIs(t, uploadErr, asset.ErrInvalidRef,
			"FK violation must map to ErrInvalidRef")

		// No objects must remain in the Fake store (original removed on rollback).
		assert.Len(t, fake.ObjsKeys(), 0,
			"rollback must remove the uploaded PDF object; orphaned keys: %v", fake.ObjsKeys())

		// DB side: no attachment row created.
		rows, dbErr := q.ListAttachments(ctx, nonExistentAssetID)
		require.NoError(t, dbErr)
		assert.Empty(t, rows, "no attachment row must exist after DB-rollback failure")
	})

	// ── sub-case 2: PNG — image, thumbnail is generated then both must be removed ─
	t.Run("image_no_orphan", func(t *testing.T) {
		fake := storage.NewFake()
		svc := asset.NewService(q, pool, fake, 5<<20)

		_, uploadErr := svc.UploadAttachment(ctx, asset.UploadInput{
			AssetID:     nonExistentAssetID,
			Filename:    "photo.png",
			ContentType: "image/png",
			Data:        attMakePNG(),
			CreatedBy:   callerID,
		})

		// Must return an error (FK violation → ErrInvalidRef).
		require.Error(t, uploadErr, "upload to non-existent asset_id must fail")
		assert.ErrorIs(t, uploadErr, asset.ErrInvalidRef,
			"FK violation must map to ErrInvalidRef")

		// Both the original and thumbnail objects must be removed (no orphans).
		assert.Len(t, fake.ObjsKeys(), 0,
			"rollback must remove BOTH the original and thumbnail objects; orphaned keys: %v", fake.ObjsKeys())

		// DB side: no attachment row created.
		rows, dbErr := q.ListAttachments(ctx, nonExistentAssetID)
		require.NoError(t, dbErr)
		assert.Empty(t, rows, "no attachment row must exist after DB-rollback failure")
	})
}

// TestAttachment_CrossAssetRejected verifies the ownership guard: an attachment that
// belongs to asset B cannot be accessed (download, delete) via asset A's URL path,
// even when the caller has scope over the office that contains both assets.
//
// The guard lives in streamAttachment and deleteAttachment:
//
//	if att.AssetID != assetID { c.JSON(404, ...) }
//
// This test seeds two assets in the SAME office so that scope is never the rejection
// reason — only the cross-asset mismatch causes the 404.
func TestAttachment_CrossAssetRejected(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	minioStore := testsupport.NewMinIO(t)

	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)

	// Both assets live in the same office so the caller's global scope covers both.
	officeID := seedOfficeWithType(t, pool, "CrossAssetOfficeType", "CRSA")
	catID := seedCategory(t, pool, "CRS")

	assetA := seedAssetDirect(t, pool, "CRSA-CRS-2026-00001", "Cross Asset A", catID, officeID)
	assetB := seedAssetDirect(t, pool, "CRSA-CRS-2026-00002", "Cross Asset B", catID, officeID)

	// Superadmin — global scope, so both assets are in-scope.
	roleID := lookupRole(t, pool, "Superadmin")

	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"cross-asset-caller", "cross-asset-caller@test.local", roleID, officeID).Scan(&userID))

	svc := asset.NewService(q, pool, minioStore, 5<<20)
	fieldSvc := authz.NewFieldService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	audSvc := audit.NewService(q)
	h := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := router.Group("/api/v1")

	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	passThrough := func(c *gin.Context) { c.Next() }
	asset.RegisterRoutes(rg, h, stubAuth, passThrough, passThrough)

	do := func(method, path string, body io.Reader, ct string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, body)
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	// Upload an attachment to asset B — this must succeed (201).
	baseB := "/api/v1/assets/" + assetB.String() + "/attachments"
	uploadBody, uploadCT := attBuildMultipart(t, "photo.png", "image/png", attMakePNG())
	w := do(http.MethodPost, baseB, uploadBody, uploadCT)
	require.Equal(t, http.StatusCreated, w.Code,
		"pre-condition: upload to asset B must succeed; body: %s", w.Body.String())

	var uploadResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uploadResp))
	aidB, ok := uploadResp["id"].(string)
	require.True(t, ok, "upload response must contain string id")

	// ── Cross-asset GET /content via asset A's path ───────────────────────────
	// att.AssetID == assetB but the URL uses assetA → ownership guard fires → 404.
	contentPath := "/api/v1/assets/" + assetA.String() + "/attachments/" + aidB + "/content"
	w = do(http.MethodGet, contentPath, nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code,
		"GET content with mismatched asset path → 404; body: %s", w.Body.String())

	// ── Cross-asset DELETE via asset A's path ─────────────────────────────────
	deletePath := "/api/v1/assets/" + assetA.String() + "/attachments/" + aidB
	w = do(http.MethodDelete, deletePath, nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code,
		"DELETE with mismatched asset path → 404; body: %s", w.Body.String())

	// Verify the attachment on asset B is still intact (the misrouted DELETE must not have deleted it).
	w = do(http.MethodGet, baseB, nil, "")
	require.Equal(t, http.StatusOK, w.Code)
	var listResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	assert.Equal(t, float64(1), listResp["total"],
		"attachment on asset B must still exist after rejected cross-asset DELETE")
}
