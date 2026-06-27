//go:build integration

package authz_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func idSet(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestScopeResolve(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	svc := authz.NewScopeService(sqlc.New(pool), rdb)
	ctx := context.Background()

	t.Run("global", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-global")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelGlobal)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelGlobal, sc.Level)
		assert.Empty(t, sc.OfficeIDs)
	})

	t.Run("own", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-own")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOwn)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)
	})

	t.Run("office", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-office")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOffice)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOffice, sc.Level)
		assert.Equal(t, []uuid.UUID{tree.Wilayah}, sc.OfficeIDs)
	})

	t.Run("office_subtree spans descendants", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-sub")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOfficeSubtree)

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, sc.Level)
		got := idSet(sc.OfficeIDs)
		assert.Len(t, sc.OfficeIDs, 2)
		assert.True(t, got[tree.Wilayah] && got[tree.Cabang])
		assert.False(t, got[tree.Pusat] || got[tree.Wilayah2])
	})

	t.Run("nil office falls back to own", func(t *testing.T) {
		testsupport.Reset(t, pool)
		testsupport.SeedOfficeTree(t, pool)
		// Two separate roles so each path uses its own (uncached) policy set —
		// seeding a second policy into one role after a Resolve would be hidden
		// by the per-role policy cache.
		subRole := testsupport.SeedRole(t, pool, "r-nil-subtree")
		testsupport.SeedScopePolicy(t, pool, subRole, "*", sqlc.SharedScopeLevelOfficeSubtree)
		offRole := testsupport.SeedRole(t, pool, "r-nil-office")
		testsupport.SeedScopePolicy(t, pool, offRole, "*", sqlc.SharedScopeLevelOffice)

		sub, err := svc.Resolve(ctx, subRole, nil, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, sub.Level, "office_subtree + nil office -> own")

		off, err := svc.Resolve(ctx, offRole, nil, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, off.Level, "office + nil office -> own")
	})

	t.Run("no policy falls back to own", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-empty")

		sc, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)
	})

	t.Run("per-module override beats default", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-override")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOwn)
		testsupport.SeedScopePolicy(t, pool, role, "employees", sqlc.SharedScopeLevelOfficeSubtree)

		emp, err := svc.Resolve(ctx, role, &tree.Wilayah, "employees")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, emp.Level)

		off, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOwn, off.Level)
	})

	t.Run("policy result is cached (stale after DB change)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		require.NoError(t, rdb.FlushDB(ctx).Err())
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-cache-policy")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOfficeSubtree)

		first, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		require.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, first.Level)

		// Soft-delete the policy in the DB; without caching this would resolve to own.
		_, err = pool.Exec(ctx,
			`UPDATE identity.data_scope_policies SET deleted_at = now() WHERE role_id = $1`, role)
		require.NoError(t, err)

		second, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedScopeLevelOfficeSubtree, second.Level,
			"policy cache should still serve the pre-change level")
	})

	t.Run("subtree result is cached (stale after new child)", func(t *testing.T) {
		testsupport.Reset(t, pool)
		require.NoError(t, rdb.FlushDB(ctx).Err())
		tree := testsupport.SeedOfficeTree(t, pool)
		role := testsupport.SeedRole(t, pool, "r-cache-subtree")
		testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOfficeSubtree)

		first, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		require.Len(t, first.OfficeIDs, 2)

		// Add a child office under Wilayah after the subtree was cached.
		var child uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
			 VALUES ($1, $2, 'New Child', 'NC') RETURNING id`,
			tree.Wilayah, tree.OfficeTypeID).Scan(&child))

		second, err := svc.Resolve(ctx, role, &tree.Wilayah, "offices")
		require.NoError(t, err)
		assert.Len(t, second.OfficeIDs, 2, "subtree cache should not include the new child")
		assert.False(t, idSet(second.OfficeIDs)[child])
	})
}
