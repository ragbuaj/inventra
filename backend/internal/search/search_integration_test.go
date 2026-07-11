//go:build integration

package search_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/search"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── seeding helpers (modeled on internal/assignment/assignment_integration_test.go) ──

// seedOffice inserts a fresh office_type + one office (distinct name/code) and
// returns the office ID.
func seedOffice(t *testing.T, pool *pgxpool.Pool, typeCode, name, code string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		typeCode).Scan(&typeID))

	var officeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`,
		typeID, name, code).Scan(&officeID))

	return officeID
}

// seedCategory inserts a masterdata.categories row (intangible, avoids the room
// FK constraint) and returns its id.
func seedCategory(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.categories (name, code, asset_class)
		 VALUES ($1, $2, 'intangible') RETURNING id`,
		code, code).Scan(&id))
	return id
}

// seedAsset inserts an asset.assets row directly with the given status and
// returns its id.
func seedAsset(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', $5::shared.asset_status)
		 RETURNING id`,
		tag, name, categoryID, officeID, status).Scan(&id))
	return id
}

// seedUser inserts an identity.users row with an explicit name (distinct from
// email) and returns its id.
func seedUser(t *testing.T, pool *pgxpool.Pool, roleID, officeID uuid.UUID, name, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		name, email, roleID, officeID).Scan(&id))
	return id
}

// grantPermission inserts a role_permissions row for the given role + key.
func grantPermission(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, key string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, $2)`,
		roleID, key)
	require.NoError(t, err)
}

// seedRequest inserts a pending approval.requests row and returns its id.
func seedRequest(t *testing.T, pool *pgxpool.Pool, officeID, requestedByID uuid.UUID, reason string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO approval.requests (type, office_id, reason, status, requested_by_id)
		 VALUES ('asset_transfer', $1, $2, 'pending', $3) RETURNING id`,
		officeID, reason, requestedByID).Scan(&id))
	return id
}

// ─── response decoding ──────────────────────────────────────────────────────

type searchItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Status      *string `json:"status"`
	AssetTag    *string `json:"asset_tag,omitempty"`
	RequestType *string `json:"request_type,omitempty"`
}

type searchGroup struct {
	Type  string       `json:"type"`
	Total int64        `json:"total"`
	Items []searchItem `json:"items"`
}

type searchResponse struct {
	Groups []searchGroup `json:"groups"`
}

// fixedOrder is the response's mandated group ordering (internal/search/service.go).
var fixedOrder = []string{"assets", "employees", "offices", "users", "requests"}

func groupTypes(groups []searchGroup) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = g.Type
	}
	return out
}

func findGroup(groups []searchGroup, typ string) *searchGroup {
	for i := range groups {
		if groups[i].Type == typ {
			return &groups[i]
		}
	}
	return nil
}

// assertGroupOrder checks that the groups present in the response appear as the
// subsequence of fixedOrder that matches their types — not just membership.
func assertGroupOrder(t *testing.T, groups []searchGroup) {
	t.Helper()
	present := make(map[string]bool, len(groups))
	for _, g := range groups {
		present[g.Type] = true
	}
	var expected []string
	for _, typ := range fixedOrder {
		if present[typ] {
			expected = append(expected, typ)
		}
	}
	assert.Equal(t, expected, groupTypes(groups),
		"groups must appear in the fixed order assets,employees,offices,users,requests")
}

func containsID(items []searchItem, id uuid.UUID) bool {
	target := id.String()
	for _, it := range items {
		if it.ID == target {
			return true
		}
	}
	return false
}

// ─── harness ─────────────────────────────────────────────────────────────────

// caller is mutated between subtests so the shared stub-auth middleware picks
// up the current test's user/role without rebuilding the router.
type caller struct {
	userID uuid.UUID
	roleID uuid.UUID
}

type harness struct {
	pool   *pgxpool.Pool
	router *gin.Engine
	cur    *caller
	uid    string

	officeA uuid.UUID
	officeB uuid.UUID
	catID   uuid.UUID

	assetATag string
	assetBTag string

	adminRole uuid.UUID
	adminUser uuid.UUID

	stafRole uuid.UUID
	stafUser uuid.UUID

	subtreeRole uuid.UUID
	subtreeUser uuid.UUID

	noPermRole uuid.UUID
	noPermUser uuid.UUID

	reqID uuid.UUID
}

// newHarness boots Postgres + Redis, seeds two offices (A, B), a superadmin-like
// role (global scope, asset.view + user.manage), a staf-like role (own scope,
// asset.view only), an office_subtree-scoped role, a role with NO permissions
// at all, one asset per office, one employee in office A, and one pending
// approval request in office A. All seeded names/reasons carry "SRCH" so a
// single query term exercises every group.
func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	q := sqlc.New(pool)

	uid := uuid.New().String()[:8]

	officeA := seedOffice(t, pool, "SrchTypeA-"+uid, "Kantor SRCH Alpha", "OFC-SRCH-A-"+uid)
	officeB := seedOffice(t, pool, "SrchTypeB-"+uid, "Kantor Beta SRCH", "OFC-SRCH-B-"+uid)
	catID := seedCategory(t, pool, "SRCHCAT-"+uid)

	assetATag := "TAG-SRCH-A-" + uid
	seedAsset(t, pool, assetATag, "Laptop Alpha SRCH", catID, officeA, "available")
	assetBTag := "TAG-SRCH-B-" + uid
	seedAsset(t, pool, assetBTag, "Laptop Beta SRCH", catID, officeB, "available")

	testsupport.SeedEmployee(t, pool, officeA, "EMP-SRCH-"+uid)

	adminRole := testsupport.SeedRole(t, pool, "SrchAdmin-"+uid)
	testsupport.SeedScopePolicy(t, pool, adminRole, "*", sqlc.SharedScopeLevelGlobal)
	grantPermission(t, pool, adminRole, "asset.view")
	grantPermission(t, pool, adminRole, "user.manage")
	adminUser := seedUser(t, pool, adminRole, officeA, "Admin SRCH", "admin.srch."+uid+"@test.local")

	stafRole := testsupport.SeedRole(t, pool, "SrchStaf-"+uid)
	testsupport.SeedScopePolicy(t, pool, stafRole, "*", sqlc.SharedScopeLevelOwn)
	grantPermission(t, pool, stafRole, "asset.view")
	stafUser := seedUser(t, pool, stafRole, officeA, "Staf Person", "staf.srch."+uid+"@test.local")

	subtreeRole := testsupport.SeedRole(t, pool, "SrchSubtree-"+uid)
	testsupport.SeedScopePolicy(t, pool, subtreeRole, "assets", sqlc.SharedScopeLevelOfficeSubtree)
	grantPermission(t, pool, subtreeRole, "asset.view")
	subtreeUser := seedUser(t, pool, subtreeRole, officeA, "Subtree Person", "subtree.srch."+uid+"@test.local")

	// No role_permissions rows at all — only auth-only (permission-less) groups
	// should ever appear for this role.
	noPermRole := testsupport.SeedRole(t, pool, "SrchNoPerm-"+uid)
	testsupport.SeedScopePolicy(t, pool, noPermRole, "*", sqlc.SharedScopeLevelOffice)
	noPermUser := seedUser(t, pool, noPermRole, officeA, "NoPerm Person", "noperm.srch."+uid+"@test.local")

	reqID := seedRequest(t, pool, officeA, adminUser, "beli laptop SRCH")

	scopeSvc := authz.NewScopeService(q, rdb)
	permSvc := authz.NewPermissionService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	svc := search.NewService(q)
	h := search.NewHandler(svc, permSvc, scoped)

	cur := &caller{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := router.Group("/api/v1")
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, cur.userID.String())
		c.Set(middleware.CtxRoleID, cur.roleID.String())
		c.Next()
	}
	search.RegisterRoutes(rg, h, stubAuth)

	return &harness{
		pool:   pool,
		router: router,
		cur:    cur,
		uid:    uid,

		officeA: officeA,
		officeB: officeB,
		catID:   catID,

		assetATag: assetATag,
		assetBTag: assetBTag,

		adminRole: adminRole,
		adminUser: adminUser,

		stafRole: stafRole,
		stafUser: stafUser,

		subtreeRole: subtreeRole,
		subtreeUser: subtreeUser,

		noPermRole: noPermRole,
		noPermUser: noPermUser,

		reqID: reqID,
	}
}

// search fires GET /api/v1/search?q=... as the given caller and decodes the response.
func (h *harness) search(t *testing.T, userID, roleID uuid.UUID, q string) (int, searchResponse) {
	t.Helper()
	h.cur.userID = userID
	h.cur.roleID = roleID

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+url.QueryEscape(q), nil)
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)

	var resp searchResponse
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "body: %s", w.Body.String())
	}
	return w.Code, resp
}

// ─── scenarios ───────────────────────────────────────────────────────────────

func TestSearch_Integration(t *testing.T) {
	h := newHarness(t)

	// (a) admin (global scope, asset.view + user.manage) searching "SRCH" sees
	// every group — assets, employees, offices, users, requests all carry the
	// term — and the groups must appear in the fixed order.
	t.Run("admin_sees_all_groups", func(t *testing.T) {
		code, resp := h.search(t, h.adminUser, h.adminRole, "SRCH")
		require.Equal(t, http.StatusOK, code)

		types := groupTypes(resp.Groups)
		assert.Contains(t, types, "assets")
		assert.Contains(t, types, "employees")
		assert.Contains(t, types, "offices")
		assert.Contains(t, types, "users")
		assert.Contains(t, types, "requests")
		assertGroupOrder(t, resp.Groups)

		usersGroup := findGroup(resp.Groups, "users")
		require.NotNil(t, usersGroup, "users group must be present")
		assert.True(t, containsID(usersGroup.Items, h.adminUser),
			"users group must include the admin's own account (name contains the query term)")
	})

	// (b) staf (own scope -> office A, asset.view only, no user.manage) searching
	// "SRCH": assets restricted to office A; users group absent.
	t.Run("staf_scoped_and_no_users", func(t *testing.T) {
		code, resp := h.search(t, h.stafUser, h.stafRole, "SRCH")
		require.Equal(t, http.StatusOK, code)

		types := groupTypes(resp.Groups)
		assert.NotContains(t, types, "users", "staf role lacks user.manage — users group must be absent")

		assetsGroup := findGroup(resp.Groups, "assets")
		require.NotNil(t, assetsGroup, "staf must see office A's matching asset")
		var subtitles []string
		for _, it := range assetsGroup.Items {
			subtitles = append(subtitles, it.Subtitle)
		}
		assert.Contains(t, subtitles, h.assetATag, "office-A asset must be present")
		assert.NotContains(t, subtitles, h.assetBTag, "office-B asset must not leak into a staf's office-A-scoped results")
	})

	// (c) role with office_subtree scope on module "assets", user placed in
	// office A: sees A's asset, not B's (exercises the subtree-expansion path,
	// distinct from the "own"/"office" path in scenario b).
	t.Run("subtree_scope_filters", func(t *testing.T) {
		code, resp := h.search(t, h.subtreeUser, h.subtreeRole, "SRCH")
		require.Equal(t, http.StatusOK, code)

		assetsGroup := findGroup(resp.Groups, "assets")
		require.NotNil(t, assetsGroup, "subtree-scoped role must see office-A's matching asset")
		var subtitles []string
		for _, it := range assetsGroup.Items {
			subtitles = append(subtitles, it.Subtitle)
		}
		assert.Contains(t, subtitles, h.assetATag)
		assert.NotContains(t, subtitles, h.assetBTag, "office-B asset is outside office A's subtree")
	})

	// (d) requests match both the free-text reason and an id-prefix.
	t.Run("requests_match_reason_and_id", func(t *testing.T) {
		code, resp := h.search(t, h.adminUser, h.adminRole, "beli laptop")
		require.Equal(t, http.StatusOK, code)
		reqGroup := findGroup(resp.Groups, "requests")
		require.NotNil(t, reqGroup, "reason match must surface the requests group")
		assert.True(t, containsID(reqGroup.Items, h.reqID), "expected the seeded request id in reason-matched results")

		prefix := h.reqID.String()[:8]
		code2, resp2 := h.search(t, h.adminUser, h.adminRole, prefix)
		require.Equal(t, http.StatusOK, code2)
		reqGroup2 := findGroup(resp2.Groups, "requests")
		require.NotNil(t, reqGroup2, "id-prefix match must surface the requests group")
		assert.True(t, containsID(reqGroup2.Items, h.reqID), "expected the seeded request id in id-prefix-matched results")
	})

	// (e) a query below MinQueryLen (2 runes) short-circuits to an empty groups
	// array without ever hitting the gates/queries.
	t.Run("short_query_empty", func(t *testing.T) {
		code, resp := h.search(t, h.adminUser, h.adminRole, "a")
		require.Equal(t, http.StatusOK, code)
		assert.Empty(t, resp.Groups)
	})

	// (f) PerGroupLimit caps items at 5 per group while total still reports the
	// full match count.
	t.Run("limit_and_total", func(t *testing.T) {
		for i := 1; i <= 7; i++ {
			tag := fmt.Sprintf("TAG-BULK-SRCH-%02d-%s", i, h.uid)
			name := fmt.Sprintf("Bulk SRCH %d", i)
			seedAsset(t, h.pool, tag, name, h.catID, h.officeA, "available")
		}

		code, resp := h.search(t, h.adminUser, h.adminRole, "Bulk SRCH")
		require.Equal(t, http.StatusOK, code)
		assetsGroup := findGroup(resp.Groups, "assets")
		require.NotNil(t, assetsGroup)
		assert.Len(t, assetsGroup.Items, 5, "items must be capped at PerGroupLimit")
		assert.EqualValues(t, 7, assetsGroup.Total, "total must report the full match count, not the capped item count")
	})

	// (g) a role with NO role_permissions rows at all still sees auth-only
	// (permission-less) groups like offices, but never assets (needs
	// asset.view) or users (needs user.manage) — even though a matching asset
	// exists in the caller's own office scope.
	t.Run("no_permission_role_sees_only_ungated_groups", func(t *testing.T) {
		code, resp := h.search(t, h.noPermUser, h.noPermRole, "SRCH")
		require.Equal(t, http.StatusOK, code)

		types := groupTypes(resp.Groups)
		assert.Contains(t, types, "offices", "offices is auth-only (no permission gate) and office A matches in-scope")
		assert.NotContains(t, types, "assets", "assets group requires asset.view, which this role lacks — must be absent despite an in-scope matching asset")
		assert.NotContains(t, types, "users", "users group requires user.manage, which this role lacks")
	})
}
