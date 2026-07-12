//go:build integration

package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestFieldPermissions(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	svc := authz.NewFieldService(sqlc.New(pool), rdb)
	ctx := context.Background()

	t.Run("ForEntity structures policies; unmapped field absent", func(t *testing.T) {
		testsupport.Reset(t, pool)
		role := testsupport.SeedRole(t, pool, "r-fields-1")
		testsupport.SeedFieldPermission(t, pool, role, "employees", "email", false, false)
		testsupport.SeedFieldPermission(t, pool, role, "employees", "name", true, false)
		testsupport.SeedFieldPermission(t, pool, role, "employees", "salary", false, false)

		pol, err := svc.ForEntity(ctx, role, "employees")
		require.NoError(t, err)
		assert.False(t, pol["email"].CanView)
		assert.True(t, pol["name"].CanView)
		assert.False(t, pol["salary"].CanView)
		_, ok := pol["code"]
		assert.False(t, ok, "a field with no policy row is absent from the map")
	})

	t.Run("FilterView drops non-viewable; default-allow keeps unmapped", func(t *testing.T) {
		testsupport.Reset(t, pool)
		role := testsupport.SeedRole(t, pool, "r-fields-2")
		testsupport.SeedFieldPermission(t, pool, role, "employees", "email", false, false)
		testsupport.SeedFieldPermission(t, pool, role, "employees", "name", true, false)
		testsupport.SeedFieldPermission(t, pool, role, "employees", "salary", false, false)

		pol, err := svc.ForEntity(ctx, role, "employees")
		require.NoError(t, err)

		data := map[string]any{"email": "a@b.c", "name": "Budi", "code": "E1", "salary": 100}
		authz.FilterView(pol, data)

		_, hasEmail := data["email"]
		_, hasSalary := data["salary"]
		_, hasName := data["name"]
		_, hasCode := data["code"]
		assert.False(t, hasEmail, "email not viewable -> dropped")
		assert.False(t, hasSalary, "salary not viewable -> dropped")
		assert.True(t, hasName, "name viewable -> kept")
		assert.True(t, hasCode, "code has no policy -> default-allow kept")
	})

	t.Run("field policies are cached (stale after DB change)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		require.NoError(t, rdb.FlushDB(ctx).Err())
		role := testsupport.SeedRole(t, pool, "r-fields-cache")
		testsupport.SeedFieldPermission(t, pool, role, "employees", "email", false, false)
		testsupport.SeedFieldPermission(t, pool, role, "employees", "name", true, false)
		testsupport.SeedFieldPermission(t, pool, role, "employees", "salary", false, false)

		first, err := svc.ForEntity(ctx, role, "employees")
		require.NoError(t, err)
		require.Len(t, first, 3)

		// Remove every policy for the role; without caching ForEntity would return an empty map.
		_, err = pool.Exec(ctx,
			`UPDATE identity.field_permissions SET deleted_at = now() WHERE role_id = $1`, role)
		require.NoError(t, err)

		second, err := svc.ForEntity(ctx, role, "employees")
		require.NoError(t, err)
		assert.Len(t, second, 3, "field cache should still serve the pre-change policies")
		assert.False(t, second["email"].CanView)
	})
}

// TestFilterEntity_RemovesNonViewableAndFailsClosed exercises the canonical
// FilterEntity helper: it must (a) delete masked fields from the map in
// place, keeping default-allow fields, and (b) propagate a policy-lookup
// error to the caller (fail-closed) instead of serving unfiltered data.
func TestFilterEntity_RemovesNonViewableAndFailsClosed(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	svc := authz.NewFieldService(sqlc.New(pool), rdb)
	ctx := context.Background()

	t.Run("removes non-viewable fields, keeps default-allow", func(t *testing.T) {
		testsupport.Reset(t, pool)
		role := testsupport.SeedRole(t, pool, "r-filterentity-ok")
		testsupport.SeedFieldPermission(t, pool, role, "assets", "book_value", false, false)
		testsupport.SeedFieldPermission(t, pool, role, "assets", "name", true, false)

		m := map[string]any{"name": "x", "book_value": "100"}
		err := svc.FilterEntity(ctx, role, "assets", m)
		require.NoError(t, err)

		_, ok := m["book_value"]
		require.False(t, ok, "book_value not viewable -> dropped")
		require.Contains(t, m, "name", "name viewable -> kept")
	})

	t.Run("propagates the lookup error (fail-closed)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		role := testsupport.SeedRole(t, pool, "r-filterentity-err")

		// A fresh role has no warm cache entry, so ForEntity must fall through
		// to Postgres. A canceled context makes both the Redis GET and the
		// Postgres query fail deterministically, forcing the lookup-error path.
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		m := map[string]any{"name": "x", "book_value": "100"}
		err := svc.FilterEntity(canceledCtx, role, "assets", m)
		require.Error(t, err, "FilterEntity must surface the ForEntity error rather than swallow it")
	})
}
