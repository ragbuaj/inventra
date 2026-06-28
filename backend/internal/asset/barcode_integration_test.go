//go:build integration

package asset_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"net/http/httptest"
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

// ─── barcode harness ─────────────────────────────────────────────────────────

// barcodeHarness wires up Postgres + a gin router for barcode/label integration
// tests. The Superadmin role (global scope) is the default caller; tests that
// need a restricted caller build their own router inline.
type barcodeHarness struct {
	router   *gin.Engine
	assetID  uuid.UUID
	assetTag string
	officeID uuid.UUID
	userID   uuid.UUID
	roleID   uuid.UUID
}

// barcodeNewHarness seeds one office + category + asset, wires a gin router
// with stub auth (Superadmin / global scope), and returns the harness.
func barcodeNewHarness(t *testing.T) *barcodeHarness {
	t.Helper()
	ctx := context.Background()

	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)

	officeID := seedOfficeWithType(t, pool, "BarcodeOfficeType", "BCR01")
	catID := seedCategory(t, pool, "BCR")
	const tag = "BCR01-BCR-2026-00001"
	assetID := seedAssetDirect(t, pool, tag, "Barcode Test Asset", catID, officeID)

	roleID := lookupRole(t, pool, "Superadmin")

	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"barcode-caller", "barcode-caller@test.local", roleID, officeID).Scan(&userID))

	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")
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

	return &barcodeHarness{
		router:   router,
		assetID:  assetID,
		assetTag: tag,
		officeID: officeID,
		userID:   userID,
		roleID:   roleID,
	}
}

// do fires a request against the harness router and returns the recorder.
func (h *barcodeHarness) do(t *testing.T, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// jsonBody marshals v as compact JSON and returns a *bytes.Reader.
func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

// ─── scan lookup: GET /assets/by-tag/:tag ────────────────────────────────────

// TestByTag_HappyPath verifies that a known tag returns 200 with matching asset_tag.
func TestByTag_HappyPath(t *testing.T) {
	h := barcodeNewHarness(t)
	w := h.do(t, http.MethodGet, "/api/v1/assets/by-tag/"+h.assetTag, nil, "")
	require.Equal(t, http.StatusOK, w.Code, "known tag → 200; body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, h.assetTag, resp["asset_tag"], "asset_tag in body must match the seeded tag")
}

// TestByTag_UnknownTag verifies that an unknown tag returns 404.
func TestByTag_UnknownTag(t *testing.T) {
	h := barcodeNewHarness(t)
	w := h.do(t, http.MethodGet, "/api/v1/assets/by-tag/XXXX-NOTEXIST-2026-99999", nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code, "unknown tag → 404")
}

// TestByTag_OutOfScope verifies that a caller whose scope excludes the asset's
// office gets 404 (not 403, to avoid tag enumeration).
func TestByTag_OutOfScope(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)

	// Asset lives in officeA.
	officeA := seedOfficeWithType(t, pool, "ScanScopeType", "SCAN01")
	var officeTypeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT office_type_id FROM masterdata.offices WHERE id = $1`, officeA).Scan(&officeTypeID))
	var officeB uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Office B', 'SCAN02') RETURNING id`, officeTypeID).Scan(&officeB))

	catID := seedCategory(t, pool, "SCN")
	const tag = "SCAN01-SCN-2026-00001"
	seedAssetDirect(t, pool, tag, "Scan Scoped Asset", catID, officeA)

	// Restricted role: office-level scope — caller sees only their own office.
	restrictedRoleID := testsupport.SeedRole(t, pool, "ScanRestrictedRole")
	testsupport.SeedScopePolicy(t, pool, restrictedRoleID, "*", sqlc.SharedScopeLevelOffice)

	// Caller is placed in officeB — so their scope is {officeB}, excluding officeA.
	var callerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"scan-excluded", "scan-excluded@test.local", restrictedRoleID, officeB).Scan(&callerID))

	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	fieldSvc := authz.NewFieldService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	audSvc := audit.NewService(q)
	h := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := router.Group("/api/v1")
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, callerID.String())
		c.Set(middleware.CtxRoleID, restrictedRoleID.String())
		c.Next()
	}
	passThrough := func(c *gin.Context) { c.Next() }
	asset.RegisterRoutes(rg, h, stubAuth, passThrough, passThrough)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets/by-tag/"+tag, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code,
		"out-of-scope tag lookup → 404 (not 403, avoids enumeration); body: %s", w.Body.String())
}

// ─── barcode PNG: GET /assets/:id/barcode ────────────────────────────────────

// TestBarcode_Code128 verifies that the default (no ?type param) returns a valid PNG.
func TestBarcode_Code128(t *testing.T) {
	h := barcodeNewHarness(t)
	path := "/api/v1/assets/" + h.assetID.String() + "/barcode"
	w := h.do(t, http.MethodGet, path, nil, "")
	require.Equal(t, http.StatusOK, w.Code, "barcode code128 → 200; body len: %d", w.Body.Len())
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"),
		"Content-Type must be image/png")
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"),
		"must set X-Content-Type-Options: nosniff")

	_, fmt, err := image.Decode(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err, "response body must decode as a valid image")
	assert.Equal(t, "png", fmt, "image format must be png")
}

// TestBarcode_QR verifies that ?type=qr returns a valid PNG.
func TestBarcode_QR(t *testing.T) {
	h := barcodeNewHarness(t)
	path := "/api/v1/assets/" + h.assetID.String() + "/barcode?type=qr"
	w := h.do(t, http.MethodGet, path, nil, "")
	require.Equal(t, http.StatusOK, w.Code, "barcode qr → 200; body len: %d", w.Body.Len())
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))

	_, fmt, err := image.Decode(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err, "QR response body must decode as a valid image")
	assert.Equal(t, "png", fmt)
}

// TestBarcode_BadType verifies that ?type=bad returns 400.
func TestBarcode_BadType(t *testing.T) {
	h := barcodeNewHarness(t)
	path := "/api/v1/assets/" + h.assetID.String() + "/barcode?type=bad"
	w := h.do(t, http.MethodGet, path, nil, "")
	assert.Equal(t, http.StatusBadRequest, w.Code, "invalid barcode type → 400; body: %s", w.Body.String())
}

// TestBarcode_NotFound verifies that a non-existent asset ID returns 404.
func TestBarcode_NotFound(t *testing.T) {
	h := barcodeNewHarness(t)
	path := "/api/v1/assets/" + uuid.New().String() + "/barcode"
	w := h.do(t, http.MethodGet, path, nil, "")
	assert.Equal(t, http.StatusNotFound, w.Code, "non-existent asset → 404")
}

// TestBarcode_OutOfScope verifies that a caller whose scope excludes the asset's
// office gets 403 on the barcode endpoint.
func TestBarcode_OutOfScope(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)

	officeA := seedOfficeWithType(t, pool, "BarcodeScopeType", "BRSCP01")
	var officeTypeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT office_type_id FROM masterdata.offices WHERE id = $1`, officeA).Scan(&officeTypeID))
	var officeB uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Office B', 'BRSCP02') RETURNING id`, officeTypeID).Scan(&officeB))

	catID := seedCategory(t, pool, "BRS")
	assetID := seedAssetDirect(t, pool, "BRSCP01-BRS-2026-00001", "Barcode Scoped Asset", catID, officeA)

	restrictedRoleID := testsupport.SeedRole(t, pool, "BarcodeRestrictedRole")
	testsupport.SeedScopePolicy(t, pool, restrictedRoleID, "*", sqlc.SharedScopeLevelOffice)

	var callerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"barcode-excl", "barcode-excl@test.local", restrictedRoleID, officeB).Scan(&callerID))

	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	fieldSvc := authz.NewFieldService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	audSvc := audit.NewService(q)
	h := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := router.Group("/api/v1")
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, callerID.String())
		c.Set(middleware.CtxRoleID, restrictedRoleID.String())
		c.Next()
	}
	passThrough := func(c *gin.Context) { c.Next() }
	asset.RegisterRoutes(rg, h, stubAuth, passThrough, passThrough)

	path := "/api/v1/assets/" + assetID.String() + "/barcode"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code,
		"out-of-scope barcode → 403; body: %s", w.Body.String())
}

// ─── label PDF: POST /assets/labels ──────────────────────────────────────────

// TestLabel_BTNRoll verifies that a BTN roll label PDF is generated for a single
// seeded asset. Logo absent in test env → plain QR used; PDF must still render.
func TestLabel_BTNRoll(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"template":  "btn",
		"layout":    "roll",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	require.Equal(t, http.StatusOK, w.Code, "BTN roll label → 200; body: %s", w.Body.String())
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"),
		"Content-Type must be application/pdf")
	assert.True(t, bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")),
		"response body must start with %%PDF")
}

// TestLabel_GenericSheet verifies that a generic sheet label PDF is generated
// for a single asset resolved by tag, with mode=both.
func TestLabel_GenericSheet(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"tags":     []string{h.assetTag},
		"template": "generic",
		"layout":   "sheet",
		"mode":     "both",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	require.Equal(t, http.StatusOK, w.Code, "generic sheet label → 200; body: %s", w.Body.String())
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.True(t, bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")),
		"response body must start with %%PDF")
}

// TestLabel_QROnlyGeneric verifies that a generic roll label with mode=qr renders a valid PDF.
func TestLabel_QROnlyGeneric(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"template":  "generic",
		"layout":    "roll",
		"mode":      "qr",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	require.Equal(t, http.StatusOK, w.Code, "generic qr-only roll → 200; body: %s", w.Body.String())
	assert.True(t, bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")))
}

// TestLabel_WithNameAndOfficeFields verifies labels render when name + office fields are enabled.
func TestLabel_WithNameAndOfficeFields(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"template":  "generic",
		"layout":    "sheet",
		"mode":      "both",
		"fields": map[string]any{
			"name":   true,
			"office": true,
		},
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	require.Equal(t, http.StatusOK, w.Code, "label with name+office fields → 200; body: %s", w.Body.String())
	assert.True(t, bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")))
}

// ─── scope/validation scenarios ──────────────────────────────────────────────

// TestLabel_EmptySelection verifies that an empty asset_ids + tags body returns 400.
func TestLabel_EmptySelection(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{},
		"tags":      []string{},
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"empty asset_ids+tags → 400; body: %s", w.Body.String())
}

// TestLabel_InvalidTemplate verifies that an unrecognized template value returns 400.
func TestLabel_InvalidTemplate(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"template":  "invalid_tmpl",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"invalid template → 400; body: %s", w.Body.String())
}

// TestLabel_InvalidLayout verifies that an unrecognized layout value returns 400.
func TestLabel_InvalidLayout(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"layout":    "invalid_layout",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"invalid layout → 400; body: %s", w.Body.String())
}

// TestLabel_InvalidMode verifies that an unrecognized mode value returns 400.
func TestLabel_InvalidMode(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"mode":      "invalid_mode",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"invalid mode → 400; body: %s", w.Body.String())
}

// TestLabel_NonExistentAssetID verifies that a valid UUID that does not exist
// in the DB is mapped to 404.
func TestLabel_NonExistentAssetID(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{uuid.New().String()},
		"template":  "generic",
		"layout":    "roll",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	assert.Equal(t, http.StatusNotFound, w.Code,
		"non-existent asset id → 404; body: %s", w.Body.String())
}

// TestLabel_OutOfScopeAsset verifies that a caller whose data scope excludes the
// asset's office gets 403 when requesting labels.
func TestLabel_OutOfScopeAsset(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)

	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)

	// Seed asset in officeA.
	officeA := seedOfficeWithType(t, pool, "LabelScopeType", "LBLA01")
	var officeTypeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT office_type_id FROM masterdata.offices WHERE id = $1`, officeA).Scan(&officeTypeID))
	var officeB uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Label B', 'LBLA02') RETURNING id`, officeTypeID).Scan(&officeB))

	catID := seedCategory(t, pool, "LBL")
	assetID := seedAssetDirect(t, pool, "LBLA01-LBL-2026-00001", "Label Scoped Asset", catID, officeA)

	// Caller restricted to officeB only.
	restrictedRoleID := testsupport.SeedRole(t, pool, "LabelRestrictedRole")
	testsupport.SeedScopePolicy(t, pool, restrictedRoleID, "*", sqlc.SharedScopeLevelOffice)

	var callerID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		"label-excl", "label-excl@test.local", restrictedRoleID, officeB).Scan(&callerID))

	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	fieldSvc := authz.NewFieldService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	audSvc := audit.NewService(q)
	h := asset.NewHandler(svc, fieldSvc, scoped, audSvc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := router.Group("/api/v1")
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, callerID.String())
		c.Set(middleware.CtxRoleID, restrictedRoleID.String())
		c.Next()
	}
	passThrough := func(c *gin.Context) { c.Next() }
	asset.RegisterRoutes(rg, h, stubAuth, passThrough, passThrough)

	body, _ := json.Marshal(map[string]any{
		"asset_ids": []string{assetID.String()},
		"template":  "generic",
		"layout":    "roll",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets/labels", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code,
		"out-of-scope label request → 403; body: %s", w.Body.String())
}

// TestLabel_InvalidAssetIDFormat verifies that a malformed (non-UUID) asset id in
// asset_ids returns 400.
func TestLabel_InvalidAssetIDFormat(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{"not-a-uuid"},
		"template":  "generic",
		"layout":    "roll",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code,
		"malformed asset id → 400; body: %s", w.Body.String())
}

// TestLabel_CustomSizePreset verifies that a known size preset (60x24) is accepted
// and still produces a valid PDF.
func TestLabel_CustomSizePreset(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"template":  "btn",
		"layout":    "roll",
		"size":      "60x24",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	require.Equal(t, http.StatusOK, w.Code, "60x24 preset → 200; body: %s", w.Body.String())
	assert.True(t, bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")))
}

// TestLabel_UnknownSizePreset verifies that an unknown size preset returns a non-200
// error (400 or 422).
func TestLabel_UnknownSizePreset(t *testing.T) {
	h := barcodeNewHarness(t)
	body := jsonBody(t, map[string]any{
		"asset_ids": []string{h.assetID.String()},
		"size":      "999x999",
	})
	w := h.do(t, http.MethodPost, "/api/v1/assets/labels", body, "application/json")
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity,
		"unknown size preset → 400 or 422; got %d; body: %s", w.Code, w.Body.String())
}
