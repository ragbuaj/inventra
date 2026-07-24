//go:build integration

// Integration tests for the bulk-import module: the full validate → confirm
// → execute pipeline against a real Postgres + Redis, driving the REAL
// target importers (asset, employee) exactly as internal/server/router.go
// wires them in production.
//
// This file is an EXTERNAL test package (importer_test, not importer) on
// purpose: the asset/employee/office packages implement importer.TargetImporter
// and therefore import internal/importer — a same-package test that also
// imported those packages would create an import cycle. worker.go exposes an
// exported Tick (added alongside this file) so each test can drive one
// validate/execute pass deterministically instead of waiting on the worker's
// polling loop.
package importer_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/importer"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/employee"
	"github.com/ragbuaj/inventra/internal/masterdata/office"
	"github.com/ragbuaj/inventra/internal/masterdata/reference"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/storage"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── harness ────────────────────────────────────────────────────────────────

// assetHeader / employeeHeader are the column orders used to build test CSVs;
// Parse matches columns by name (case-insensitive, order-insensitive), so the
// order here is cosmetic but kept aligned with asset/importer.go and
// masterdata/employee/importer.go's Columns() for readability.
var assetHeader = []string{"asset_tag", "nama", "kategori", "kantor", "tgl_beli", "harga", "vendor", "lokasi"}
var employeeHeader = []string{"kode", "nama", "email", "telepon", "kantor", "status", "departemen", "jabatan"}
var brandHeader = []string{"nama"}
var unitHeader = []string{"nama", "simbol"}
var modelHeader = []string{"merek", "nama"}

// harness bundles every service the bulk-import pipeline needs, wired exactly
// like internal/server/router.go wires them in production (NewRouter): one
// importer.Service with the asset/employee/office targets plus all five
// reference targets (provinces/cities/brands/units/models) registered, one
// approval.Service with the asset_import executor registered, and the HTTP
// handler/worker built on top of both.
type harness struct {
	pool        *pgxpool.Pool
	rdb         *redis.Client
	q           *sqlc.Queries
	permSvc     *authz.PermissionService
	approvalSvc *approval.Service
	importSvc   *importer.Service
	handler     *importer.Handler
	worker      *importer.Worker
	store       *storage.Fake
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	q := sqlc.New(pool)

	scopeSvc := authz.NewScopeService(q, rdb)
	permSvc := authz.NewPermissionService(q, rdb)
	approvalSvc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	empSvc := employee.NewService(q)
	officeSvc := office.NewService(q)
	approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetImport, assetSvc.ImportExecutor())

	refSvc := reference.NewService(q)

	store := storage.NewFake()
	importSvc := importer.NewService(q, pool, store, rdb, 1000, 10<<20)
	importSvc.RegisterTarget(assetSvc.Importer())
	importSvc.RegisterTarget(empSvc.Importer())
	importSvc.RegisterTarget(officeSvc.Importer())
	importSvc.RegisterTarget(reference.NewImporter(refSvc, "provinces"))
	importSvc.RegisterTarget(reference.NewImporter(refSvc, "cities"))
	importSvc.RegisterTarget(reference.NewImporter(refSvc, "brands"))
	importSvc.RegisterTarget(reference.NewImporter(refSvc, "models"))
	importSvc.RegisterTarget(reference.NewImporter(refSvc, "units"))

	auditSvc := audit.NewService(q)
	handler := importer.NewHandler(importSvc, permSvc, common.ScopedDeps{Q: q, Scope: scopeSvc}, auditSvc)
	// poll is irrelevant here — every test drives the worker via the exported
	// Tick instead of Run's polling loop.
	worker := importer.NewWorker(importSvc, pool, rdb, approvalSvc, scopeSvc, time.Hour)

	return &harness{
		pool: pool, rdb: rdb, q: q,
		permSvc: permSvc, approvalSvc: approvalSvc, importSvc: importSvc,
		handler: handler, worker: worker, store: store,
	}
}

// stubAuth injects the given user/role IDs into the Gin context in place of
// the real JWT middleware, mirroring internal/approval/integration_test.go's
// callGet pattern.
func stubAuth(userID, roleID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		// RequireAuth always sets the audience; RequireAudience now fails closed
		// on an empty one (ADR-0017 hardening), so the stub must set it to web.
		c.Set(middleware.CtxAudience, auth.AudienceWeb)
		c.Next()
	}
}

// newRouter builds a fresh Gin engine with the import routes mounted behind
// stubAuth for (userID, roleID), plus the production web-only audience gate
// (the importer is on ADR-0017's aud=mobile deny list).
func newRouter(h *harness, userID, roleID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	importer.RegisterRoutes(v1, h.handler, stubAuth(userID, roleID), middleware.RequireAudience(auth.AudienceWeb))
	return r
}

// ADR-0017 keputusan 4: /imports denies aud=mobile outright — even a caller
// whose role could import gets 403 before any handler logic runs.
func TestImports_MobileAudienceDenied(t *testing.T) {
	h := newHarness(t)
	userID, roleID := uuid.New(), uuid.New()

	mobileAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Set(middleware.CtxAudience, auth.AudienceMobile)
		c.Next()
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	importer.RegisterRoutes(v1, h.handler, mobileAuth, middleware.RequireAudience(auth.AudienceWeb))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/imports", nil))
	require.Equal(t, http.StatusForbidden, w.Code,
		"a mobile-audience caller must be denied on /imports: %s", w.Body.String())
}

// ─── seed helpers ───────────────────────────────────────────────────────────

// seedOfficeSimple inserts one office_type and one root office and returns
// the office's id. code is used both as the office code (matched by the
// importers' "kantor" lookup) and to derive unique names.
func seedOfficeSimple(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		"Tipe "+code).Scan(&typeID))
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`,
		typeID, "Kantor "+code, code).Scan(&id))
	return id
}

// TestImport_OfficeExecute_CreatesOfficeWithDefaultKind drives an office import
// end to end (validate → confirm → execute; the office target needs no approval)
// and asserts the row is actually created. This guards the class of bug where a
// NOT NULL column added to masterdata.offices (legacy-parity Fase 5's office_kind)
// is populated by the service but not by the importer's direct CreateOffice call —
// which fails the enum cast at execute time and was previously only caught by e2e.
func TestImport_OfficeExecute_CreatesOfficeWithDefaultKind(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// An office type the import's `tipe` column can resolve by name.
	var typeID uuid.UUID
	require.NoError(t, h.pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		"Tipe OFI").Scan(&typeID))

	roleID := seedGlobalMakerRole(t, h.pool, "masterdata.office.manage")
	makerID := seedUser(t, h.pool, roleID, nil, "maker.ofi@test.local")

	header := []string{"kode", "nama", "tipe", "induk", "aktif"}
	csvBytes := buildCSV(t, header, [][]string{
		{"OFI1", "Kantor Import Satu", "Tipe OFI", "", "true"},
	})

	job, err := h.importSvc.CreateJob(ctx, "office", "csv", "offices.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	// validate
	did, err := h.worker.Tick(ctx)
	require.NoError(t, err)
	assert.True(t, did)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)

	// confirm + execute (no approval for the office target)
	_, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	did, err = h.worker.Tick(ctx)
	require.NoError(t, err)
	assert.True(t, did)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 0, job.FailedRows)

	// The office row exists and carries the NOT NULL office_kind default.
	var name, kind string
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT name, office_kind::text FROM masterdata.offices WHERE code = $1 AND deleted_at IS NULL`,
		"OFI1").Scan(&name, &kind))
	assert.Equal(t, "Kantor Import Satu", name)
	assert.Equal(t, "konvensional", kind, "importer must supply the office_kind default")
}

// seedDepartment inserts a masterdata.departments row and returns its id.
func seedDepartment(t *testing.T, pool *pgxpool.Pool, name, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.departments (name, code) VALUES ($1, $2) RETURNING id`,
		name, code).Scan(&id))
	return id
}

// seedPosition inserts a masterdata.positions row and returns its id.
func seedPosition(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.positions (name) VALUES ($1) RETURNING id`,
		name).Scan(&id))
	return id
}

// seedBrand inserts a masterdata.brands row and returns its id.
func seedBrand(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.brands (name) VALUES ($1) RETURNING id`,
		name).Scan(&id))
	return id
}

// seedCategory inserts a masterdata.categories row and returns its id.
func seedCategory(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.categories (name, code, asset_class)
		 VALUES ($1, $2, 'tangible') RETURNING id`,
		code, code).Scan(&id))
	return id
}

// seedRoom inserts a floor + room under officeID and returns the room id. The
// asset importer's createRows always creates tangible assets (see
// internal/asset/importer.go), and asset.assets has chk_assets_tangible_location
// CHECK(asset_class = 'intangible' OR floor_id IS NOT NULL OR room_id IS NOT
// NULL) — the import template carries only a room (no floor), so every
// asset-import row that is meant to succeed needs a resolvable room in its
// target office.
func seedRoom(t *testing.T, pool *pgxpool.Pool, officeID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	floorID := testsupport.SeedFloor(t, pool, officeID, "Lantai "+name)
	return testsupport.SeedRoom(t, pool, floorID, name)
}

// grantPermission inserts an identity.role_permissions row for the role.
func grantPermission(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, key string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, $2)`,
		roleID, key)
	require.NoError(t, err)
}

// seedUser inserts an identity.users row (optionally placed in an office) and
// returns its id.
func seedUser(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, officeID *uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		email, email, roleID, officeID).Scan(&id))
	return id
}

// lookupRole queries identity.roles by name (migration-seeded roles) and
// returns its id.
func lookupRole(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM identity.roles WHERE name = $1 AND deleted_at IS NULL LIMIT 1`,
		name).Scan(&id))
	return id
}

// seedGlobalMakerRole seeds a fresh role with global data scope (module '*')
// and the given action permission granted — a maker/approver role that never
// has to fight scope rules, for tests whose focus is elsewhere.
func seedGlobalMakerRole(t *testing.T, pool *pgxpool.Pool, perm string) uuid.UUID {
	t.Helper()
	roleID := testsupport.SeedRole(t, pool, "maker-"+uuid.New().String()[:8])
	testsupport.SeedScopePolicy(t, pool, roleID, "*", sqlc.SharedScopeLevelGlobal)
	grantPermission(t, pool, roleID, perm)
	return roleID
}

// buildCSV renders a header + data rows into CSV bytes for upload.
func buildCSV(t *testing.T, header []string, rows [][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	require.NoError(t, w.Write(header))
	for _, r := range rows {
		require.NoError(t, w.Write(r))
	}
	w.Flush()
	require.NoError(t, w.Error())
	return buf.Bytes()
}

// multipartCSV builds a multipart/form-data body for POST /imports: a "file"
// field carrying csvBody as "batch.csv", plus a "target" field.
func multipartCSV(t *testing.T, target string, csvBody []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("target", target))
	fw, err := w.CreateFormFile("file", "batch.csv")
	require.NoError(t, err)
	_, err = fw.Write(csvBody)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

// rowsByName groups a job's rows by their "nama" cell (asset target) for
// assertions that need to find a specific row.
func rowsByName(t *testing.T, rows []sqlc.ImportImportRow) map[string]sqlc.ImportImportRow {
	t.Helper()
	out := map[string]sqlc.ImportImportRow{}
	for _, r := range rows {
		var data map[string]string
		require.NoError(t, json.Unmarshal(r.Data, &data))
		out[data["nama"]] = r
	}
	return out
}

// ─── 1. asset full cycle ────────────────────────────────────────────────────

// TestImport_AssetFullCycle_ApproveCreatesAssets drives a complete asset
// import: upload (one valid + one invalid row) → validate → confirm →
// execute (submits an asset_import approval request) → approve → the valid
// row's asset exists in the DB, the job is completed, and its result_ref
// carries the generated asset tag.
func TestImport_AssetFullCycle_ApproveCreatesAssets(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "AFC")
	seedCategory(t, h.pool, "AFC-CAT")
	seedRoom(t, h.pool, officeID, "Ruang AFC")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.afc@test.local")
	approverRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	approverID := seedUser(t, h.pool, approverRoleID, nil, "approver.afc@test.local")

	csvBytes := buildCSV(t, assetHeader, [][]string{
		{"", "Laptop Valid", "AFC-CAT", "AFC", "2026-01-05", "5000000", "", "Ruang AFC"},
		{"", "", "AFC-CAT", "AFC", "2026-01-05", "5000000", "", "Ruang AFC"}, // missing nama -> required
	})

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "batch.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusPending, job.Status)

	// --- validate ---
	did, err := h.worker.Tick(ctx)
	require.NoError(t, err)
	assert.True(t, did)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 2, job.TotalRows)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)
	require.NotNil(t, job.OfficeID)
	assert.Equal(t, officeID, *job.OfficeID)

	allRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: false, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, allRows, 2)
	byName := rowsByName(t, allRows)
	invalidRow := byName[""]
	assert.False(t, invalidRow.Valid)
	var invalidErrs []importer.CellError
	require.NoError(t, json.Unmarshal(invalidRow.Errors, &invalidErrs))
	require.Len(t, invalidErrs, 1)
	assert.Equal(t, "nama", invalidErrs[0].Column)
	assert.Equal(t, "required", invalidErrs[0].ErrorKey)
	assert.True(t, byName["Laptop Valid"].Valid)

	// --- confirm ---
	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusConfirmed, job.Status)

	// --- execute: submits the asset_import approval request ---
	did, err = h.worker.Tick(ctx)
	require.NoError(t, err)
	assert.True(t, did)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusAwaitingApproval, job.Status)
	require.NotNil(t, job.RequestID)

	reqRow, err := h.q.GetRequest(ctx, *job.RequestID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestTypeAssetImport, reqRow.Type)
	assert.Equal(t, sqlc.SharedRequestStatusPending, reqRow.Status)
	require.NotNil(t, reqRow.Amount)
	assert.Equal(t, "5000000.00", *reqRow.Amount, "sumHarga totals only the valid row's harga")

	// --- approve ---
	caller := approval.Caller{UserID: approverID, RoleID: approverRoleID, AllScope: true}
	decided, err := h.approvalSvc.Decide(ctx, *job.RequestID, caller, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, decided.Status)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)
	// The row that failed VALIDATION (missing nama) must still count as
	// failed on the completed job — only rows that passed validation are
	// ever executed, so "0 rows failed during execute" must not erase the
	// batch's original validation failure (see asset/executor.go's
	// assetImportExec.Execute, which now preserves job.FailedRows instead of
	// overwriting it with only execution-time failures).
	assert.EqualValues(t, 1, job.FailedRows)

	allRows, err = h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: false, Off: 0, Lim: 20})
	require.NoError(t, err)
	byName = rowsByName(t, allRows)
	validRow := byName["Laptop Valid"]
	require.NotNil(t, validRow.ResultRef, "the executor stamps the generated asset tag as result_ref")

	createdAsset, err := h.q.GetAssetByTag(ctx, *validRow.ResultRef)
	require.NoError(t, err)
	assert.Equal(t, "Laptop Valid", createdAsset.Name)
	assert.Equal(t, officeID, createdAsset.OfficeID)
	assert.Equal(t, sqlc.SharedAssetClassTangible, createdAsset.AssetClass)
	assert.Equal(t, "5000000.00", *createdAsset.PurchaseCost, "numeric column renders with 2 decimals")

	// Every create path must seed an initial location-history row (registration).
	hist, err := h.q.ListAssetLocationHistory(ctx, createdAsset.ID)
	require.NoError(t, err)
	require.Len(t, hist, 1, "imported asset must have exactly one registration location-history row")
	assert.Equal(t, sqlc.SharedLocationChangeSourceRegistration, hist[0].Source)
	require.NotNil(t, hist[0].RoomID)
	assert.Equal(t, *createdAsset.RoomID, *hist[0].RoomID)
}

// TestImport_AssetTangibleWithoutLocation_SkippedNotPoisoned drives an import
// whose valid row carries NO lokasi. It passes validation (lokasi is optional),
// but at create time a tangible asset with neither floor nor room would violate
// chk_assets_tangible_location (23514) and poison the shared approval commit.
// The createRows pre-check must instead skip it as a failed row so the batch
// completes cleanly and no asset is created.
func TestImport_AssetTangibleWithoutLocation_SkippedNotPoisoned(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "NLC")
	seedCategory(t, h.pool, "NLC-CAT")
	seedRoom(t, h.pool, officeID, "Ruang NLC")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.nlc@test.local")
	approverRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	approverID := seedUser(t, h.pool, approverRoleID, nil, "approver.nlc@test.local")

	// Keep the batch total (2 x 2,000,000 = 4,000,000) inside the single-step
	// approval band so one Decide finalizes the request and runs the executor.
	csvBytes := buildCSV(t, assetHeader, [][]string{
		{"", "Aset Berlokasi", "NLC-CAT", "NLC", "2026-01-05", "2000000", "", "Ruang NLC"},
		{"", "Aset Tanpa Lokasi", "NLC-CAT", "NLC", "2026-01-05", "2000000", "", ""}, // valid, but no room
	})

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "nolokasi.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	// validate: both rows pass (lokasi is optional at validation time).
	did, err := h.worker.Tick(ctx)
	require.NoError(t, err)
	assert.True(t, did)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 2, job.SuccessRows)
	assert.EqualValues(t, 0, job.FailedRows)

	// confirm + execute (submits approval).
	_, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	did, err = h.worker.Tick(ctx)
	require.NoError(t, err)
	assert.True(t, did)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	require.NotNil(t, job.RequestID)

	// approve: the executor creates the located row and skips the location-less
	// one as failed — the commit must NOT be poisoned by a 23514.
	caller := approval.Caller{UserID: approverID, RoleID: approverRoleID, AllScope: true}
	decided, err := h.approvalSvc.Decide(ctx, *job.RequestID, caller, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, decided.Status)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows, "only the located row is created")
	assert.EqualValues(t, 1, job.FailedRows, "the location-less row is skipped as failed")

	// The location-less asset must not exist; the located one must.
	allRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: false, Off: 0, Lim: 20})
	require.NoError(t, err)
	byName := rowsByName(t, allRows)
	assert.Nil(t, byName["Aset Tanpa Lokasi"].ResultRef, "no asset created for the location-less row")
	require.NotNil(t, byName["Aset Berlokasi"].ResultRef)
	var failedErrs []importer.CellError
	require.NoError(t, json.Unmarshal(byName["Aset Tanpa Lokasi"].Errors, &failedErrs))
	require.Len(t, failedErrs, 1)
	assert.Equal(t, "lokasi", failedErrs[0].Column)
	assert.Equal(t, "lokasiRequired", failedErrs[0].ErrorKey, "missing room reports 'required', not 'not found'")
}

// ─── 2. asset reject ─────────────────────────────────────────────────────────

// TestImport_AssetReject_DerivedStatusAndErrorReport drives the same pipeline
// to awaiting_approval, then rejects the batch: the job's own status stays
// awaiting_approval (rejection is derived, never synced onto the job row),
// GET /imports/:id surfaces "approval_status":"rejected" via
// Handler.enrichJob, and the invalid row remains retrievable through the
// error-report endpoint.
func TestImport_AssetReject_DerivedStatusAndErrorReport(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "REJ")
	seedCategory(t, h.pool, "REJ-CAT")
	seedRoom(t, h.pool, officeID, "Ruang REJ")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.rej@test.local")
	approverRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	approverID := seedUser(t, h.pool, approverRoleID, nil, "approver.rej@test.local")

	csvBytes := buildCSV(t, assetHeader, [][]string{
		{"", "Printer Valid", "REJ-CAT", "REJ", "2026-02-01", "2000000", "", "Ruang REJ"},
		{"", "Printer NoKategori", "TIDAK-ADA", "REJ", "2026-02-01", "2000000", "", "Ruang REJ"}, // unknown kategori
	})

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "reject.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	_, err = h.worker.Tick(ctx) // validate
	require.NoError(t, err)

	// Task 5: the validate phase's failed row (unknown kategori) must have
	// triggered storeErrorReport, persisting error_report_key and uploading a
	// report to object storage — before the job is ever confirmed/executed.
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	require.NotNil(t, job.ErrorReportKey, "validate phase must persist error_report_key when failed_rows > 0")

	storedRC, storedInfo, err := h.store.Get(ctx, *job.ErrorReportKey)
	require.NoError(t, err)
	storedBody, err := io.ReadAll(storedRC)
	require.NoError(t, err)
	require.NoError(t, storedRC.Close())
	require.NotEmpty(t, storedBody, "stored error report must not be empty")
	assert.Equal(t, "text/csv", storedInfo.ContentType)
	storedLines := strings.SplitN(string(storedBody), "\n", 2)
	require.NotEmpty(t, storedLines)
	header := storedLines[0]
	for _, col := range assetHeader {
		assert.Contains(t, header, col, "stored report header must contain the target's columns")
	}
	assert.Contains(t, header, "keterangan", "stored report header must contain the trailing keterangan column")

	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx) // execute -> submit approval
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	require.NotNil(t, job.RequestID)

	caller := approval.Caller{UserID: approverID, RoleID: approverRoleID, AllScope: true}
	note := "batch ditolak"
	decided, err := h.approvalSvc.Decide(ctx, *job.RequestID, caller, false, &note)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, decided.Status)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusAwaitingApproval, job.Status,
		"the job row itself is never synced to a rejected status")

	r := newRouter(h, makerID, makerRoleID)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest(http.MethodGet, "/api/v1/imports/"+job.ID.String(), nil)
	r.ServeHTTP(w, httpReq)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "awaiting_approval", body["status"])
	assert.Equal(t, "rejected", body["approval_status"], "derived from the approval request, not the job row")

	// Error report: the invalid row (unknown kategori) must be retrievable.
	// asset is approval-gated (NeedsApproval()==true), so per the followup fix
	// in handler.go this ALWAYS rebuilds on-demand rather than serving the
	// stored object at error_report_key (see
	// TestImport_AssetApprovedExecuteFailure_ErrorReportIncludesExecuteTimeFailure
	// for why: an asset job's stored report can go stale once execute-time
	// failures are appended after the validate phase already persisted it).
	// Content is still byte-identical to what was stored here because nothing
	// changed the failed-row set between validate and this request (the batch
	// was rejected, not approved, so no execute-time failures were added).
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/imports/"+job.ID.String()+"/error-report?format=csv", nil)
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Header().Get("Content-Disposition"), "import-errors-")
	report := w2.Body.String()
	assert.Contains(t, report, "Printer NoKategori")
	assert.Contains(t, report, "kat", "the kategori cell's error_key ('kat') is listed in keterangan")
	assert.NotContains(t, report, "Printer Valid", "only failed rows are listed in the error report")
	assert.Equal(t, storedBody, w2.Body.Bytes(),
		"error-report endpoint must stream exactly the object persisted at error_report_key")
}

// ─── 3. mid-batch tag collision (tx-poisoning regression) ──────────────────

// TestImport_AssetMidBatchTagCollision_BatchStillCompletes is the regression
// test for the Task 10 tx-poisoning fix: a batch's explicit asset_tag is
// still free at validation time, but by the time the batch is approved a
// concurrent (already-approved) batch has taken it. Without the pre-check in
// createRows, CreateAsset's 23505 would poison the WHOLE approval-commit
// transaction, rolling back the approval decision itself and permanently
// stranding the request. With the fix: the colliding row is marked failed
// (dupTag), the non-colliding row is still created, and the job completes.
func TestImport_AssetMidBatchTagCollision_BatchStillCompletes(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "COL")
	catID := seedCategory(t, h.pool, "COL-CAT")
	seedRoom(t, h.pool, officeID, "Ruang COL")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.col@test.local")
	approverRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	approverID := seedUser(t, h.pool, approverRoleID, nil, "approver.col@test.local")

	const collidingTag = "COL-EXPLICIT-01"
	csvBytes := buildCSV(t, assetHeader, [][]string{
		{collidingTag, "Aset Bentrok", "COL-CAT", "COL", "2026-03-01", "3000000", "", "Ruang COL"},
		{"", "Aset Selamat", "COL-CAT", "COL", "2026-03-01", "4000000", "", "Ruang COL"},
	})

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "collide.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	// Validate: the tag does not exist yet, so BOTH rows are valid.
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, job.SuccessRows)
	assert.EqualValues(t, 0, job.FailedRows)

	// Simulate a tag committed by a concurrent, already-approved batch between
	// THIS batch's validation and its own approval.
	_, err = h.pool.Exec(ctx,
		`INSERT INTO asset.assets (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, 'Concurrent Winner', $2, $3, 'intangible', true, '{}', 'available')`,
		collidingTag, catID, officeID)
	require.NoError(t, err)

	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx) // execute -> submit approval (rows untouched so far)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	require.NotNil(t, job.RequestID)

	caller := approval.Caller{UserID: approverID, RoleID: approverRoleID, AllScope: true}
	decided, err := h.approvalSvc.Decide(ctx, *job.RequestID, caller, true, nil)
	require.NoError(t, err, "the batch must still complete despite the mid-batch collision")
	assert.Equal(t, sqlc.SharedRequestStatusApproved, decided.Status)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status,
		"the batch completes; a single collision must not roll back the whole approval")
	assert.EqualValues(t, 1, job.SuccessRows, "only the non-colliding row was created")
	assert.EqualValues(t, 1, job.FailedRows, "the colliding row is marked failed instead of aborting")

	allRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: false, Off: 0, Lim: 20})
	require.NoError(t, err)
	byName := rowsByName(t, allRows)

	collided := byName["Aset Bentrok"]
	assert.False(t, collided.Valid, "MarkRowFailed flips the row to invalid at execute time")
	var collidedErrs []importer.CellError
	require.NoError(t, json.Unmarshal(collided.Errors, &collidedErrs))
	require.Len(t, collidedErrs, 1)
	assert.Equal(t, "asset_tag", collidedErrs[0].Column)
	assert.Equal(t, "dupTag", collidedErrs[0].ErrorKey)

	survived := byName["Aset Selamat"]
	assert.True(t, survived.Valid)
	require.NotNil(t, survived.ResultRef)
	survivedAsset, err := h.q.GetAssetByTag(ctx, *survived.ResultRef)
	require.NoError(t, err)
	assert.Equal(t, "Aset Selamat", survivedAsset.Name)

	// Exactly one asset carries collidingTag — the pre-inserted "concurrent
	// winner" — proving the importer's own would-be insert was skipped rather
	// than erroring out or double-creating.
	var count int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM asset.assets WHERE asset_tag = $1`, collidingTag).Scan(&count))
	assert.Equal(t, 1, count)
}

// TestImport_AssetApprovedExecuteFailure_ErrorReportIncludesExecuteTimeFailure
// is the regression test for the stale-stored-report fix in handler.go's
// errorReport: an asset batch that has BOTH a validate-time failure (unknown
// kategori) AND, after approval, an execute-time dup-tag collision (the same
// mid-batch TOCTOU technique as
// TestImport_AssetMidBatchTagCollision_BatchStillCompletes) on a DIFFERENT
// row. The validate phase persists error_report_key pointing at an object
// that only lists the validate-time failure; assetImportExec.Execute (see
// asset/executor.go) appends the execute-time failure to job.FailedRows but
// never refreshes that stored object. Before the fix, GET
// /imports/:id/error-report (format=csv, matching job.Format) would hit the
// handler's stored-object fast path and silently omit the execute-time
// failure. After the fix (guard now excludes NeedsApproval() targets), the
// endpoint always rebuilds on-demand for "asset" and returns both failures.
func TestImport_AssetApprovedExecuteFailure_ErrorReportIncludesExecuteTimeFailure(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "STL")
	catID := seedCategory(t, h.pool, "STL-CAT")
	seedRoom(t, h.pool, officeID, "Ruang STL")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.stl@test.local")
	approverRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	approverID := seedUser(t, h.pool, approverRoleID, nil, "approver.stl@test.local")

	const collidingTag = "STL-EXPLICIT-01"
	csvBytes := buildCSV(t, assetHeader, [][]string{
		{collidingTag, "Aset Bentrok Eksekusi", "STL-CAT", "STL", "2026-04-01", "3000000", "", "Ruang STL"},
		{"", "Aset Gagal Validasi", "TIDAK-ADA", "STL", "2026-04-01", "2000000", "", "Ruang STL"}, // unknown kategori -> fails at VALIDATE
		{"", "Aset Selamat", "STL-CAT", "STL", "2026-04-01", "4000000", "", "Ruang STL"},
	})

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "stale.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	// --- validate: only "Aset Gagal Validasi" fails (unknown kategori); the
	// colliding tag is still free at this point, so "Aset Bentrok Eksekusi"
	// validates fine.
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.EqualValues(t, 3, job.TotalRows)
	assert.EqualValues(t, 2, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)
	require.NotNil(t, job.ErrorReportKey, "validate phase must persist error_report_key when failed_rows > 0")

	// Snapshot the object the validate phase stored — this is the STALE
	// report the bug would have served after execute-time failures land.
	staleRC, _, err := h.store.Get(ctx, *job.ErrorReportKey)
	require.NoError(t, err)
	staleBody, err := io.ReadAll(staleRC)
	require.NoError(t, err)
	require.NoError(t, staleRC.Close())
	assert.Contains(t, string(staleBody), "Aset Gagal Validasi")
	assert.NotContains(t, string(staleBody), "Aset Bentrok Eksekusi",
		"the validate-phase stored report cannot yet know about the not-yet-occurred execute-time collision")

	// Simulate a tag committed by a concurrent, already-approved batch between
	// THIS batch's validation and its own approval (same technique as
	// TestImport_AssetMidBatchTagCollision_BatchStillCompletes).
	_, err = h.pool.Exec(ctx,
		`INSERT INTO asset.assets (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, 'Concurrent Winner STL', $2, $3, 'intangible', true, '{}', 'available')`,
		collidingTag, catID, officeID)
	require.NoError(t, err)

	// --- confirm + execute: submits the asset_import approval request.
	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	require.NotNil(t, job.RequestID)

	// --- approve: assetImportExec.Execute now discovers the mid-batch dup-tag
	// collision on "Aset Bentrok Eksekusi" and appends it to job.FailedRows,
	// WITHOUT touching error_report_key.
	caller := approval.Caller{UserID: approverID, RoleID: approverRoleID, AllScope: true}
	decided, err := h.approvalSvc.Decide(ctx, *job.RequestID, caller, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, decided.Status)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows, "only \"Aset Selamat\" survives to creation")
	assert.EqualValues(t, 2, job.FailedRows,
		"1 validate-time failure + 1 execute-time dup-tag collision")

	// error_report_key must still point at the STALE validate-only object —
	// the executor never refreshes it (that's the bug this test guards
	// against being reintroduced).
	stillStaleRC, _, err := h.store.Get(ctx, *job.ErrorReportKey)
	require.NoError(t, err)
	stillStaleBody, err := io.ReadAll(stillStaleRC)
	require.NoError(t, err)
	require.NoError(t, stillStaleRC.Close())
	assert.Equal(t, staleBody, stillStaleBody,
		"the executor does not refresh the stored object — precondition for this regression test")

	// --- the fix under test: GET /imports/:id/error-report must NOT serve
	// that stale stored object for an approval-gated target — it must rebuild
	// fresh and include BOTH failed rows.
	r := newRouter(h, makerID, makerRoleID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/imports/"+job.ID.String()+"/error-report?format=csv", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	report := w.Body.String()
	assert.Contains(t, report, "Aset Gagal Validasi", "the validate-time failure must still be listed")
	assert.Contains(t, report, "Aset Bentrok Eksekusi", "the execute-time dup-tag failure must be listed (this is the fix)")
	assert.NotContains(t, report, "Aset Selamat", "only failed rows are listed in the error report")
	assert.NotEqual(t, staleBody, w.Body.Bytes(),
		"the served report must not be the stale validate-only object")

	// The served report's row count must match the job's final FailedRows
	// (2), not the validate-phase-only count (1) that the stale object holds.
	reportLines := strings.Split(strings.TrimRight(report, "\n"), "\n")
	require.Len(t, reportLines, 3, "header + 2 failed rows")
}

// ─── 4. employee cycle ──────────────────────────────────────────────────────

// TestImport_EmployeeCycle_DuplicateCodeMarkedFailed drives the employee
// target (NeedsApproval() == false, so the worker executes it directly): a
// "kode" that collides with a row inserted directly into the DB between
// validation and execution (the same anti-poisoning race as the asset test,
// but for master data with no approval indirection) is marked failed while
// the other row succeeds and the job still completes.
func TestImport_EmployeeCycle_DuplicateCodeMarkedFailed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "EMP")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "masterdata.employee.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.emp@test.local")

	const dupCode = "EMP-DUP-01"
	csvBytes := buildCSV(t, employeeHeader, [][]string{
		{dupCode, "Pegawai Bentrok", "bentrok@test.local", "0800000001", "EMP", "active", "", ""},
		{"EMP-OK-01", "Pegawai Selamat", "selamat@test.local", "0800000002", "EMP", "active", "", ""},
	})

	job, err := h.importSvc.CreateJob(ctx, "employee", "csv", "employee.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	_, err = h.worker.Tick(ctx) // validate: no collision yet, both rows valid
	require.NoError(t, err)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 2, job.SuccessRows)

	// Simulate a code committed concurrently between validation and execution.
	_, err = h.pool.Exec(ctx,
		`INSERT INTO masterdata.employees (code, name, office_id) VALUES ($1, 'Concurrent Employee', $2)`,
		dupCode, officeID)
	require.NoError(t, err)

	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)

	_, err = h.worker.Tick(ctx) // execute: no approval needed, runs directly
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)

	survivor, err := h.q.GetEmployeeByCode(ctx, "EMP-OK-01")
	require.NoError(t, err)
	assert.Equal(t, "Pegawai Selamat", survivor.Name)

	failedRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: true, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, failedRows, 1)
	var errs []importer.CellError
	require.NoError(t, json.Unmarshal(failedRows[0].Errors, &errs))
	require.Len(t, errs, 1)
	assert.Equal(t, "kode", errs[0].Column)
	assert.Equal(t, "dupKode", errs[0].ErrorKey)
}

// TestImport_EmployeeCycle_DeptPositionResolved is a sibling of
// TestImport_EmployeeCycle_DuplicateCodeMarkedFailed focused on the new
// optional "departemen"/"jabatan" columns (Task 1): one row resolves both by
// name (department by its human name, position by its only lookup key) and,
// once executed, the created employee carries the resolved department_id /
// position_id; a second row names a department that does not exist and is
// rejected at VALIDATION time (department/position resolution has no
// execute-time re-check, unlike "kode" — see validateEmployeeRows) with the
// "departemen" error_key, and never reaches Execute at all.
func TestImport_EmployeeCycle_DeptPositionResolved(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "DPT")
	deptID := seedDepartment(t, h.pool, "Teknologi Informasi", "DEPT-TI")
	posID := seedPosition(t, h.pool, "Staf")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "masterdata.employee.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.dpt@test.local")

	csvBytes := buildCSV(t, employeeHeader, [][]string{
		{"EMP-DPT-01", "Pegawai Departemen", "dept@test.local", "0800000101", "DPT", "active", "Teknologi Informasi", "Staf"},
		{"EMP-DPT-02", "Pegawai Tanpa Departemen", "nodept@test.local", "0800000102", "DPT", "active", "Departemen Hantu", ""},
	})

	job, err := h.importSvc.CreateJob(ctx, "employee", "csv", "employee-dept.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	// --- validate: the unknown department is rejected here, not at execute ---
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)

	failedAtValidate, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: true, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, failedAtValidate, 1)
	var validateErrs []importer.CellError
	require.NoError(t, json.Unmarshal(failedAtValidate[0].Errors, &validateErrs))
	require.Len(t, validateErrs, 1)
	assert.Equal(t, "departemen", validateErrs[0].Column)
	assert.Equal(t, "departemen", validateErrs[0].ErrorKey)

	// --- confirm + execute (employee target needs no approval) ---
	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows, "the row rejected at validation still counts as failed on completion")

	created, err := h.q.GetEmployeeByCode(ctx, "EMP-DPT-01")
	require.NoError(t, err)
	require.NotNil(t, created.DepartmentID, "department_id must be resolved and persisted")
	assert.Equal(t, deptID, *created.DepartmentID)
	require.NotNil(t, created.PositionID, "position_id must be resolved and persisted")
	assert.Equal(t, posID, *created.PositionID)
	assert.Equal(t, officeID, created.OfficeID)
}

// ─── 5. 403 per permission ──────────────────────────────────────────────────

// TestImport_CreateJob_PermissionDenied verifies POST /imports enforces
// Service.PermissionKey per target: "Staf" (a migration-seeded default role,
// see db/migrations/000005_seed_identity.up.sql) holds neither "asset.manage"
// nor "masterdata.employee.manage", so creating either kind of import job
// must be rejected with 403 before any row is even parsed.
func TestImport_CreateJob_PermissionDenied(t *testing.T) {
	h := newHarness(t)

	stafRoleID := lookupRole(t, h.pool, "Staf")
	stafOfficeID := seedOfficeSimple(t, h.pool, "STF")
	stafUserID := seedUser(t, h.pool, stafRoleID, &stafOfficeID, "staf.denied@test.local")

	post := func(target string, header []string, row []string) int {
		r := newRouter(h, stafUserID, stafRoleID)
		body, contentType := multipartCSV(t, target, buildCSV(t, header, [][]string{row}))
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/imports", body)
		req.Header.Set("Content-Type", contentType)
		r.ServeHTTP(w, req)
		return w.Code
	}

	assert.Equal(t, http.StatusForbidden,
		post("asset", assetHeader, []string{"", "X", "X", "X", "2026-01-01", "1", "", ""}),
		"Staf lacks asset.manage")
	assert.Equal(t, http.StatusForbidden,
		post("employee", employeeHeader, []string{"X", "X", "", "", "X", "active", "", ""}),
		"Staf lacks masterdata.employee.manage")
}

// TestImport_CreateJob_PermissionGranted is the positive counterpart: a role
// holding the target's permission key may create the job (201), proving the
// 403 assertions above are not simply "always forbidden" false negatives.
func TestImport_CreateJob_PermissionGranted(t *testing.T) {
	h := newHarness(t)

	roleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	userID := seedUser(t, h.pool, roleID, nil, "granted.asset@test.local")

	r := newRouter(h, userID, roleID)
	body, contentType := multipartCSV(t, "asset", buildCSV(t, assetHeader,
		[][]string{{"", "X", "X", "X", "2026-01-01", "1", "", ""}}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/imports", body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

// ─── 6. scope enforcement ───────────────────────────────────────────────────

// TestImport_AssetValidate_ScopeOutOfReach verifies the M4 worker fix
// (resolveMakerScope in worker.go): a maker whose role is scoped to a single
// office (module "imports") uploading a row whose "kantor" names an office
// outside that scope gets that row rejected at validation, while the
// in-scope row validates cleanly.
//
// buildAssetLookups (internal/asset/importer.go) loads the office lookup
// table itself scoped to the caller (ListOffices with the SAME
// AllScope/OfficeIDs the worker resolved) — so an out-of-scope office code
// never even appears in the lookup map, and the row is rejected with the
// "kantor" (office not found) error_key rather than the separate "scope"
// error_key (which fires only when an office IS resolved from the lookup but
// fails the redundant post-hoc scope re-check — unreachable through this
// integration path given the lookup itself is already scope-filtered, but
// exercised directly by internal/asset/importer_test.go's unit tests against
// hand-built lookups). Either way, the caller's scope is enforced: the
// out-of-scope office is never resolvable and the row never validates.
func TestImport_AssetValidate_ScopeOutOfReach(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeIn := seedOfficeSimple(t, h.pool, "SIN")
	officeOut := seedOfficeSimple(t, h.pool, "SOUT")
	seedCategory(t, h.pool, "SCOPE-CAT")
	seedRoom(t, h.pool, officeIn, "Ruang SIN")
	seedRoom(t, h.pool, officeOut, "Ruang SOUT")

	makerRoleID := testsupport.SeedRole(t, h.pool, "scoped-maker-"+uuid.New().String()[:8])
	testsupport.SeedScopePolicy(t, h.pool, makerRoleID, "imports", sqlc.SharedScopeLevelOffice)
	grantPermission(t, h.pool, makerRoleID, "asset.manage")
	makerID := seedUser(t, h.pool, makerRoleID, &officeIn, "maker.scope@test.local")

	csvBytes := buildCSV(t, assetHeader, [][]string{
		{"", "Dalam Scope", "SCOPE-CAT", "SIN", "2026-04-01", "1000000", "", "Ruang SIN"},
		{"", "Luar Scope", "SCOPE-CAT", "SOUT", "2026-04-01", "1000000", "", "Ruang SOUT"},
	})

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "scope.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)

	failedRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: true, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, failedRows, 1)
	var errs []importer.CellError
	require.NoError(t, json.Unmarshal(failedRows[0].Errors, &errs))
	require.Len(t, errs, 2)
	byColumn := map[string]string{}
	for _, e := range errs {
		byColumn[e.Column] = e.ErrorKey
	}
	assert.Equal(t, "kantor", byColumn["kantor"], "the out-of-scope office is invisible to the scoped lookup")
	assert.Equal(t, "lokasi", byColumn["lokasi"], "no office resolved -> its room can never match either")
}

// TestImport_AssetValidate_MultiOfficeBatchFlagged verifies the asset
// importer's batch-office-consistency rule: a global-scope maker uploading
// rows resolving to two DIFFERENT offices gets the first office as the
// batch's office (valid) and every later row resolving elsewhere flagged
// "multiOffice", even though each row is individually well-formed and
// in-scope.
func TestImport_AssetValidate_MultiOfficeBatchFlagged(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	office1 := seedOfficeSimple(t, h.pool, "MO1")
	office2 := seedOfficeSimple(t, h.pool, "MO2")
	seedCategory(t, h.pool, "MO-CAT")
	seedRoom(t, h.pool, office1, "Ruang MO1")
	seedRoom(t, h.pool, office2, "Ruang MO2")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.multi@test.local")

	csvBytes := buildCSV(t, assetHeader, [][]string{
		{"", "Aset Kantor 1", "MO-CAT", "MO1", "2026-05-01", "1000000", "", "Ruang MO1"},
		{"", "Aset Kantor 2", "MO-CAT", "MO2", "2026-05-01", "1000000", "", "Ruang MO2"},
	})

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "multioffice.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, job.SuccessRows, "the first resolved office wins the batch")
	assert.EqualValues(t, 1, job.FailedRows)
	require.NotNil(t, job.OfficeID)
	assert.Equal(t, office1, *job.OfficeID)

	failedRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: true, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, failedRows, 1)
	var errs []importer.CellError
	require.NoError(t, json.Unmarshal(failedRows[0].Errors, &errs))
	require.Len(t, errs, 1)
	assert.Equal(t, "kantor", errs[0].Column)
	assert.Equal(t, "multiOffice", errs[0].ErrorKey)
}

// ─── 7. access control ──────────────────────────────────────────────────────

// TestImport_GetJob_ForeignJobForbidden verifies Service.GetJob's ownership
// check: a caller who did not create the job gets ErrForbidden, while the
// owner reads it fine.
func TestImport_GetJob_ForeignJobForbidden(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	roleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	userA := seedUser(t, h.pool, roleID, nil, "usera.access@test.local")
	userB := seedUser(t, h.pool, roleID, nil, "userb.access@test.local")

	csvBytes := buildCSV(t, assetHeader, [][]string{{"", "X", "X", "X", "2026-01-01", "1", "", ""}})
	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "own.csv", "text/csv", csvBytes, userA)
	require.NoError(t, err)

	_, err = h.importSvc.GetJob(ctx, job.ID, userB)
	require.ErrorIs(t, err, importer.ErrForbidden)

	got, err := h.importSvc.GetJob(ctx, job.ID, userA)
	require.NoError(t, err)
	assert.Equal(t, job.ID, got.ID)

	// Also exercised through the real HTTP route: userB's GET must 403.
	r := newRouter(h, userB, roleID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/imports/"+job.ID.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ─── 8. recovery ────────────────────────────────────────────────────────────

// TestImport_Recover_ResetsInFlightJobs verifies Worker.Recover: a job left
// "processing" (crashed mid-validate) resets to "pending"; a job left
// "executing" (crashed mid-execute) resets to "confirmed" — see
// RecoverStuckJobs in db/queries/importer.sql.
func TestImport_Recover_ResetsInFlightJobs(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	roleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, roleID, nil, "maker.recover@test.local")

	csvBytes := buildCSV(t, assetHeader, [][]string{{"", "X", "X", "X", "2026-01-01", "1", "", ""}})

	jobProcessing, err := h.importSvc.CreateJob(ctx, "asset", "csv", "p.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)
	_, err = h.pool.Exec(ctx, `UPDATE import.import_jobs SET status = 'processing' WHERE id = $1`, jobProcessing.ID)
	require.NoError(t, err)

	jobExecuting, err := h.importSvc.CreateJob(ctx, "asset", "csv", "e.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)
	_, err = h.pool.Exec(ctx, `UPDATE import.import_jobs SET status = 'executing' WHERE id = $1`, jobExecuting.ID)
	require.NoError(t, err)

	require.NoError(t, h.worker.Recover(ctx))

	got, err := h.importSvc.GetJob(ctx, jobProcessing.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusPending, got.Status, "processing -> pending")

	got, err = h.importSvc.GetJob(ctx, jobExecuting.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusConfirmed, got.Status, "executing -> confirmed")
}

// ─── 9. pagination + only_errors ────────────────────────────────────────────

// TestImport_Rows_PaginationAndOnlyErrors drives GET /imports/:id/rows
// end-to-end: 3 valid + 2 invalid rows, paginated 2-at-a-time, plus the
// only_errors filter.
func TestImport_Rows_PaginationAndOnlyErrors(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	officeID := seedOfficeSimple(t, h.pool, "PAG")
	seedCategory(t, h.pool, "PAG-CAT")
	seedRoom(t, h.pool, officeID, "Ruang PAG")

	roleID := seedGlobalMakerRole(t, h.pool, "asset.manage")
	makerID := seedUser(t, h.pool, roleID, nil, "maker.pag@test.local")

	var rowsIn [][]string
	for i := 1; i <= 3; i++ {
		rowsIn = append(rowsIn, []string{"", fmt.Sprintf("Valid %d", i), "PAG-CAT", "PAG", "2026-06-01", "1000000", "", "Ruang PAG"})
	}
	for i := 1; i <= 2; i++ {
		rowsIn = append(rowsIn, []string{"", "", "PAG-CAT", "PAG", "2026-06-01", "1000000", "", "Ruang PAG"}) // missing nama
		_ = i
	}
	csvBytes := buildCSV(t, assetHeader, rowsIn)

	job, err := h.importSvc.CreateJob(ctx, "asset", "csv", "pagination.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	r := newRouter(h, makerID, roleID)

	get := func(qs string) map[string]any {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/imports/"+job.ID.String()+"/rows?"+qs, nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return body
	}

	page1 := get("limit=2&offset=0")
	assert.EqualValues(t, 5, page1["total"])
	assert.Len(t, page1["data"], 2)
	assert.EqualValues(t, 2, page1["limit"])
	assert.EqualValues(t, 0, page1["offset"])

	page2 := get("limit=2&offset=2")
	assert.Len(t, page2["data"], 2)

	page3 := get("limit=2&offset=4")
	assert.Len(t, page3["data"], 1, "5 rows total, page 3 has the remainder")

	onlyErrors := get("only_errors=true&limit=20&offset=0")
	assert.EqualValues(t, 2, onlyErrors["total"])
	assert.Len(t, onlyErrors["data"], 2)
	for _, raw := range onlyErrors["data"].([]any) {
		rec := raw.(map[string]any)
		assert.Equal(t, false, rec["valid"])
	}
}

// ─── 10. reference targets (brands/units/models) ───────────────────────────
//
// Tasks 2-3 added the reference:brands, reference:units, reference:models
// import targets (internal/masterdata/reference/importer.go); Task 4 wires
// them into newHarness above exactly like router.go's production wiring. All
// three targets have NeedsApproval() == false, so the worker executes them
// directly (same shape as TestImport_EmployeeCycle_DuplicateCodeMarkedFailed)
// rather than routing through approval.Service like the asset target.

// TestImport_ReferenceBrandCycle_DuplicateNameMarkedFailed drives the brands
// target: two rows validate cleanly, then a name collision is committed
// directly to the DB between validation and execution (the same
// anti-poisoning race proven for employees/assets) — the colliding row is
// marked failed with dupNama while the other row is created and the job
// still completes.
func TestImport_ReferenceBrandCycle_DuplicateNameMarkedFailed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	makerRoleID := seedGlobalMakerRole(t, h.pool, "masterdata.global.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.brand@test.local")

	const dupName = "Brand Bentrok"
	csvBytes := buildCSV(t, brandHeader, [][]string{
		{dupName},
		{"Brand Selamat"},
	})

	job, err := h.importSvc.CreateJob(ctx, "reference:brands", "csv", "brands.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	_, err = h.worker.Tick(ctx) // validate: no collision yet, both rows valid
	require.NoError(t, err)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 2, job.SuccessRows)

	// Simulate a name committed concurrently between validation and execution.
	_, err = h.pool.Exec(ctx, `INSERT INTO masterdata.brands (name) VALUES ($1)`, dupName)
	require.NoError(t, err)

	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx) // execute: no approval needed, runs directly
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)

	survivor, err := h.q.GetBrandByName(ctx, "Brand Selamat")
	require.NoError(t, err)
	assert.Equal(t, "Brand Selamat", survivor.Name)

	failedRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: true, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, failedRows, 1)
	var errs []importer.CellError
	require.NoError(t, json.Unmarshal(failedRows[0].Errors, &errs))
	require.Len(t, errs, 1)
	assert.Equal(t, "nama", errs[0].Column)
	assert.Equal(t, "dupNama", errs[0].ErrorKey)
}

// TestImport_ReferenceUnitCycle_DuplicateNameMarkedFailed is the units-target
// sibling of the brands test above: same anti-poisoning shape, on
// masterdata.units instead.
func TestImport_ReferenceUnitCycle_DuplicateNameMarkedFailed(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	makerRoleID := seedGlobalMakerRole(t, h.pool, "masterdata.global.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.unit@test.local")

	const dupName = "Satuan Bentrok"
	csvBytes := buildCSV(t, unitHeader, [][]string{
		{dupName, "kg"},
		{"Satuan Selamat", "pcs"},
	})

	job, err := h.importSvc.CreateJob(ctx, "reference:units", "csv", "units.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	_, err = h.worker.Tick(ctx) // validate: no collision yet, both rows valid
	require.NoError(t, err)
	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 2, job.SuccessRows)

	// Simulate a name committed concurrently between validation and execution.
	_, err = h.pool.Exec(ctx, `INSERT INTO masterdata.units (name, symbol) VALUES ($1, 'kg')`, dupName)
	require.NoError(t, err)

	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx) // execute: no approval needed, runs directly
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)

	survivor, err := h.q.GetUnitByName(ctx, "Satuan Selamat")
	require.NoError(t, err)
	assert.Equal(t, "Satuan Selamat", survivor.Name)
	require.NotNil(t, survivor.Symbol)
	assert.Equal(t, "pcs", *survivor.Symbol)

	failedRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: true, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, failedRows, 1)
	var errs []importer.CellError
	require.NoError(t, json.Unmarshal(failedRows[0].Errors, &errs))
	require.Len(t, errs, 1)
	assert.Equal(t, "nama", errs[0].Column)
	assert.Equal(t, "dupNama", errs[0].ErrorKey)
}

// TestImport_ReferenceModelCycle_UnknownBrandAndDuplicatePair covers both
// model-specific rules in one pass through the pipeline: an unknown "merek"
// is rejected at VALIDATION time (mirrors
// TestImport_EmployeeCycle_DeptPositionResolved's "departemen" case — brand
// resolution has no execute-time re-check), while a (brand, nama) pair that
// collides with a row committed directly to the DB between validation and
// execution is marked failed at EXECUTE time with dupNama (the
// anti-poisoning race, proven here for the composite unique constraint
// uq_models_brand_name).
func TestImport_ReferenceModelCycle_UnknownBrandAndDuplicatePair(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	brandID := seedBrand(t, h.pool, "Canon")

	makerRoleID := seedGlobalMakerRole(t, h.pool, "masterdata.global.manage")
	makerID := seedUser(t, h.pool, makerRoleID, nil, "maker.model@test.local")

	const dupModelName = "EOS Bentrok"
	csvBytes := buildCSV(t, modelHeader, [][]string{
		{"Canon", "EOS Selamat"},
		{"Nikon", "Z6"}, // unknown brand -> rejected at validation
		{"Canon", dupModelName},
	})

	job, err := h.importSvc.CreateJob(ctx, "reference:models", "csv", "models.csv", "text/csv", csvBytes, makerID)
	require.NoError(t, err)

	// --- validate: the unknown brand is rejected here, not at execute ---
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusValidated, job.Status)
	assert.EqualValues(t, 3, job.TotalRows)
	assert.EqualValues(t, 2, job.SuccessRows)
	assert.EqualValues(t, 1, job.FailedRows)

	failedAtValidate, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: true, Off: 0, Lim: 20})
	require.NoError(t, err)
	require.Len(t, failedAtValidate, 1)
	var validateErrs []importer.CellError
	require.NoError(t, json.Unmarshal(failedAtValidate[0].Errors, &validateErrs))
	require.Len(t, validateErrs, 1)
	assert.Equal(t, "merek", validateErrs[0].Column)
	assert.Equal(t, "merek", validateErrs[0].ErrorKey)

	// Simulate a (brand, nama) pair committed concurrently between validation
	// and execution — the tx-poisoning regression check.
	_, err = h.pool.Exec(ctx,
		`INSERT INTO masterdata.models (brand_id, name) VALUES ($1, $2)`,
		brandID, dupModelName)
	require.NoError(t, err)

	// --- confirm + execute (models target needs no approval) ---
	job, err = h.importSvc.ConfirmJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	_, err = h.worker.Tick(ctx)
	require.NoError(t, err)

	job, err = h.importSvc.GetJob(ctx, job.ID, makerID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedImportStatusCompleted, job.Status)
	assert.EqualValues(t, 1, job.SuccessRows, "only the non-colliding valid row was created")
	assert.EqualValues(t, 2, job.FailedRows, "the validation-time failure plus the mid-batch collision")

	survivor, err := h.q.GetModelByBrandAndName(ctx, sqlc.GetModelByBrandAndNameParams{BrandID: brandID, Lower: "EOS Selamat"})
	require.NoError(t, err)
	assert.Equal(t, "EOS Selamat", survivor.Name)

	allRows, err := h.q.ListImportRows(ctx, sqlc.ListImportRowsParams{JobID: job.ID, OnlyErrors: false, Off: 0, Lim: 20})
	require.NoError(t, err)
	byName := rowsByName(t, allRows)
	collided := byName[dupModelName]
	assert.False(t, collided.Valid, "MarkRowFailed flips the row to invalid at execute time")
	var collidedErrs []importer.CellError
	require.NoError(t, json.Unmarshal(collided.Errors, &collidedErrs))
	require.Len(t, collidedErrs, 1)
	assert.Equal(t, "nama", collidedErrs[0].Column)
	assert.Equal(t, "dupNama", collidedErrs[0].ErrorKey)
}
