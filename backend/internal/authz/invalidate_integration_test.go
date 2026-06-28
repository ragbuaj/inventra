//go:build integration

package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestScopeService_Invalidate(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	testsupport.Reset(t, pool)

	role := testsupport.SeedRole(t, pool, "r-inval-scope")
	testsupport.SeedScopePolicy(t, pool, role, "*", sqlc.SharedScopeLevelOwn)
	q := sqlc.New(pool)
	svc := authz.NewScopeService(q, rdb)

	// Warm the cache: resolves to 'own'.
	sc, err := svc.Resolve(ctx, role, nil, "assets")
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)

	// Change the policy directly in the DB to 'global'.
	_, err = pool.Exec(ctx, `UPDATE identity.data_scope_policies SET scope_level='global' WHERE role_id=$1 AND module='*'`, role)
	require.NoError(t, err)

	// Still cached as 'own' until invalidated.
	sc, _ = svc.Resolve(ctx, role, nil, "assets")
	require.Equal(t, sqlc.SharedScopeLevelOwn, sc.Level)

	require.NoError(t, svc.Invalidate(ctx, role))

	sc, err = svc.Resolve(ctx, role, nil, "assets")
	require.NoError(t, err)
	require.Equal(t, sqlc.SharedScopeLevelGlobal, sc.Level)
}

func TestFieldService_Invalidate(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	testsupport.Reset(t, pool)

	role := testsupport.SeedRole(t, pool, "r-inval-field")
	q := sqlc.New(pool)
	svc := authz.NewFieldService(q, rdb)

	// Warm cache: no policy for entity "assets" -> empty.
	pol, err := svc.ForEntity(ctx, role, "assets")
	require.NoError(t, err)
	require.Empty(t, pol)

	// Insert a hiding policy directly.
	_, err = pool.Exec(ctx, `INSERT INTO identity.field_permissions (entity, field, role_id, can_view, can_edit) VALUES ('assets','purchase_cost',$1,false,false)`, role)
	require.NoError(t, err)

	// Still cached empty.
	pol, _ = svc.ForEntity(ctx, role, "assets")
	require.Empty(t, pol)

	require.NoError(t, svc.Invalidate(ctx, role))

	pol, err = svc.ForEntity(ctx, role, "assets")
	require.NoError(t, err)
	require.Contains(t, pol, "purchase_cost")
}
