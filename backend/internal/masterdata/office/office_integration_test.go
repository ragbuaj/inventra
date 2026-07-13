//go:build integration

package office_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/masterdata/office"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func idSet(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func rowIDs(rows []sqlc.MasterdataOffice) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func f64(v float64) *float64 { return &v }

func TestOfficeDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("GetOfficeSubtree returns self + descendants only", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		sub, err := q.GetOfficeSubtree(ctx, tree.Wilayah)
		require.NoError(t, err)
		got := idSet(sub)
		assert.Len(t, sub, 2)
		assert.True(t, got[tree.Wilayah], "subtree includes itself")
		assert.True(t, got[tree.Cabang], "subtree includes descendant")
		assert.False(t, got[tree.Pusat], "subtree excludes ancestor")
		assert.False(t, got[tree.Wilayah2], "subtree excludes sibling")

		full, err := q.GetOfficeSubtree(ctx, tree.Pusat)
		require.NoError(t, err)
		assert.Len(t, full, 5, "root subtree spans the whole tree")
	})

	t.Run("scoped List returns only in-scope offices", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		got := rowIDs(rows)
		assert.True(t, got[tree.Wilayah] && got[tree.Cabang])
		assert.False(t, got[tree.Pusat] || got[tree.Wilayah2] || got[tree.Cabang2])
	})

	t.Run("global List returns all offices", func(t *testing.T) {
		testsupport.Reset(t, pool)
		testsupport.SeedOfficeTree(t, pool)

		rows, total, err := svc.List(ctx, true, nil, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, rows, 5)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, tree.Pusat, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, tree.Cabang, false, ids)
		require.NoError(t, err)
		assert.Equal(t, tree.Cabang, got.ID)
	})

	t.Run("Create rejects out-of-scope parent", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Bad", Code: "BAD", IsActive: true,
		})
		assert.ErrorIs(t, err, office.ErrParentOutOfScope)

		created, err := svc.Create(ctx, false, ids, office.CreateInput{
			ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
			Name: "Outlet", Code: "O1", IsActive: true,
		})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, *created.ParentID)
	})

	t.Run("Update rejects reparent outside scope", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, tree.Cabang, false, ids, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1", Code: "C1", IsActive: true,
			},
		})
		assert.ErrorIs(t, err, office.ErrReparentOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, tree.Wilayah2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted code can be reused (partial-unique)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Reuse", Code: "REUSE", IsActive: true,
		})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Reuse Again", Code: "REUSE", IsActive: true,
		})
		assert.NoError(t, err, "code reusable after soft delete")
	})

	t.Run("update advances updated_at (set_updated_at trigger)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		before, after, err := svc.Update(ctx, tree.Cabang, true, nil, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1 Renamed", Code: "C1", IsActive: true,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Cabang 1 Renamed", after.Name)
		assert.False(t, after.UpdatedAt.Time.Before(before.UpdatedAt.Time), "updated_at must not regress")
	})
}

func TestOfficeMapList(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("resolves names + coords, asset_count zero without assets", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		provID := uuid.New()
		cityID := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO masterdata.provinces (id, name, code) VALUES ($1,$2,$3)`, provID, "DKI Jakarta", "31")
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO masterdata.cities (id, province_id, name, code) VALUES ($1,$2,$3,$4)`, cityID, provID, "Jakarta Pusat", "3171")
		require.NoError(t, err)

		created, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			ProvinceID: &provID, CityID: &cityID,
			Name: "Map Office", Code: "MAP1", IsActive: true,
			Latitude: f64(-6.1754), Longitude: f64(106.8272),
		})
		require.NoError(t, err)

		rows, err := svc.MapList(ctx, true, nil)
		require.NoError(t, err)

		var got *sqlc.ListOfficesMapRow
		for i := range rows {
			if rows[i].ID == created.ID {
				got = &rows[i]
			}
		}
		require.NotNil(t, got, "created office present in map list")
		require.NotNil(t, got.OfficeTypeName)
		assert.NotEmpty(t, *got.OfficeTypeName)
		require.NotNil(t, got.ProvinceName)
		assert.Equal(t, "DKI Jakarta", *got.ProvinceName)
		require.NotNil(t, got.CityName)
		assert.Equal(t, "Jakarta Pusat", *got.CityName)
		require.NotNil(t, got.Latitude)
		assert.InDelta(t, -6.1754, *got.Latitude, 1e-9)
		assert.Equal(t, int64(0), got.AssetCount)
	})

	t.Run("respects data scope", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		outOfScope, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Under Pusat", Code: "UP1", IsActive: true,
		})
		require.NoError(t, err)

		rows, err := svc.MapList(ctx, false, []uuid.UUID{tree.Wilayah, tree.Cabang})
		require.NoError(t, err)
		ids := make(map[uuid.UUID]bool, len(rows))
		for _, r := range rows {
			ids[r.ID] = true
		}
		assert.True(t, ids[tree.Cabang], "in-scope office present")
		assert.False(t, ids[outOfScope.ID], "out-of-scope office absent")
		assert.False(t, ids[tree.Pusat], "ancestor out of scope absent")
	})
}

func TestOfficeCoordinates(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("create stores and returns coordinates", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		created, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Coord Office", Code: "COORD", IsActive: true,
			Latitude: f64(-6.1754), Longitude: f64(106.8272),
		})
		require.NoError(t, err)
		require.NotNil(t, created.Latitude)
		require.NotNil(t, created.Longitude)
		assert.InDelta(t, -6.1754, *created.Latitude, 1e-9)
		assert.InDelta(t, 106.8272, *created.Longitude, 1e-9)
	})

	t.Run("update changes coordinates", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		_, after, err := svc.Update(ctx, tree.Cabang, true, nil, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1", Code: "C1", IsActive: true,
				Latitude: f64(-6.29), Longitude: f64(106.80),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, after.Latitude)
		assert.InDelta(t, -6.29, *after.Latitude, 1e-9)
	})
}

// seedOfficeCaller inserts an identity.users row for roleID (CallerOfficeScope
// resolves the caller's office scope via the user row, not the JWT claims) and
// returns its id.
func seedOfficeCaller(t *testing.T, pool *pgxpool.Pool, roleID, officeID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		email, email, roleID, officeID).Scan(&id))
	return id
}

// officeDoRequest builds a fresh gin engine wired to the real office handler
// (via office.RegisterRoutes) with a stub auth middleware that injects the
// caller's user/role directly (bypassing real JWT), then drives an HTTP
// request and decodes the JSON body into a map for inspection.
func officeDoRequest(t *testing.T, h *office.Handler, method, path string, userID, roleID uuid.UUID) (int, map[string]any) {
	t.Helper()
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	r := gin.New()
	v1 := r.Group("/api/v1")
	office.RegisterRoutes(v1, h, stubAuth, stubAuth)
	w := httptest.NewRecorder()
	req, err := http.NewRequest(method, path, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)
	var body map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &body)
	}
	return w.Code, body
}

// TestOffice_Tree_ReturnsFullScopedSetNoLimit drives the real HTTP handler
// end-to-end and proves GET /offices/tree returns the full scoped set with
// no 100-row cap, unlike the paginated list endpoint.
func TestOffice_Tree_ReturnsFullScopedSetNoLimit(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	testsupport.Reset(t, pool)

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	auditSvc := audit.NewService(q)
	h := office.NewHandler(q, scopeSvc, auditSvc)

	ctx := context.Background()
	tree := testsupport.SeedOfficeTree(t, pool)

	const seedCount = 105
	for i := 0; i < seedCount; i++ {
		_, err := pool.Exec(ctx,
			`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
			 VALUES ($1, $2, $3, $4)`,
			tree.Wilayah, tree.OfficeTypeID, "Tree Office", uuid.NewString())
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	role := testsupport.SeedRole(t, pool, "r-office-tree-global")
	testsupport.SeedScopePolicy(t, pool, role, "offices", sqlc.SharedScopeLevelGlobal)
	caller := seedOfficeCaller(t, pool, role, tree.Pusat, "office-tree-global@test.local")

	code, body := officeDoRequest(t, h, http.MethodGet, "/api/v1/offices/tree", caller, role)
	require.Equal(t, http.StatusOK, code)

	data, ok := body["data"].([]any)
	require.True(t, ok, "data is an array")
	assert.GreaterOrEqual(t, len(data), seedCount, "no 100-row cap")

	total, ok := body["total"].(float64)
	require.True(t, ok, "total is present")
	assert.Equal(t, float64(len(data)), total)
}

// TestOffice_Tree_RespectsSubtreeScope proves GET /offices/tree still enforces
// the caller's office-subtree data scope: an office outside the subtree must
// not appear in the response.
func TestOffice_Tree_RespectsSubtreeScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	testsupport.Reset(t, pool)

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	auditSvc := audit.NewService(q)
	h := office.NewHandler(q, scopeSvc, auditSvc)

	ctx := context.Background()
	tree := testsupport.SeedOfficeTree(t, pool)

	var inScopeChild uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		tree.Cabang, tree.OfficeTypeID, "In Scope Child", "TREE-IN").Scan(&inScopeChild))

	gin.SetMode(gin.TestMode)
	role := testsupport.SeedRole(t, pool, "r-office-tree-subtree")
	testsupport.SeedScopePolicy(t, pool, role, "offices", sqlc.SharedScopeLevelOfficeSubtree)
	caller := seedOfficeCaller(t, pool, role, tree.Wilayah, "office-tree-subtree@test.local")

	code, body := officeDoRequest(t, h, http.MethodGet, "/api/v1/offices/tree", caller, role)
	require.Equal(t, http.StatusOK, code)

	data, ok := body["data"].([]any)
	require.True(t, ok, "data is an array")

	ids := make(map[string]bool, len(data))
	for _, item := range data {
		row, ok := item.(map[string]any)
		require.True(t, ok)
		id, ok := row["id"].(string)
		require.True(t, ok)
		ids[id] = true
	}

	assert.True(t, ids[inScopeChild.String()], "in-scope child present")
	assert.True(t, ids[tree.Wilayah.String()], "caller's own office present")
	assert.False(t, ids[tree.Wilayah2.String()], "out-of-scope sibling absent")
	assert.False(t, ids[tree.Cabang2.String()], "out-of-scope office absent")
	assert.False(t, ids[tree.Pusat.String()], "out-of-scope ancestor absent")
}
