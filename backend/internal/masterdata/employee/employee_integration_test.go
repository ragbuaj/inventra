//go:build integration

package employee_test

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
	"github.com/ragbuaj/inventra/internal/masterdata/employee"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func strptr(s string) *string { return &s }

func empIDs(rows []sqlc.MasterdataEmployee) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		m[r.ID] = true
	}
	return m
}

func TestEmployeeDataScope(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	svc := employee.NewService(sqlc.New(pool))
	ctx := context.Background()

	t.Run("scoped List returns only in-scope employees", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eW := testsupport.SeedEmployee(t, pool, tree.Wilayah, "E-W")
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		rows, total, err := svc.List(ctx, false, ids, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		got := empIDs(rows)
		assert.True(t, got[eW] && got[eC])
		assert.False(t, got[eW2])
	})

	t.Run("global List returns all employees", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		testsupport.SeedEmployee(t, pool, tree.Wilayah, "E-W")
		testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")

		rows, total, err := svc.List(ctx, true, nil, "", 100, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, rows, 3)
	})

	t.Run("Get out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Get(ctx, eW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)

		got, err := svc.Get(ctx, eC, false, ids)
		require.NoError(t, err)
		assert.Equal(t, eC, got.ID)
	})

	t.Run("Create rejects out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Create(ctx, false, ids, employee.CreateInput{
			Code: "E-BAD", Name: "Bad", OfficeID: tree.Pusat, Status: sqlc.SharedUserStatusActive,
		})
		assert.ErrorIs(t, err, employee.ErrOfficeOutOfScope)

		created, err := svc.Create(ctx, false, ids, employee.CreateInput{
			Code: "E-OK", Name: "Ok", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		require.NoError(t, err)
		assert.Equal(t, tree.Wilayah, created.OfficeID)
	})

	t.Run("Update rejects move to out-of-scope office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eC := testsupport.SeedEmployee(t, pool, tree.Cabang, "E-C")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, _, err := svc.Update(ctx, eC, false, ids, employee.UpdateInput{
			CreateInput: employee.CreateInput{
				Code: "E-C", Name: "Moved", OfficeID: tree.Pusat, Status: sqlc.SharedUserStatusActive,
			},
		})
		assert.ErrorIs(t, err, employee.ErrOfficeOutOfScope)
	})

	t.Run("Delete out of scope is not found", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		eW2 := testsupport.SeedEmployee(t, pool, tree.Wilayah2, "E-W2")
		ids := []uuid.UUID{tree.Wilayah, tree.Cabang}

		_, err := svc.Delete(ctx, eW2, false, ids)
		assert.ErrorIs(t, err, common.ErrNotFound)
	})

	t.Run("soft-deleted code can be reused", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		first, err := svc.Create(ctx, true, nil, employee.CreateInput{
			Code: "E-REUSE", Name: "First", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		require.NoError(t, err)

		_, err = svc.Delete(ctx, first.ID, true, nil)
		require.NoError(t, err)

		_, err = svc.Create(ctx, true, nil, employee.CreateInput{
			Code: "E-REUSE", Name: "Second", OfficeID: tree.Wilayah, Status: sqlc.SharedUserStatusActive,
		})
		assert.NoError(t, err, "code reusable after soft delete")
	})
}

func TestEmployeePhone(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := employee.NewService(q)
	ctx := context.Background()

	testsupport.Reset(t, pool)
	tree := testsupport.SeedOfficeTree(t, pool)

	created, err := svc.Create(ctx, true, nil, employee.CreateInput{
		Code: "EP-1", Name: "Phone Emp", OfficeID: tree.Cabang,
		Status: sqlc.SharedUserStatus("active"), Phone: strptr("0812-1111"),
	})
	require.NoError(t, err)
	require.NotNil(t, created.Phone)
	assert.Equal(t, "0812-1111", *created.Phone)

	_, after, err := svc.Update(ctx, created.ID, true, nil, employee.UpdateInput{CreateInput: employee.CreateInput{
		Code: "EP-1", Name: "Phone Emp", OfficeID: tree.Cabang,
		Status: sqlc.SharedUserStatus("active"), Phone: strptr("0813-2222"),
	}})
	require.NoError(t, err)
	require.NotNil(t, after.Phone)
	assert.Equal(t, "0813-2222", *after.Phone)

	created2, err := svc.Create(ctx, true, nil, employee.CreateInput{
		Code: "EP-2", Name: "No Phone", OfficeID: tree.Cabang, Status: sqlc.SharedUserStatus("active"),
	})
	require.NoError(t, err)
	assert.Nil(t, created2.Phone)
}

// seedEmployeeCaller inserts an identity.users row for roleID (CallerOfficeScope
// resolves the caller's office scope via the user row, not the JWT claims) and
// returns its id.
func seedEmployeeCaller(t *testing.T, pool *pgxpool.Pool, roleID, officeID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		email, email, roleID, officeID).Scan(&id))
	return id
}

// employeeDoRequest builds a fresh gin engine wired to the real employee
// handler (via employee.RegisterRoutes) with a stub auth middleware that
// injects the caller's user/role directly (bypassing real JWT), then drives
// an HTTP request and decodes the JSON body into a map for inspection.
func employeeDoRequest(t *testing.T, h *employee.Handler, method, path string, userID, roleID uuid.UUID) (int, map[string]any) {
	t.Helper()
	stubAuth := func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxRoleID, roleID.String())
		c.Next()
	}
	r := gin.New()
	v1 := r.Group("/api/v1")
	employee.RegisterRoutes(v1, h, stubAuth, stubAuth)
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

// TestEmployee_FieldMasking_HidesEmail drives the real HTTP handler (gin
// engine built via employee.RegisterRoutes) end-to-end and proves that
// field-permission masking actually executes on the get response path — not
// just that the underlying authz.FilterEntity/FilterView helpers work in
// isolation.
func TestEmployee_FieldMasking_HidesEmail(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	testsupport.Reset(t, pool)

	q := sqlc.New(pool)
	fieldSvc := authz.NewFieldService(q, rdb)
	scopeSvc := authz.NewScopeService(q, rdb)
	auditSvc := audit.NewService(q)
	h := employee.NewHandler(q, scopeSvc, auditSvc, fieldSvc)

	tree := testsupport.SeedOfficeTree(t, pool)
	empID := testsupport.SeedEmployee(t, pool, tree.Pusat, "E-MASK")

	gin.SetMode(gin.TestMode)

	t.Run("masked role: email hidden, name present", func(t *testing.T) {
		role := testsupport.SeedRole(t, pool, "r-emp-masking")
		testsupport.SeedScopePolicy(t, pool, role, "employees", sqlc.SharedScopeLevelGlobal)
		testsupport.SeedFieldPermission(t, pool, role, "employees", "email", false, false)
		caller := seedEmployeeCaller(t, pool, role, tree.Pusat, "emp-masking-caller@test.local")

		code, body := employeeDoRequest(t, h, http.MethodGet, "/api/v1/employees/"+empID.String(), caller, role)
		require.Equal(t, http.StatusOK, code)
		assert.NotContains(t, body, "email", "email not viewable -> dropped")
		assert.Contains(t, body, "name", "name has no policy -> default-allow kept")
	})

	t.Run("unmasked role: no policy rows -> all fields visible (default-allow)", func(t *testing.T) {
		role := testsupport.SeedRole(t, pool, "r-emp-unmasked")
		testsupport.SeedScopePolicy(t, pool, role, "employees", sqlc.SharedScopeLevelGlobal)
		caller := seedEmployeeCaller(t, pool, role, tree.Pusat, "emp-unmasked-caller@test.local")

		code, body := employeeDoRequest(t, h, http.MethodGet, "/api/v1/employees/"+empID.String(), caller, role)
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, "email", "no policy row -> default-allow keeps email")
		assert.Contains(t, body, "name")
	})
}
