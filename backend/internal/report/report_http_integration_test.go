//go:build integration

package report_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/report"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── HTTP-wiring harness (Task 8) ────────────────────────────────────────────
//
// Drives the real report.Handler through report.RegisterRoutes + a stub auth
// middleware + real middleware.RequirePermission, exactly like the depreciation
// httpHarness. The stub auth models RequireAuth: it reads the identity from
// X-Test-User/X-Test-Role headers and 401s when they are absent, so the
// "no Authorization header → 401" case exercises the gate genuinely.

// seedRoleUser inserts an active user with the named seeded role, placed in
// office, and returns (userID, roleID).
func seedRoleUser(t *testing.T, pool *pgxpool.Pool, roleName string, office uuid.UUID, sfx string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	roleID := lookupRole(t, pool, roleName)
	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		roleName+" "+sfx, roleName+"."+sfx+"@test.local", roleID, office).Scan(&userID))
	return userID, roleID
}

type reportHTTP struct {
	t        *testing.T
	pool     *pgxpool.Pool
	handler  *report.Handler
	permSvc  *authz.PermissionService
	f        fixture
	superUID uuid.UUID
	superRID uuid.UUID
	mgrUID   uuid.UUID
	mgrRID   uuid.UUID
	stafUID  uuid.UUID
	stafRID  uuid.UUID
}

// newReportHTTP boots a seeded DB (nil-Redis report service → dashboard reads
// are always fresh, so KPI assertions are deterministic), wires the handler
// with a Redis-backed scope/permission service, and seeds one user per role
// (Superadmin/Manager/Staf) in office A.
func newReportHTTP(t *testing.T) *reportHTTP {
	t.Helper()
	svc, pool := newSvc(t)
	f := seedFixture(t, pool)
	q := sqlc.New(pool)
	rdb := testsupport.NewRedis(t)
	scopeSvc := authz.NewScopeService(q, rdb)
	permSvc := authz.NewPermissionService(q, rdb)
	handler := report.NewHandler(svc, common.ScopedDeps{Q: q, Scope: scopeSvc})

	superUID, superRID := seedRoleUser(t, pool, "Superadmin", f.officeA, f.sfx+"-sa")
	mgrUID, mgrRID := seedRoleUser(t, pool, "Manager", f.officeA, f.sfx+"-mg")
	stafUID, stafRID := seedRoleUser(t, pool, "Staf", f.officeA, f.sfx+"-st")

	return &reportHTTP{
		t: t, pool: pool, handler: handler, permSvc: permSvc, f: f,
		superUID: superUID, superRID: superRID,
		mgrUID: mgrUID, mgrRID: mgrRID,
		stafUID: stafUID, stafRID: stafRID,
	}
}

// engine builds a fresh gin engine wired with report.RegisterRoutes + the stub
// auth + real permission middleware.
func (rh *reportHTTP) engine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	stubAuth := func(c *gin.Context) {
		uid := c.GetHeader("X-Test-User")
		rid := c.GetHeader("X-Test-Role")
		if uid == "" || rid == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set(middleware.CtxUserID, uid)
		c.Set(middleware.CtxRoleID, rid)
		c.Next()
	}
	r := gin.New()
	v1 := r.Group("/api/v1")
	report.RegisterRoutes(v1, rh.handler, stubAuth,
		middleware.RequirePermission(rh.permSvc, "report.view"),
		middleware.RequirePermission(rh.permSvc, "report.export"),
	)
	return r
}

// setAuth attaches the test identity headers unless userID is the nil UUID
// (nil → an unauthenticated request, for the 401 path).
func setAuth(req *http.Request, userID, roleID uuid.UUID) {
	if userID != uuid.Nil {
		req.Header.Set("X-Test-User", userID.String())
		req.Header.Set("X-Test-Role", roleID.String())
	}
}

// doJSON drives one request and JSON-decodes the body into a map.
func (rh *reportHTTP) doJSON(method, path string, userID, roleID uuid.UUID) (int, map[string]any) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	setAuth(req, userID, roleID)
	rh.engine().ServeHTTP(w, req)
	var body map[string]any
	if w.Body.Len() > 0 {
		require.NoError(rh.t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w.Code, body
}

// doRaw drives one request and returns the raw status/headers/body (for the
// binary export endpoints).
func (rh *reportHTTP) doRaw(method, path string, userID, roleID uuid.UUID) (int, http.Header, []byte) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	setAuth(req, userID, roleID)
	rh.engine().ServeHTTP(w, req)
	return w.Code, w.Header().Clone(), w.Body.Bytes()
}

// ─── tests ──────────────────────────────────────────────────────────────────

// TestReportHTTP_DashboardSummary_Superadmin: global scope → all 4 seeded
// assets, KPI echoed in the JSON body.
func TestReportHTTP_DashboardSummary_Superadmin(t *testing.T) {
	rh := newReportHTTP(t)
	code, body := rh.doJSON(http.MethodGet, "/api/v1/dashboard/summary?period=last30", rh.superUID, rh.superRID)
	require.Equal(t, http.StatusOK, code)
	kpi, ok := body["kpi"].(map[string]any)
	require.True(t, ok, "kpi object present")
	assert.EqualValues(t, 4, kpi["total_assets"], "global scope sees offices A + B")
}

// TestReportHTTP_DashboardSummary_ManagerOutOfScope is the SECURITY-CRITICAL
// case: a Manager (office-scoped to A) drilling into office B must be rejected
// with 403 BEFORE the service runs — no office-name or aggregate leak.
func TestReportHTTP_DashboardSummary_ManagerOutOfScope(t *testing.T) {
	rh := newReportHTTP(t)
	path := "/api/v1/dashboard/summary?period=last30&office_id=" + rh.f.officeB.String()
	code, body := rh.doJSON(http.MethodGet, path, rh.mgrUID, rh.mgrRID)
	require.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, body, "error")
}

// TestReportHTTP_ReportsAssets_ManagerScoped: a Manager sees only office-A rows
// (A1/A2/A3), never office B's asset.
func TestReportHTTP_ReportsAssets_ManagerScoped(t *testing.T) {
	rh := newReportHTTP(t)
	code, body := rh.doJSON(http.MethodGet, "/api/v1/reports/assets?period=last30", rh.mgrUID, rh.mgrRID)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "assets", body["type"])
	rows, ok := body["rows"].([]any)
	require.True(t, ok)
	assert.Len(t, rows, 3, "office B asset is out of the Manager's scope")
	assert.EqualValues(t, 3, body["row_count"])
}

// TestReportHTTP_ReportsAssets_StafView: Staf carries report.view (but not
// report.export) and an own-office scope → the JSON read succeeds with the same
// 3 office-A rows.
func TestReportHTTP_ReportsAssets_StafView(t *testing.T) {
	rh := newReportHTTP(t)
	code, body := rh.doJSON(http.MethodGet, "/api/v1/reports/assets?period=last30", rh.stafUID, rh.stafRID)
	require.Equal(t, http.StatusOK, code)
	rows, ok := body["rows"].([]any)
	require.True(t, ok)
	assert.Len(t, rows, 3)
}

// TestReportHTTP_InvalidReportType: an unknown :type → 400.
func TestReportHTTP_InvalidReportType(t *testing.T) {
	rh := newReportHTTP(t)
	code, _ := rh.doJSON(http.MethodGet, "/api/v1/reports/nope?period=last30", rh.superUID, rh.superRID)
	assert.Equal(t, http.StatusBadRequest, code)
}

// TestReportHTTP_HalfCustomRange: a custom range with only date_from (no
// date_to) is an invalid period → 400.
func TestReportHTTP_HalfCustomRange(t *testing.T) {
	rh := newReportHTTP(t)
	code, _ := rh.doJSON(http.MethodGet, "/api/v1/reports/assets?date_from=2026-01-01", rh.superUID, rh.superRID)
	assert.Equal(t, http.StatusBadRequest, code)
}

// TestReportHTTP_ExportXLSX_Manager: the assets xlsx export returns 200 with the
// spreadsheet content type + an attachment disposition, and the body opens as a
// valid workbook.
func TestReportHTTP_ExportXLSX_Manager(t *testing.T) {
	rh := newReportHTTP(t)
	code, hdr, body := rh.doRaw(http.MethodGet, "/api/v1/reports/assets/export?format=xlsx&period=last30", rh.mgrUID, rh.mgrRID)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, hdr.Get("Content-Type"), "spreadsheetml.sheet")
	assert.Equal(t, "nosniff", hdr.Get("X-Content-Type-Options"))
	assert.True(t, strings.HasPrefix(hdr.Get("Content-Disposition"), `attachment; filename="laporan-assets-`),
		"disposition = %q", hdr.Get("Content-Disposition"))
	assert.True(t, strings.HasSuffix(hdr.Get("Content-Disposition"), `.xlsx"`))

	xf, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err, "export body must be a valid xlsx")
	defer xf.Close() //nolint:errcheck
	rows, err := xf.GetRows("Laporan")
	require.NoError(t, err)
	require.NotEmpty(t, rows, "sheet has a header + data rows")
	assert.Equal(t, "Kode", rows[0][0], "first column header")
}

// TestReportHTTP_ExportPDF_StafForbidden: Staf lacks report.export → the export
// route is 403 even though the plain read would be 200.
func TestReportHTTP_ExportPDF_StafForbidden(t *testing.T) {
	rh := newReportHTTP(t)
	code, _, _ := rh.doRaw(http.MethodGet, "/api/v1/reports/assets/export?format=pdf&period=last30", rh.stafUID, rh.stafRID)
	assert.Equal(t, http.StatusForbidden, code)
}

// TestReportHTTP_ExportInvalidFormat: format=docx is neither xlsx nor pdf → 400.
func TestReportHTTP_ExportInvalidFormat(t *testing.T) {
	rh := newReportHTTP(t)
	code, _, _ := rh.doRaw(http.MethodGet, "/api/v1/reports/assets/export?format=docx&period=last30", rh.superUID, rh.superRID)
	assert.Equal(t, http.StatusBadRequest, code)
}

// TestReportHTTP_GlRecapVariant_WrongType: variant=gl_recap is only valid for
// the disposals type; on transfers it is 422.
func TestReportHTTP_GlRecapVariant_WrongType(t *testing.T) {
	rh := newReportHTTP(t)
	code, _, _ := rh.doRaw(http.MethodGet, "/api/v1/reports/transfers/export?variant=gl_recap&format=xlsx&period=last30", rh.superUID, rh.superRID)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
}

// TestReportHTTP_GlRecapVariant_DisposalsBalanced: a single gain disposal in
// the window yields a balanced GL recap (Σ debit = Σ credit), exported as a
// valid xlsx whose TOTAL row balances.
func TestReportHTTP_GlRecapVariant_DisposalsBalanced(t *testing.T) {
	rh := newReportHTTP(t)
	today := refToday()

	// A fresh office-A asset with a gain disposal inside last30.
	assetID := insertAsset(t, rh.pool, assetSeed{
		tag: "DGL-" + rh.f.sfx, name: "Aset Dilepas " + rh.f.sfx, category: lookupCatID(t, rh.pool, rh.f.catName),
		office: rh.f.officeA, room: nil, class: "intangible", status: "disposed",
		cost: "12000000.00", book: "8000000.00", purchaseDate: d(today.AddDate(0, 0, -20)), excluded: false,
	})
	insertDisposal(t, rh.pool, assetID, rh.superUID, "sale",
		d(today.AddDate(0, 0, -5)), "10000000.00", "8000000.00", "2000000.00")

	code, hdr, body := rh.doRaw(http.MethodGet, "/api/v1/reports/disposals/export?variant=gl_recap&format=xlsx&period=last30", rh.superUID, rh.superRID)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, hdr.Get("Content-Type"), "spreadsheetml.sheet")
	assert.True(t, strings.HasPrefix(hdr.Get("Content-Disposition"), `attachment; filename="laporan-disposals-gl-`))

	xf, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer xf.Close() //nolint:errcheck
	rows, err := xf.GetRows("Rekap GL")
	require.NoError(t, err)

	var debit, credit string
	for _, r := range rows {
		if len(r) >= 4 && r[0] == "TOTAL" {
			debit, credit = r[2], r[3]
		}
	}
	require.NotEmpty(t, debit, "TOTAL row present")
	assert.Equal(t, "10000000.00", debit, "Σ debit = proceeds")
	assert.Equal(t, debit, credit, "recap balances: Σ debit = Σ credit")
}

// TestReportHTTP_NoAuth: no identity headers → the auth gate 401s before any
// permission check or handler runs.
func TestReportHTTP_NoAuth(t *testing.T) {
	rh := newReportHTTP(t)
	code, _ := rh.doJSON(http.MethodGet, "/api/v1/dashboard/summary?period=last30", uuid.Nil, uuid.Nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

// lookupCatID resolves a seeded category's id by name (for tests that add an
// asset to the fixture's existing category).
func lookupCatID(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM masterdata.categories WHERE name = $1 AND deleted_at IS NULL LIMIT 1`,
		name).Scan(&id))
	return id
}
