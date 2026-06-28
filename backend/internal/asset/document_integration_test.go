//go:build integration

package asset_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// ─── harness ─────────────────────────────────────────────────────────────────

// docHarness holds all wired-up resources for a document HTTP integration test.
type docHarness struct {
	router   *gin.Engine
	svc      *asset.Service
	store    *storage.MinIOStorage
	assetID  uuid.UUID
	officeID uuid.UUID
	userID   uuid.UUID
	roleID   uuid.UUID
}

// docNewHarness boots throwaway Postgres + MinIO testcontainers, seeds one office
// + category + asset, wires a gin router with stub auth (no JWT), and returns the
// harness.  maxBytes controls the service's upload size limit.
//
// The seeded caller has the migration-seeded "Superadmin" role (global scope),
// so all endpoints are in-scope by default.
func docNewHarness(t *testing.T, maxBytes int64) *docHarness {
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

	officeID := seedOfficeWithType(t, pool, "DocOfficeType", "DOC01")
	catID := seedCategory(t, pool, "DCT")
	assetID := seedAssetDirect(t, pool, "DOC01-DCT-2026-00001", "Document Test Asset", catID, officeID)

	// Superadmin has global scope — migration 000005 seeds both the role and its policy.
	roleID := lookupRole(t, pool, "Superadmin")

	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"doc-caller", "doc-caller@test.local", roleID, officeID).Scan(&userID))

	svc := asset.NewService(q, pool, minioStore, maxBytes, "")
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

	return &docHarness{
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
func (h *docHarness) do(t *testing.T, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// docPath returns the base documents path for the harness asset.
func (h *docHarness) docPath() string {
	return "/api/v1/assets/" + h.assetID.String() + "/documents"
}

// objectExists returns true when the given key exists in MinIO (GET succeeds).
func objectExists(t *testing.T, store *storage.MinIOStorage, key string) bool {
	t.Helper()
	rc, _, err := store.Get(context.Background(), key)
	if err == nil {
		_ = rc.Close()
		return true
	}
	if errors.Is(err, storage.ErrObjectNotFound) {
		return false
	}
	// Unexpected error — treat as "unknown" but still fail informatively.
	t.Logf("objectExists: unexpected error for key %q: %v", key, err)
	return false
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestDocument groups all 10 integration cases for asset documents.
func TestDocument(t *testing.T) {

	// ── Case 1: Create_MetadataOnly ───────────────────────────────────────────
	// POST /assets/:id/documents with only doc_type → 201, has_file=false,
	// no object_key key in response body.
	t.Run("Create_MetadataOnly", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		body := bytes.NewBufferString(`{"doc_type":"bast_acquisition","doc_no":"BAST-1"}`)
		w := h.do(t, http.MethodPost, base, body, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "create metadata-only doc → 201; body: %s", w.Body.String())

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, false, resp["has_file"], "metadata-only doc must have has_file=false")
		assert.NotContains(t, resp, "object_key", "object_key must never appear in response")
		docID, ok := resp["id"].(string)
		require.True(t, ok, "response must contain string id")
		assert.Equal(t, "bast_acquisition", resp["doc_type"])
		assert.Equal(t, h.assetID.String(), resp["asset_id"])
		_ = docID
	})

	// ── Case 2: AttachAndDownload_RoundTrip ───────────────────────────────────
	// Create doc; PUT .../file (PDF) → 200, has_file=true.
	// GET .../file → 200, bytes byte-identical, headers include
	// X-Content-Type-Options:nosniff and Content-Disposition.
	t.Run("AttachAndDownload_RoundTrip", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		// Create metadata-only doc.
		createBody := bytes.NewBufferString(`{"doc_type":"invoice","doc_no":"INV-001"}`)
		w := h.do(t, http.MethodPost, base, createBody, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "pre-condition: create doc → 201; body: %s", w.Body.String())
		var createResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
		docID := createResp["id"].(string)

		// Upload PDF file.
		pdfData := attMakePDF()
		fileBody, fileCT := attBuildMultipart(t, "invoice.pdf", "application/pdf", pdfData)
		w = h.do(t, http.MethodPut, base+"/"+docID+"/file", fileBody, fileCT)
		require.Equal(t, http.StatusOK, w.Code, "attach PDF → 200; body: %s", w.Body.String())

		var uploadResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &uploadResp))
		assert.Equal(t, true, uploadResp["has_file"], "after file upload has_file must be true")
		assert.NotContains(t, uploadResp, "object_key", "object_key must never appear in response")

		// Download the file.
		w = h.do(t, http.MethodGet, base+"/"+docID+"/file", nil, "")
		require.Equal(t, http.StatusOK, w.Code, "download file → 200; body: %s", w.Body.String())
		assert.Equal(t, pdfData, w.Body.Bytes(), "downloaded bytes must be byte-identical to uploaded PDF")
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"),
			"must set X-Content-Type-Options: nosniff")
		contentDisp := w.Header().Get("Content-Disposition")
		assert.NotEmpty(t, contentDisp, "must set Content-Disposition header")
		assert.Contains(t, contentDisp, "INV-001", "Content-Disposition must reference doc_no as filename base")
	})

	// ── Case 3: ReplaceFile_RemovesOld ────────────────────────────────────────
	// Attach PNG, then replace with PDF.
	// Old MinIO object must be gone; new object must exist.
	// Old/new keys differ by extension.
	t.Run("ReplaceFile_RemovesOld", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		// Create doc.
		createBody := bytes.NewBufferString(`{"doc_type":"contract","doc_no":"CTR-001"}`)
		w := h.do(t, http.MethodPost, base, createBody, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "pre-condition: create doc → 201; body: %s", w.Body.String())
		var createResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
		docID := createResp["id"].(string)

		// First upload: PNG.
		pngData := attMakePNG()
		pngBody, pngCT := attBuildMultipart(t, "contract.png", "image/png", pngData)
		w = h.do(t, http.MethodPut, base+"/"+docID+"/file", pngBody, pngCT)
		require.Equal(t, http.StatusOK, w.Code, "first upload PNG → 200; body: %s", w.Body.String())

		// Derive the old key: assets/<assetID>/documents/<docID>.png
		oldKey := fmt.Sprintf("assets/%s/documents/%s.png", h.assetID, docID)
		require.True(t, objectExists(t, h.store, oldKey), "old PNG object must exist after first upload")

		// Second upload: PDF (replaces PNG).
		pdfData := attMakePDF()
		pdfBody, pdfCT := attBuildMultipart(t, "contract.pdf", "application/pdf", pdfData)
		w = h.do(t, http.MethodPut, base+"/"+docID+"/file", pdfBody, pdfCT)
		require.Equal(t, http.StatusOK, w.Code, "replace with PDF → 200; body: %s", w.Body.String())

		// Derive the new key.
		newKey := fmt.Sprintf("assets/%s/documents/%s.pdf", h.assetID, docID)

		// Old PNG object must be gone; new PDF object must exist.
		assert.False(t, objectExists(t, h.store, oldKey),
			"old PNG object must be removed after file replacement")
		assert.True(t, objectExists(t, h.store, newKey),
			"new PDF object must exist after file replacement")

		// Keys must differ (different extensions).
		assert.NotEqual(t, oldKey, newKey, "old and new object keys must differ by extension")

		// GET .../file should return PDF content.
		w = h.do(t, http.MethodGet, base+"/"+docID+"/file", nil, "")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, pdfData, w.Body.Bytes(), "download after replacement must return new PDF bytes")
	})

	// ── Case 4: ListAndGet ────────────────────────────────────────────────────
	// Create two docs → list returns total=2, newest-first; GET one by id ok.
	t.Run("ListAndGet", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		// Create first doc.
		body1 := bytes.NewBufferString(`{"doc_type":"bast_acquisition","doc_no":"BAST-A"}`)
		w := h.do(t, http.MethodPost, base, body1, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "create doc 1 → 201; body: %s", w.Body.String())
		var r1 map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &r1))
		docID1 := r1["id"].(string)

		// Small sleep to ensure different created_at timestamps for ordering.
		time.Sleep(10 * time.Millisecond)

		// Create second doc.
		body2 := bytes.NewBufferString(`{"doc_type":"invoice","doc_no":"INV-B"}`)
		w = h.do(t, http.MethodPost, base, body2, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "create doc 2 → 201; body: %s", w.Body.String())
		var r2 map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &r2))
		docID2 := r2["id"].(string)

		// List → total=2.
		w = h.do(t, http.MethodGet, base, nil, "")
		require.Equal(t, http.StatusOK, w.Code, "list → 200; body: %s", w.Body.String())
		var listResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
		assert.Equal(t, float64(2), listResp["total"], "list total must be 2")
		items, _ := listResp["data"].([]any)
		require.Len(t, items, 2, "list data must have 2 items")

		// Newest-first: doc2 should appear before doc1.
		first := items[0].(map[string]any)
		second := items[1].(map[string]any)
		assert.Equal(t, docID2, first["id"], "newest doc must be first in list")
		assert.Equal(t, docID1, second["id"], "older doc must be second in list")

		// GET one by id.
		w = h.do(t, http.MethodGet, base+"/"+docID1, nil, "")
		require.Equal(t, http.StatusOK, w.Code, "GET doc by id → 200; body: %s", w.Body.String())
		var getResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &getResp))
		assert.Equal(t, docID1, getResp["id"])
		assert.Equal(t, "bast_acquisition", getResp["doc_type"])
	})

	// ── Case 5: WrongAssetDoc_404 ─────────────────────────────────────────────
	// Doc belongs to asset A; GET /assets/{B}/documents/{docA} → 404.
	t.Run("WrongAssetDoc_404", func(t *testing.T) {
		ctx := context.Background()
		pool := testsupport.NewPostgres(t)
		rdb := testsupport.NewRedis(t)
		minioStore := testsupport.NewMinIO(t)

		_, err := pool.Exec(ctx,
			`TRUNCATE approval.request_approvals, approval.requests,
			 asset.asset_tag_counters, asset.assets CASCADE`)
		require.NoError(t, err)

		q := sqlc.New(pool)
		officeID := seedOfficeWithType(t, pool, "CrossDocOfficeType", "CDOC")
		catID := seedCategory(t, pool, "CDX")

		// Two different assets in the same office.
		assetA := seedAssetDirect(t, pool, "CDOC-CDX-2026-00001", "Asset A", catID, officeID)
		assetB := seedAssetDirect(t, pool, "CDOC-CDX-2026-00002", "Asset B", catID, officeID)

		roleID := lookupRole(t, pool, "Superadmin")
		var userID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO identity.users (name, email, role_id, office_id, status)
			 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
			"cross-doc-caller", "cross-doc-caller@test.local", roleID, officeID).Scan(&userID))

		svc := asset.NewService(q, pool, minioStore, 5<<20, "")
		fieldSvc := authz.NewFieldService(q, rdb)
		scopeSvc := authz.NewScopeService(q, rdb)
		scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
		audSvc := audit.NewService(q)
		handler := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		rg := router.Group("/api/v1")
		stubAuth := func(c *gin.Context) {
			c.Set(middleware.CtxUserID, userID.String())
			c.Set(middleware.CtxRoleID, roleID.String())
			c.Next()
		}
		passThrough := func(c *gin.Context) { c.Next() }
		asset.RegisterRoutes(rg, handler, stubAuth, passThrough, passThrough)

		do := func(method, path string, body io.Reader, ct string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(method, path, body)
			if ct != "" {
				req.Header.Set("Content-Type", ct)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			return w
		}

		// Create a doc under asset A.
		baseA := "/api/v1/assets/" + assetA.String() + "/documents"
		createBody := bytes.NewBufferString(`{"doc_type":"bast_acquisition","doc_no":"BAST-A1"}`)
		w := do(http.MethodPost, baseA, createBody, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "pre-condition: create doc under A → 201; body: %s", w.Body.String())
		var createResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
		docA := createResp["id"].(string)

		// Access docA via asset B's path → 404.
		baseB := "/api/v1/assets/" + assetB.String() + "/documents"
		w = do(http.MethodGet, baseB+"/"+docA, nil, "")
		assert.Equal(t, http.StatusNotFound, w.Code,
			"GET doc from wrong asset → 404; body: %s", w.Body.String())
	})

	// ── Case 6: Scope_Forbidden ───────────────────────────────────────────────
	// Caller whose office scope excludes the asset's office gets 403 on
	// POST (create), GET (list), and GET .../file.
	t.Run("Scope_Forbidden", func(t *testing.T) {
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
		officeA := seedOfficeWithType(t, pool, "DocScopeOfficeType", "DSCA")
		var officeTypeID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT office_type_id FROM masterdata.offices WHERE id = $1`, officeA).Scan(&officeTypeID))
		var officeB uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
			 VALUES (NULL, $1, 'Doc Office B', 'DSCB') RETURNING id`, officeTypeID).Scan(&officeB))

		// Asset lives in officeA.
		catID := seedCategory(t, pool, "DSC")
		assetID := seedAssetDirect(t, pool, "DSCA-DSC-2026-00001", "Scoped Doc Asset", catID, officeA)

		// Create a doc on this asset using a superadmin first, to test file download scope.
		superRoleID := lookupRole(t, pool, "Superadmin")
		var superUserID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO identity.users (name, email, role_id, office_id, status)
			 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
			"super-doc-scope", "super-doc-scope@test.local", superRoleID, officeA).Scan(&superUserID))

		// Restricted role: office-level scope (only the caller's own office).
		restrictedRoleID := testsupport.SeedRole(t, pool, "DocScopeEnfRole")
		testsupport.SeedScopePolicy(t, pool, restrictedRoleID, "*", sqlc.SharedScopeLevelOffice)

		// Caller is placed in officeB — scope is {officeB}, excluding officeA.
		var excludedUserID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO identity.users (name, email, role_id, office_id, status)
			 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
			"excluded-doc-caller", "excluded-doc-caller@test.local", restrictedRoleID, officeB).Scan(&excludedUserID))

		svc := asset.NewService(q, pool, minioStore, 5<<20, "")
		fieldSvc := authz.NewFieldService(q, rdb)
		scopeSvc := authz.NewScopeService(q, rdb)
		scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
		audSvc := audit.NewService(q)
		handler := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		rg := router.Group("/api/v1")

		// Use excluded caller for scope enforcement tests.
		stubAuth := func(c *gin.Context) {
			c.Set(middleware.CtxUserID, excludedUserID.String())
			c.Set(middleware.CtxRoleID, restrictedRoleID.String())
			c.Next()
		}
		passThrough := func(c *gin.Context) { c.Next() }
		asset.RegisterRoutes(rg, handler, stubAuth, passThrough, passThrough)

		do := func(method, path string, body io.Reader, ct string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(method, path, body)
			if ct != "" {
				req.Header.Set("Content-Type", ct)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			return w
		}

		basePath := "/api/v1/assets/" + assetID.String() + "/documents"
		fakeDocID := uuid.New().String()

		// POST (create) → 403.
		createBody := bytes.NewBufferString(`{"doc_type":"invoice"}`)
		w := do(http.MethodPost, basePath, createBody, "application/json")
		assert.Equal(t, http.StatusForbidden, w.Code,
			"create from out-of-scope caller → 403; body: %s", w.Body.String())

		// GET list → 403.
		w = do(http.MethodGet, basePath, nil, "")
		assert.Equal(t, http.StatusForbidden, w.Code,
			"list from out-of-scope caller → 403; body: %s", w.Body.String())

		// GET .../file → 403.
		w = do(http.MethodGet, basePath+"/"+fakeDocID+"/file", nil, "")
		assert.Equal(t, http.StatusForbidden, w.Code,
			"file download from out-of-scope caller → 403; body: %s", w.Body.String())
	})

	// ── Case 7: Delete_RemovesObject ─────────────────────────────────────────
	// Attach file, DELETE doc → 204; row soft-deleted (GET → 404) and MinIO
	// object removed (no orphan).
	t.Run("Delete_RemovesObject", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		// Create doc.
		createBody := bytes.NewBufferString(`{"doc_type":"bast_disposal","doc_no":"DISP-001"}`)
		w := h.do(t, http.MethodPost, base, createBody, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "create doc → 201; body: %s", w.Body.String())
		var createResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
		docID := createResp["id"].(string)

		// Upload PDF file.
		pdfData := attMakePDF()
		fileBody, fileCT := attBuildMultipart(t, "disposal.pdf", "application/pdf", pdfData)
		w = h.do(t, http.MethodPut, base+"/"+docID+"/file", fileBody, fileCT)
		require.Equal(t, http.StatusOK, w.Code, "attach PDF → 200; body: %s", w.Body.String())

		// Confirm object exists in MinIO.
		expectedKey := fmt.Sprintf("assets/%s/documents/%s.pdf", h.assetID, docID)
		require.True(t, objectExists(t, h.store, expectedKey), "PDF object must exist before delete")

		// DELETE → 204.
		w = h.do(t, http.MethodDelete, base+"/"+docID, nil, "")
		require.Equal(t, http.StatusNoContent, w.Code, "delete doc → 204; body: %s", w.Body.String())

		// GET → 404 (soft-deleted row excluded).
		w = h.do(t, http.MethodGet, base+"/"+docID, nil, "")
		assert.Equal(t, http.StatusNotFound, w.Code, "GET after delete → 404")

		// MinIO object must be gone (no orphan).
		assert.False(t, objectExists(t, h.store, expectedKey),
			"MinIO object must be removed after doc delete; key: %s", expectedKey)
	})

	// ── Case 8: InvalidRelatedRequest_400 ────────────────────────────────────
	// POST with related_request_id = random UUID (no such request) → 400.
	t.Run("InvalidRelatedRequest_400", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		randomReqID := uuid.New().String()
		bodyStr := fmt.Sprintf(`{"doc_type":"bast_acquisition","related_request_id":"%s"}`, randomReqID)
		w := h.do(t, http.MethodPost, base, bytes.NewBufferString(bodyStr), "application/json")
		assert.Equal(t, http.StatusBadRequest, w.Code,
			"invalid related_request_id → 400; body: %s", w.Body.String())
	})

	// ── Case 9: UploadOversize_413 and UploadDisallowedType_415 ──────────────
	// PUT .../file with >maxBytes body → 413.
	// PUT .../file with .zip part → 415.
	t.Run("UploadOversize_413", func(t *testing.T) {
		const maxBytes = 200
		// Large data: 500 bytes of fake PDF prefix — total multipart body exceeds 201 bytes.
		largeData := bytes.Repeat([]byte("%PDF"), 125) // 500 bytes
		require.Greater(t, len(largeData), maxBytes, "pre-condition: data must exceed maxBytes")

		h := docNewHarness(t, maxBytes)
		base := h.docPath()

		// Create doc first.
		createBody := bytes.NewBufferString(`{"doc_type":"other"}`)
		w := h.do(t, http.MethodPost, base, createBody, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "pre-condition: create doc → 201; body: %s", w.Body.String())
		var createResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
		docID := createResp["id"].(string)

		fileBody, fileCT := attBuildMultipart(t, "big.pdf", "application/pdf", largeData)
		w = h.do(t, http.MethodPut, base+"/"+docID+"/file", fileBody, fileCT)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code,
			"oversize upload → 413; body: %s", w.Body.String())
	})

	t.Run("UploadDisallowedType_415", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		// Create doc.
		createBody := bytes.NewBufferString(`{"doc_type":"other"}`)
		w := h.do(t, http.MethodPost, base, createBody, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "pre-condition: create doc → 201; body: %s", w.Body.String())
		var createResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
		docID := createResp["id"].(string)

		zipData := []byte("PK\x03\x04fake zip content")
		fileBody, fileCT := attBuildMultipart(t, "archive.zip", "application/zip", zipData)
		w = h.do(t, http.MethodPut, base+"/"+docID+"/file", fileBody, fileCT)
		assert.Equal(t, http.StatusUnsupportedMediaType, w.Code,
			"disallowed type → 415; body: %s", w.Body.String())
	})

	// ── Case 10: DownloadNoFile_404 ───────────────────────────────────────────
	// Create metadata-only doc, GET .../file → 404 (no file attached).
	t.Run("DownloadNoFile_404", func(t *testing.T) {
		h := docNewHarness(t, 5<<20)
		base := h.docPath()

		// Create metadata-only doc (no file upload).
		createBody := bytes.NewBufferString(`{"doc_type":"bast_transfer","doc_no":"BAST-T1"}`)
		w := h.do(t, http.MethodPost, base, createBody, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "create metadata-only doc → 201; body: %s", w.Body.String())
		var createResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
		docID := createResp["id"].(string)

		// Confirm has_file=false.
		assert.Equal(t, false, createResp["has_file"], "metadata-only doc must have has_file=false")

		// GET .../file → 404 (no file attached).
		w = h.do(t, http.MethodGet, base+"/"+docID+"/file", nil, "")
		assert.Equal(t, http.StatusNotFound, w.Code,
			"download with no file attached → 404; body: %s", w.Body.String())
	})
}
